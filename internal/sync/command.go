package sync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// GitOptions applies to every Git subprocess, including local commands.
type GitOptions struct {
	Context     context.Context
	IdleTimeout time.Duration
}

// GitIdleTimeoutError describes output inactivity, not a proven network stall.
type GitIdleTimeoutError struct {
	Operation string
	Elapsed   time.Duration
	Idle      time.Duration
	Limit     time.Duration
}

func (e *GitIdleTimeoutError) Error() string {
	return fmt.Sprintf("git %s: output inactivity timeout (elapsed %s, no output for %s, limit %s)",
		e.Operation, e.Elapsed.Round(time.Millisecond), e.Idle.Round(time.Millisecond), e.Limit)
}

// Optional diagnostics may ignore ordinary Git errors, but cancellation and
// inactivity must always reach the repository result.
func commandInterrupted(err error) bool {
	var timeout *GitIdleTimeoutError
	return errors.As(err, &timeout) || errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) || errors.Is(err, exec.ErrWaitDelay)
}

// outputActivity is shared by stdout and stderr. Any nonempty write counts,
// including partial lines and carriage-return progress updates.
type outputActivity struct {
	mu      sync.Mutex
	last    time.Time
	changed chan struct{}
}

func (a *outputActivity) idle(now time.Time) time.Duration {
	a.mu.Lock()
	defer a.mu.Unlock()
	return max(0, now.Sub(a.last))
}

func (a *outputActivity) expired(now time.Time, limit time.Duration) bool {
	return limit > 0 && a.idle(now) >= limit
}

type activityWriter struct {
	activity *outputActivity
	dest     io.Writer
	tracef   func(string, ...any)
}

func (w activityWriter) Write(p []byte) (int, error) {
	w.activity.mu.Lock()
	if len(p) > 0 {
		w.activity.last = time.Now()
		select {
		case w.activity.changed <- struct{}{}:
		default:
		}
	}
	n, err := w.dest.Write(p)
	w.activity.mu.Unlock()
	if len(p) > 0 && w.tracef != nil {
		w.tracef("git output: %s", strings.TrimSpace(string(p)))
	}
	return n, err
}

// Keep only the tail of diagnostic output. Machine-readable stdout is captured
// separately without truncation so status and ignored-path parsing stay intact.
type outputTail []byte

const maxDiagnosticOutput = 64 * 1024

func (b *outputTail) Write(p []byte) (int, error) {
	n := len(p)
	if n >= maxDiagnosticOutput {
		*b = append((*b)[:0], p[n-maxDiagnosticOutput:]...)
	} else {
		if excess := len(*b) + n - maxDiagnosticOutput; excess > 0 {
			*b = append((*b)[:0], (*b)[excess:]...)
		}
		*b = append(*b, p...)
	}
	return n, nil
}

func commandOperation(cmd *exec.Cmd) string {
	for i := 1; i < len(cmd.Args); i++ {
		if cmd.Args[i] == "-C" || cmd.Args[i] == "-c" {
			i++
			continue
		}
		return cmd.Args[i]
	}
	return "command"
}

func (g *ExecGitRunner) run(cmd *exec.Cmd, combined bool) ([]byte, error) {
	ctx := g.options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tree, err := prepareProcessTree(cmd)
	if err != nil {
		return nil, fmt.Errorf("preparing Git process tree: %w", err)
	}
	defer tree.close()
	// Bound waiting for output pipes inherited by helpers after Git exits.
	cmd.WaitDelay = 2 * time.Second
	started := time.Now()
	activity := &outputActivity{last: started, changed: make(chan struct{}, 1)}
	var stdout bytes.Buffer
	var diagnostic outputTail
	var out io.Writer = &stdout
	if combined {
		out = &diagnostic
	}
	cmd.Stdout = activityWriter{activity, out, g.tracef}
	cmd.Stderr = activityWriter{activity, &diagnostic, g.tracef}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	if err := tree.started(); err != nil {
		_ = tree.kill()
		_ = cmd.Process.Kill() // also covers Windows job-assignment failure
		_ = cmd.Wait()
		return nil, fmt.Errorf("starting Git process tree: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	operation := commandOperation(cmd)
	result := func(err error) ([]byte, error) {
		if errors.Is(err, exec.ErrWaitDelay) {
			// Git exited but a helper kept its pipes open. Do not orphan it.
			err = errors.Join(err, tree.kill())
		}
		if combined {
			return diagnostic, err
		}
		if err != nil && len(diagnostic) > 0 {
			err = fmt.Errorf("%s: %w", strings.TrimSpace(string(diagnostic)), err)
		}
		return stdout.Bytes(), err
	}
	stop := func(reason error) ([]byte, error) {
		killErr := tree.kill()
		if killErr != nil {
			killErr = errors.Join(killErr, cmd.Process.Kill())
		}
		<-done // reap Git and let WaitDelay bound inherited pipe waits
		return result(errors.Join(reason, killErr))
	}
	limit := g.options.IdleTimeout
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	if limit > 0 {
		timer.Reset(limit)
	} else {
		timer.Stop()
	}
	diagnostics := time.NewTicker(30 * time.Second)
	defer diagnostics.Stop()
	for {
		select {
		case err := <-done:
			return result(err)
		case <-ctx.Done():
			return stop(ctx.Err())
		case <-activity.changed:
			if limit > 0 {
				timer.Reset(max(time.Nanosecond, limit-activity.idle(time.Now())))
			}
		case now := <-timer.C:
			// An output write can race the timer. Check the current activity,
			// rather than treating a previously scheduled deadline as final.
			if activity.expired(now, limit) {
				select {
				case err := <-done:
					return result(err)
				default:
				}
				return stop(&GitIdleTimeoutError{operation, now.Sub(started), activity.idle(now), limit})
			}
			timer.Reset(max(time.Nanosecond, limit-activity.idle(time.Now())))
		case now := <-diagnostics.C:
			if g.logf != nil {
				g.logf("git %s active: elapsed=%s no-output=%s", operation,
					now.Sub(started).Round(time.Second), activity.idle(now).Round(time.Second))
			}
		}
	}
}
