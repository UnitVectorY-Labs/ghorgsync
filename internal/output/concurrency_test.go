package output

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestConcurrentOutputAndProgress(t *testing.T) {
	for _, live := range []bool{false, true} {
		t.Run(fmt.Sprintf("live=%t", live), func(t *testing.T) {
			// Capture bytes without a terminal, git commands, or external services.
			file, err := os.CreateTemp(t.TempDir(), "output")
			if err != nil {
				t.Fatal(err)
			}
			original := os.Stdout
			os.Stdout = file
			defer func() { os.Stdout = original; file.Close() }()
			p := &Printer{verbosity: 2, interactive: live}
			const count = 40
			p.StartRepoProgress(count)
			var workers sync.WaitGroup
			for i := range count {
				workers.Go(func() {
					name := fmt.Sprintf("repo-%02d", i)
					p.RepoDirty(name, "main", "main", []DirtyFileInfo{{Path: name + ".txt", Staged: true}}, 0, 0)
					p.Trace("%s begin\n%s end", name, name)
					p.Verbose("%s diagnostic", name)
					p.AdvanceRepoProgress()
				})
			}
			workers.Wait()
			if p.repoProgress.current != count {
				t.Fatalf("progress = %d", p.repoProgress.current)
			}
			p.FinishRepoProgress()
			data, err := os.ReadFile(file.Name())
			if err != nil {
				t.Fatal(err)
			}
			out := string(data)
			for i := range count {
				name := fmt.Sprintf("repo-%02d", i)
				for _, block := range []string{
					fmt.Sprintf("  repo %s [dirty] on main\n       checkout/pull skipped due to dirty working tree\n       [staged] %s.txt\n", name, name),
					fmt.Sprintf("  %s begin\n  %s end\n", name, name),
					fmt.Sprintf("  %s diagnostic\n", name),
				} {
					if strings.Count(out, block) != 1 {
						t.Errorf("missing, duplicated, or interleaved block %q", block)
					}
				}
			}
			if live {
				if !strings.Contains(out, "40/40") || !strings.HasSuffix(out, "100%\n") {
					t.Error("missing completed progress")
				}
			} else if strings.Contains(out, clearLine) {
				t.Error("terminal escapes in non-TTY output")
			}
		})
	}
}
