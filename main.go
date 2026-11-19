package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/UnitVectorY-Labs/ghorgsync/internal/archived"
	"github.com/UnitVectorY-Labs/ghorgsync/internal/cleanup"
	"github.com/UnitVectorY-Labs/ghorgsync/internal/config"
	"github.com/UnitVectorY-Labs/ghorgsync/internal/github"
	"github.com/UnitVectorY-Labs/ghorgsync/internal/model"
	"github.com/UnitVectorY-Labs/ghorgsync/internal/output"
	"github.com/UnitVectorY-Labs/ghorgsync/internal/scanner"
	"github.com/UnitVectorY-Labs/ghorgsync/internal/sync"
)

var Version = "dev" // This will be set by the build systems to the release version

var semverRe = regexp.MustCompile(`^\d+\.\d+\.\d+`)

func normalizedVersion(version string) string {
	if semverRe.MatchString(version) && !strings.HasPrefix(version, "v") {
		return "v" + version
	}
	return version
}

func versionString(version string) string {
	return fmt.Sprintf("ghorgsync version %s (%s, %s/%s)", normalizedVersion(version), runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

// countFlag is a flag.Value that counts how many times --verbose is repeated.
// Implements IsBoolFlag so it can be used without an explicit value (--verbose).
type countFlag int

func (c *countFlag) String() string   { return fmt.Sprintf("%d", int(*c)) }
func (c *countFlag) IsBoolFlag() bool { return true }
func (c *countFlag) Set(s string) error {
	if s == "false" {
		*c = 0
		return nil
	}
	*c++
	return nil
}

// printUsage describes the modes and their safety boundaries before listing flags.
func printUsage(w io.Writer, flags *flag.FlagSet) {
	fmt.Fprintln(w, "Usage: ghorgsync [flags]")
	fmt.Fprintln(w, "\nSync GitHub organization or user repositories in the current directory.")
	fmt.Fprintln(w, "Requires a YAML dotfile named after the executable (normally .ghorgsync).")
	fmt.Fprintln(w, "\nDefault: clone missing repositories; fetch existing repositories and only")
	fmt.Fprintln(w, "checkout/pull when clean. --clone only clones; --status only reads local state.")
	fmt.Fprintln(w, "\nRemoval requires --clean or --delete-archived. --dry-run previews removal;")
	fmt.Fprintln(w, "normal sync still runs. --force overrides archived content/history protection.")
	fmt.Fprintln(w, "\nUse -h or --help for help; --version prints build information.")
	fmt.Fprintln(w, "\nFlags (single or double hyphens are accepted):")
	flags.PrintDefaults()
}

func operationalExitCode(errors int) int {
	if errors > 0 {
		return 1
	}
	return 0
}

// main is the entry point for the ghorgsync command-line tool.
func main() {
	os.Exit(run())
}

// run returns before main exits so account restoration runs on error paths too.
func run() (exitCode int) {
	// Set the build version from the build info if not set by the build system
	if Version == "dev" || Version == "" {
		if bi, ok := debug.ReadBuildInfo(); ok {
			if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
				Version = bi.Main.Version
			}
		}
	}

	// Parse flags
	flag.Usage = func() { printUsage(flag.CommandLine.Output(), flag.CommandLine) }
	versionFlag := flag.Bool("version", false, "Print version and exit")
	var verbosity countFlag
	flag.Var(&verbosity, "verbose", "Enable verbose output; repeat (--verbose --verbose) for trace-level detail including raw command output and API response bodies")
	noColorFlag := flag.Bool("no-color", false, "Disable color output")
	noProgressFlag := flag.Bool("no-progress", false, "Suppress the live progress bar (useful for scripting, CI, and when output is consumed by another program)")
	workersFlag := flag.String("workers", "", "Maximum concurrent repositories (GHORGSYNC_WORKERS; default: 4 times logical CPU count, capped at 32; 1 for sequential processing)")
	cloneOnlyFlag := flag.Bool("clone", false, "Only clone missing repositories (skip processing existing repos)")
	statusFlag := flag.Bool("status", false, "Show status of repositories (dirty repos and branch drift only)")
	deleteArchivedFlag := flag.Bool("delete-archived", false, "Delete eligible local clones excluded because they are archived (non-interactive)")
	cleanFlag := flag.Bool("clean", false, "Remove git-ignored files and directories after syncing (non-interactive)")
	forceFlag := flag.Bool("force", false, "With --delete-archived, delete despite local content or unpublished history (also accepted with --clean for compatibility)")
	dryRunFlag := flag.Bool("dry-run", false, "With --clean or --delete-archived, preview removal without deleting anything")
	flag.Parse()
	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "error: unexpected positional argument %q; usage: ghorgsync [flags]\n", flag.Arg(0))
		return 1
	}

	if err := validateDeletionFlags(*deleteArchivedFlag, *cloneOnlyFlag, *statusFlag, *cleanFlag); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	// Mode flags are mutually exclusive
	if *cloneOnlyFlag && *statusFlag {
		fmt.Fprintln(os.Stderr, "error: --clone and --status are mutually exclusive")
		return 1
	}
	if (*forceFlag || *dryRunFlag) && !*cleanFlag && !*deleteArchivedFlag {
		fmt.Fprintln(os.Stderr, "error: --force and --dry-run require --clean or --delete-archived")
		return 1
	}
	if *cleanFlag && (*cloneOnlyFlag || *statusFlag) {
		fmt.Fprintln(os.Stderr, "error: --clean is only available with the default sync mode")
		return 1
	}

	if *versionFlag {
		fmt.Println(versionString(Version))
		return 0
	}

	useColor := !*noColorFlag && output.ShouldColor()
	printer := output.NewPrinter(useColor, int(verbosity), *noProgressFlag)

	// Startup gate: check for dotfile
	exePath, err := os.Executable()
	if err != nil {
		printer.SystemError("executable", err)
		return 1
	}
	baseName := filepath.Base(exePath)
	ext := filepath.Ext(baseName)
	if ext != "" {
		baseName = strings.TrimSuffix(baseName, ext)
	}
	dotfileName := "." + baseName

	if _, err := os.Stat(dotfileName); err != nil {
		if os.IsNotExist(err) {
			printer.MissingDotfile(dotfileName)
			return 0
		}
		printer.ConfigError(err)
		return 1
	}

	// Load and validate config
	workersSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "workers" {
			workersSet = true
		}
	})
	workerCount, err := config.ResolveWorkers(*workersFlag, os.Getenv("GHORGSYNC_WORKERS"), workersSet)
	if err != nil {
		printer.ConfigError(err)
		return 1
	}
	// Cleanup must finish before the next repository.
	if *cleanFlag {
		workerCount = 1
	}
	started := time.Now()
	defer func() { printer.Verbose("total elapsed: %s", time.Since(started).Round(time.Millisecond)) }()
	cfg, err := config.Load(dotfileName)
	if err != nil {
		printer.ConfigError(err)
		return 1
	}
	if err := cfg.Validate(); err != nil {
		printer.ConfigError(err)
		return 1
	}

	if *deleteArchivedFlag && cfg.ShouldIncludeArchived() {
		printer.ConfigError(fmt.Errorf("--delete-archived requires include_archived: false"))
		return 1
	}

	// Select the configured CLI account before resolving any API credentials.
	printer.WorkerCount(workerCount)
	var token string
	if cfg.AuthUser != "" {
		var restore func() error
		token, restore, err = github.SelectAccount(cfg.AuthUser, cfg.AuthSwitchBack, printer.AuthSwitched)
		if restore != nil {
			defer func() {
				if err := restore(); err != nil {
					printer.AuthError(err)
					exitCode = 1
				}
			}()
		}
		if err != nil {
			printer.AuthError(err)
			return 1
		}
	} else {
		token = github.ResolveToken()
	}
	client := github.NewClient(token, printer.Verbose, printer.Trace)

	var allRepos []model.RepoInfo
	discoveryStarted := time.Now()
	if cfg.IsUserMode() {
		authUser, authUserErr := client.GetAuthenticatedUser()
		if authUserErr == nil && strings.EqualFold(authUser, cfg.User) {
			allRepos, err = client.ListOwnRepos()
		} else {
			if cfg.ShouldIncludePrivate() {
				if authUserErr != nil {
					printer.Verbose("warning: could not verify authenticated user (%v); private repositories may not be included", authUserErr)
				} else {
					printer.Verbose("warning: configured user %q does not match authenticated user %q; private repositories will not be included", cfg.User, authUser)
				}
			}
			allRepos, err = client.ListUserRepos(cfg.User)
		}
	} else {
		allRepos, err = client.ListOrgRepos(cfg.Organization)
	}
	if err != nil {
		printer.AuthError(err)
		return 1
	}

	// Filter repos
	printer.Verbose("repository discovery elapsed: %s", time.Since(discoveryStarted).Round(time.Millisecond))
	included, excludedNames, emptyRepos := github.FilterRepos(allRepos, cfg)
	printer.Verbose("Found %d repositories (%d included, %d excluded, %d empty)", len(allRepos), len(included), len(excludedNames), len(emptyRepos))

	// Scan directory
	dir, _ := os.Getwd()
	scanStarted := time.Now()
	scanResult, err := scanner.ScanDirectory(dir, included, excludedNames, cfg)
	if err != nil {
		printer.SystemError("scan", err)
		return 1
	}

	// Create sync engine
	printer.Verbose("directory scan elapsed: %s", time.Since(scanStarted).Round(time.Millisecond))
	eng := sync.NewEngine(dir, int(verbosity), printer.Verbose, printer.Trace)

	// Build lookup map from repo name → RepoInfo
	repoMap := make(map[string]model.RepoInfo, len(included))
	for _, r := range included {
		repoMap[r.Name] = r
	}

	// Summary counters
	var summary model.Summary
	deletedArchived, plannedArchived, retainedArchived := 0, 0, 0
	summary.TotalRepos = len(included)
	summary.Empty = len(emptyRepos)
	summary.Errors = len(scanResult.Collisions)
	for _, entry := range scanResult.Collisions {
		printer.Collision(entry.Name, entry.Detail)
	}

	if *cloneOnlyFlag {
		// Clone-only mode: only clone missing repos, skip everything else
		printer.StartRepoProgress(len(scanResult.ManagedMissing))

		runRepoWorkers(scanResult.ManagedMissing, repoMap, workerCount, dir, int(verbosity), printer, (*sync.Engine).CloneRepo, func(result model.RepoResult) {
			handleResult(printer, result, &summary)
			printer.AdvanceRepoProgress()
		})

		printer.FinishRepoProgress()
	} else if *statusFlag {
		// Status mode: read-only check of existing repos
		printer.StartRepoProgress(len(scanResult.ManagedFound))

		runRepoWorkers(scanResult.ManagedFound, repoMap, workerCount, dir, int(verbosity), printer, (*sync.Engine).StatusRepo, func(result model.RepoResult) {
			switch result.Action {
			case model.ActionDirty:
				printer.RepoStatusDirty(result.Name, result.CurrentBranch, result.DefaultBranch, result.StatusOutput)
				summary.Dirty++
			case model.ActionBranchDrift:
				printer.RepoStatusBranchDrift(result.Name, result.CurrentBranch, result.DefaultBranch)
				summary.BranchDrift++
			case model.ActionFetchError, model.ActionIdentityError:
				printer.RepoError(result.Name, result.Action.String(), result.Error)
				summary.Errors++
			}
			printer.AdvanceRepoProgress()
		})

		printer.FinishRepoProgress()

		printer.StatusSummary(len(scanResult.ManagedFound), summary.Dirty, summary.BranchDrift, summary.Errors, summary.Empty)
		return operationalExitCode(summary.Errors)
	} else {
		// Default mode: full sync
		summary.UnknownFolders = len(scanResult.Unknown)
		summary.ExcludedButPresent = len(scanResult.ExcludedButPresent)

		repoWorkTotal := len(scanResult.ManagedMissing) + len(scanResult.ManagedFound)
		printer.StartRepoProgress(repoWorkTotal)
		// Clone missing repos
		runRepoWorkers(scanResult.ManagedMissing, repoMap, workerCount, dir, int(verbosity), printer, (*sync.Engine).CloneRepo, func(result model.RepoResult) {
			handleResult(printer, result, &summary)
			if *cleanFlag && result.Action == model.ActionCloned {
				cleanRepoIgnoredContent(eng, dir, result.Name, printer, *dryRunFlag, &summary)
			}
			printer.AdvanceRepoProgress()
		})

		// Process existing repos
		runRepoWorkers(scanResult.ManagedFound, repoMap, workerCount, dir, int(verbosity), printer, (*sync.Engine).ProcessRepo, func(result model.RepoResult) {
			handleResult(printer, result, &summary)
			if *cleanFlag && result.Action != model.ActionIdentityError {
				cleanRepoIgnoredContent(eng, dir, result.Name, printer, *dryRunFlag, &summary)
			}
			printer.AdvanceRepoProgress()
		})

		printer.FinishRepoProgress()

		// Report unknown folders
		for _, entry := range scanResult.Unknown {
			printer.UnknownFolder(entry.Name)
		}

		// Report excluded-but-present
		for _, entry := range scanResult.ExcludedButPresent {
			reason := "name matches exclude_repos"
			var candidate model.RepoInfo
			for _, repo := range allRepos {
				if repo.Name == entry.Name && archived.Candidate(repo, cfg) {
					reason = "archived"
					candidate = repo
					break
				}
			}
			if *deleteArchivedFlag && candidate.IsArchived {
				result := deleteArchivedRepo(dir, candidate, printer, *forceFlag, *dryRunFlag)
				if result == "deleted" {
					summary.ExcludedButPresent--
					deletedArchived++
				} else if result == "planned" {
					plannedArchived++
				} else {
					retainedArchived++
				}
				if result == "error" {
					summary.Errors++
				}
			} else {
				printer.ExcludedReason(entry.Name, reason)
			}
		}

	}

	if *deleteArchivedFlag {
		printer.ArchivedTotals(deletedArchived, plannedArchived, retainedArchived)
	}

	// Print summary
	printer.Summary(
		summary.TotalRepos,
		summary.Cloned,
		summary.Updated,
		summary.Dirty,
		summary.BranchDrift,
		summary.UnknownFolders,
		summary.ExcludedButPresent,
		summary.Errors,
		summary.Empty,
	)
	return operationalExitCode(summary.Errors)
}

