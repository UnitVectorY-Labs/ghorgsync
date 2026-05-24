package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"

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

// main is the entry point for the ghorgsync command-line tool.
func main() {
	// Set the build version from the build info if not set by the build system
	if Version == "dev" || Version == "" {
		if bi, ok := debug.ReadBuildInfo(); ok {
			if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
				Version = bi.Main.Version
			}
		}
	}

	// Parse flags
	versionFlag := flag.Bool("version", false, "Print version and exit")
	var verbosity countFlag
	flag.Var(&verbosity, "verbose", "Enable verbose output; repeat (--verbose --verbose) for trace-level detail including raw command output and API response bodies")
	noColorFlag := flag.Bool("no-color", false, "Disable color output")
	noProgressFlag := flag.Bool("no-progress", false, "Suppress the live progress bar (useful for scripting, CI, and when output is consumed by another program)")
	cloneOnlyFlag := flag.Bool("clone", false, "Only clone missing repositories (skip processing existing repos)")
	statusFlag := flag.Bool("status", false, "Show status of repositories (dirty repos and branch drift only)")
	flag.Parse()

	// Mode flags are mutually exclusive
	if *cloneOnlyFlag && *statusFlag {
		fmt.Fprintln(os.Stderr, "error: --clone and --status are mutually exclusive")
		os.Exit(1)
	}

	if *versionFlag {
		fmt.Println(versionString(Version))
		os.Exit(0)
	}

	useColor := !*noColorFlag && output.ShouldColor()
	printer := output.NewPrinter(useColor, int(verbosity), *noProgressFlag)

	// Startup gate: check for dotfile
	exePath, err := os.Executable()
	if err != nil {
		printer.SystemError("executable", err)
		os.Exit(1)
	}
	baseName := filepath.Base(exePath)
	ext := filepath.Ext(baseName)
	if ext != "" {
		baseName = strings.TrimSuffix(baseName, ext)
	}
	dotfileName := "." + baseName

	if _, err := os.Stat(dotfileName); os.IsNotExist(err) {
		printer.MissingDotfile(dotfileName)
		os.Exit(0)
	}

	// Load and validate config
	cfg, err := config.Load(dotfileName)
	if err != nil {
		printer.ConfigError(err)
		os.Exit(1)
	}
	if err := cfg.Validate(); err != nil {
		printer.ConfigError(err)
		os.Exit(1)
	}

	// Resolve token and create GitHub client
	token := github.ResolveToken()
	client := github.NewClient(token, printer.Verbose, printer.Trace)

	var allRepos []model.RepoInfo
	if cfg.IsUserMode() {
		allRepos, err = client.ListUserRepos(cfg.User)
	} else {
		allRepos, err = client.ListOrgRepos(cfg.Organization)
	}
	if err != nil {
		printer.AuthError(err)
		os.Exit(1)
	}

	// Filter repos
	included, excludedNames := github.FilterRepos(allRepos, cfg)
	printer.Verbose("Found %d repositories (%d included, %d excluded)", len(allRepos), len(included), len(excludedNames))

	// Scan directory
	dir, _ := os.Getwd()
	scanResult, err := scanner.ScanDirectory(dir, included, excludedNames, cfg)
	if err != nil {
		printer.SystemError("scan", err)
		os.Exit(1)
	}

	// Create sync engine
	eng := sync.NewEngine(dir, int(verbosity), printer.Verbose, printer.Trace)

	// Build lookup map from repo name → RepoInfo
	repoMap := make(map[string]model.RepoInfo, len(included))
	for _, r := range included {
		repoMap[r.Name] = r
	}

	// Summary counters
	var summary model.Summary
	summary.TotalRepos = len(included)

	if *cloneOnlyFlag {
		// Clone-only mode: only clone missing repos, skip everything else
		printer.StartRepoProgress(len(scanResult.ManagedMissing))

		for _, name := range scanResult.ManagedMissing {
			repo := repoMap[name]
			result := eng.CloneRepo(repo)
			handleResult(printer, result, &summary)
			printer.AdvanceRepoProgress()
		}

		printer.FinishRepoProgress()
	} else if *statusFlag {
		// Status mode: read-only check of existing repos
		printer.StartRepoProgress(len(scanResult.ManagedFound))

		for _, name := range scanResult.ManagedFound {
			repo := repoMap[name]
			result := eng.StatusRepo(repo)
			switch result.Action {
			case model.ActionDirty:
				printer.RepoStatusDirty(result.Name, result.CurrentBranch, result.DefaultBranch, result.StatusOutput)
				summary.Dirty++
			case model.ActionBranchDrift:
				printer.RepoStatusBranchDrift(result.Name, result.CurrentBranch, result.DefaultBranch)
				summary.BranchDrift++
			case model.ActionFetchError:
				printer.RepoError(result.Name, result.Action.String(), result.Error)
				summary.Errors++
			}
			printer.AdvanceRepoProgress()
		}

		printer.FinishRepoProgress()

		printer.StatusSummary(summary.TotalRepos, summary.Dirty, summary.BranchDrift)
		os.Exit(0)
	} else {
		// Default mode: full sync
		summary.UnknownFolders = len(scanResult.Unknown)
		summary.ExcludedButPresent = len(scanResult.ExcludedButPresent)
		summary.Errors = len(scanResult.Collisions)

		repoWorkTotal := len(scanResult.ManagedMissing) + len(scanResult.ManagedFound)
		printer.StartRepoProgress(repoWorkTotal)

		// Clone missing repos
		for _, name := range scanResult.ManagedMissing {
			repo := repoMap[name]
			result := eng.CloneRepo(repo)
			handleResult(printer, result, &summary)
			printer.AdvanceRepoProgress()
		}

		// Process existing repos
		for _, name := range scanResult.ManagedFound {
			repo := repoMap[name]
			result := eng.ProcessRepo(repo)
			handleResult(printer, result, &summary)
			printer.AdvanceRepoProgress()
		}

		printer.FinishRepoProgress()

		// Report collisions
		for _, entry := range scanResult.Collisions {
			printer.Collision(entry.Name, entry.Detail)
		}

		// Report unknown folders
		for _, entry := range scanResult.Unknown {
			printer.UnknownFolder(entry.Name)
		}

		// Report excluded-but-present
		for _, entry := range scanResult.ExcludedButPresent {
			printer.ExcludedButPresent(entry.Name)
		}
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
	)
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
	case model.ActionCloneError, model.ActionFetchError, model.ActionCheckoutError, model.ActionPullError, model.ActionSubmoduleError:
		printer.RepoError(result.Name, result.Action.String(), result.Error)
		summary.Errors++
	}
}
