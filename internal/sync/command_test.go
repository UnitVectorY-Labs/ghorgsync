package sync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestOutputInactivity(t *testing.T) {
	start := time.Unix(1000, 0)
	a := &outputActivity{last: start}
	limit := 2 * time.Minute
	if a.expired(start.Add(24*time.Hour), 0) {
		t.Fatal("default must never expire")
	}
	if a.expired(start.Add(limit-time.Nanosecond), limit) || !a.expired(start.Add(limit), limit) {
		t.Fatal("incorrect inactivity boundary")
	}
	// Long total elapsed time does not matter if output continues.
	a.last = start.Add(24 * time.Hour)
	if a.expired(a.last.Add(time.Second), limit) {
		t.Fatal("recent activity must keep a long-running command alive")
	}
	// A timer that fired before a concurrent write must not cancel the command.
	if a.expired(start.Add(limit), limit) {
		t.Fatal("a stale timer must not expire newer activity")
	}
}

func TestActivityWriterPartialOutput(t *testing.T) {
	a := &outputActivity{last: time.Unix(1, 0), changed: make(chan struct{}, 1)}
	var out bytes.Buffer
	w := activityWriter{activity: a, dest: &out}
	for _, p := range []string{"Receiving objects: 1%\r", "Receiving objects: 2%", "\x00"} {
		before := time.Now()
		n, err := w.Write([]byte(p))
		if err != nil || n != len(p) || a.last.Before(before) {
			t.Fatalf("write did not register activity: n=%d err=%v", n, err)
		}
	}
	if out.String() != "Receiving objects: 1%\rReceiving objects: 2%\x00" {
		t.Fatal("writer changed raw output")
	}
	last := a.last
	_, _ = w.Write(nil)
	if !a.last.Equal(last) {
		t.Fatal("empty writes must not extend the deadline")
	}
	if len(a.changed) != 1 {
		t.Fatal("activity notifications should coalesce without blocking writers")
	}
}

func TestConcurrentOutputActivity(t *testing.T) {
	a := &outputActivity{last: time.Now(), changed: make(chan struct{}, 1)}
	var out bytes.Buffer
	w := activityWriter{activity: a, dest: &out}
	var workers sync.WaitGroup
	for range 2 { // stdout and stderr share the activity state
		workers.Go(func() {
			for range 100 {
				_, _ = w.Write([]byte("x"))
				_ = a.expired(time.Now(), time.Minute)
			}
		})
	}
	workers.Wait()
	if out.Len() != 200 {
		t.Fatalf("lost output: %d bytes", out.Len())
	}
}

func TestDiagnosticOutputTail(t *testing.T) {
	var tail outputTail
	_, _ = tail.Write(bytes.Repeat([]byte("a"), maxDiagnosticOutput-2))
	_, _ = tail.Write([]byte("bcde"))
	if len(tail) != maxDiagnosticOutput || !strings.HasSuffix(string(tail), "bcde") {
		t.Fatal("small writes must evict old diagnostic output")
	}
	large := bytes.Repeat([]byte("z"), maxDiagnosticOutput+100)
	_, _ = tail.Write(large)
	if len(tail) != maxDiagnosticOutput || !bytes.Equal(tail, large[100:]) {
		t.Fatal("large writes must retain only the tail")
	}
}

func TestIdleTimeoutErrorClassification(t *testing.T) {
	original := &GitIdleTimeoutError{Operation: "fetch", Elapsed: 10 * time.Minute, Idle: 2 * time.Minute, Limit: 2 * time.Minute}
	wrapped := fmt.Errorf("git fetch: %w", errors.Join(original, errors.New("cleanup error")))
	var timeout *GitIdleTimeoutError
	if !errors.As(wrapped, &timeout) || timeout != original {
		t.Fatal("timeout classification must survive operation and cleanup wrapping")
	}
	for _, want := range []string{"fetch", "elapsed 10m0s", "no output for 2m0s", "limit 2m0s"} {
		if !strings.Contains(wrapped.Error(), want) {
			t.Fatalf("error lacks %q: %v", want, wrapped)
		}
	}
}

func TestCommandInterruption(t *testing.T) {
	for _, err := range []error{context.Canceled, context.DeadlineExceeded, exec.ErrWaitDelay,
		&GitIdleTimeoutError{Operation: "status", Limit: time.Minute}} {
		if !commandInterrupted(fmt.Errorf("optional diagnostic: %w", err)) {
			t.Fatalf("interruption must not be swallowed: %v", err)
		}
	}
	if commandInterrupted(nil) || commandInterrupted(errors.New("ordinary Git error")) {
		t.Fatal("ordinary optional diagnostic errors should remain non-fatal")
	}
}