// Each repository gets its own diagnostics context. Only the serial consumer
// updates summaries; the printer serializes worker diagnostics with results.
func runRepoWorkers(names []string, repos map[string]model.RepoInfo, workerCount int, baseDir string, verbosity int, printer *output.Printer, operation func(*sync.Engine, model.RepoInfo) model.RepoResult, consume func(model.RepoResult)) {
	sync.RunWorkers(names, workerCount, func(name string) model.RepoResult {
		logf := func(format string, args ...any) {
			printer.Verbose("repo %s: %s", name, fmt.Sprintf(format, args...))
		}
		tracef := func(format string, args ...any) {
			printer.Trace("repo %s: %s", name, fmt.Sprintf(format, args...))
		}
		eng := sync.NewEngine(baseDir, verbosity, logf, tracef)
		started := time.Now()
		result := operation(eng, repos[name])
		logf("completed in %s", time.Since(started).Round(time.Millisecond))
		return result
	}, consume)
}

// cleanRepoIgnoredContent is the final phase for one repository. It runs
// immediately after that repository's normal sync work.
func cleanRepoIgnoredContent(eng *sync.Engine, baseDir, name string, printer *output.Printer, dryRun bool, summary *model.Summary) {
	repoDir := filepath.Join(baseDir, name)
	paths, err := eng.Git.IgnoredPaths(repoDir)
	if err != nil {
		printer.RepoError(name, "cleanup-error", err)
		summary.Errors++
		return
	}
	targets, err := cleanup.Plan(repoDir, paths)
	if err != nil {
		printer.RepoError(name, "cleanup-error", err)
		summary.Errors++
		return
	}
	if len(targets) == 0 {
		printer.Verbose("cleanup: %s has no ignored content", name)
		return
	}

	files, _ := cleanup.Counts(targets)
	printer.RepoCleanupPlanned(name, files, cleanup.TotalSize(targets), dryRun)
	if dryRun {
		for _, target := range targets {
			printer.Verbose("cleanup: %s would remove %s", name, target.Path)
		}
		return
	}

	for _, target := range targets {
		if err := cleanup.Remove(repoDir, target); err != nil {
			printer.RepoError(name, "cleanup-error", err)
			summary.Errors++
			continue
		}
		printer.Verbose("cleanup: %s removed %s", name, target.Path)
		removedDirs, err := cleanup.PruneEmptyParents(repoDir, target)
		if err != nil {
			printer.RepoError(name, "cleanup-error", err)
			summary.Errors++
			continue
		}
		for _, dir := range removedDirs {
			printer.Verbose("cleanup: %s removed %s/", name, dir)
		}
	}
}

// handleResult maps a RepoResult to the appropriate printer call and updates summary counts.
func handleResult(printer *output.Printer, result model.RepoResult, summary *model.Summary) {
	switch result.Action {
	case model.ActionCloned:
		printer.RepoCloned(result.Name)
		summary.Cloned++
	case model.ActionUpdated:
		printer.RepoUpdated(result.Name)
		summary.Updated++
	case model.ActionDirty:
		files := make([]output.DirtyFileInfo, len(result.DirtyFiles))
		for i, f := range result.DirtyFiles {
			files[i] = output.DirtyFileInfo{
				Path:     f.Path,
				Staged:   f.Staged,
				Unstaged: f.Unstaged,
			}
		}
		printer.RepoDirty(result.Name, result.CurrentBranch, result.DefaultBranch, files, result.Additions, result.Deletions)
		summary.Dirty++
	case model.ActionBranchDrift:
		printer.RepoBranchDrift(result.Name, result.CurrentBranch, result.DefaultBranch, result.Updated)
		if result.Updated {
			summary.Updated++
		}
		summary.BranchDrift++
	case model.ActionAlreadyCurrent:
		printer.Verbose("%s is already up to date", result.Name)
	case model.ActionCloneError, model.ActionFetchError, model.ActionCheckoutError, model.ActionPullError, model.ActionSubmoduleError, model.ActionIdentityError:
		if result.Updated {
			summary.Updated++
		}
		printer.RepoError(result.Name, result.Action.String(), result.Error)
		summary.Errors++
	}
}

func validateDeletionFlags(deleteArchived, clone, status, clean bool) error {
	if deleteArchived && (clone || status || clean) {
		return fmt.Errorf("--delete-archived cannot be combined with --clone, --status, or --clean")
	}
	return nil
}

func deleteArchivedRepo(root string, repo model.RepoInfo, printer *output.Printer, force, dryRun bool) string {
	info, err := archived.Check(root, repo, force)
	if err != nil {
		if archived.IsRetained(err) {
			printer.ArchivedStatus(repo.Name, "retained: "+err.Error()+"; use --force to override content/history protection", false)
			return "retained"
		}
		printer.RepoError(repo.Name, "archive-deletion-error", err)
		return "error"
	}
	if dryRun {
		printer.ArchivedStatus(repo.Name, "would delete local folder (dry-run)", false)
		return "planned"
	}
	printer.ArchivedStatus(repo.Name, "local folder eligible for deletion", false)
	if err := archived.Remove(root, repo, info, force); err != nil {
		if archived.IsRetained(err) {
			printer.ArchivedStatus(repo.Name, "retained: "+err.Error()+"; --force overrides content/history protection", false)
			return "retained"
		}
		printer.RepoError(repo.Name, "archive-deletion-error", err)
		return "error"
	}
	printer.ArchivedStatus(repo.Name, "deleted local folder", true)
	return "deleted"
}
