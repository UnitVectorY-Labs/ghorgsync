package sync

import (
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunWorkersBoundAndCompletion(t *testing.T) {
	for _, workerCount := range []int{1, 2, 4, 100} {
		items := []int{0, 1, 2, 3, 4, 5, 6, 7}
		var active, peak atomic.Int32
		var got []int // Deliberately not locked: consume must be serial.
		RunWorkers(items, workerCount, func(item int) int {
			n := active.Add(1)
			for old := peak.Load(); n > old; old = peak.Load() {
				if peak.CompareAndSwap(old, n) {
					break
				}
			}
			defer active.Add(-1)
			return item
		}, func(result int) { got = append(got, result) })
		slices.Sort(got)
		if !slices.Equal(got, items) || active.Load() != 0 || peak.Load() > int32(workerCount) {
			t.Fatalf("workerCount=%d: results=%v active=%d peak=%d", workerCount, got, active.Load(), peak.Load())
		}
	}
}

func TestRunWorkersSlowItemDoesNotBlockOthers(t *testing.T) {
	release := make(chan struct{})
	consumed := make(chan int, 3)
	done := make(chan struct{})
	go func() {
		RunWorkers([]int{0, 1, 2}, 2, func(i int) int {
			if i == 0 {
				<-release
			}
			return i
		}, func(i int) { consumed <- i })
		close(done)
	}()
	defer func() { close(release); <-done }()
	for _, want := range []int{1, 2} {
		select {
		case got := <-consumed:
			if got != want {
				t.Fatalf("got %d, want %d", got, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("slow first item blocked other workers")
		}
	}
}

func TestRunWorkersSequentialConsumer(t *testing.T) {
	consumed := 0
	RunWorkers([]int{0, 1, 2}, 1, func(i int) int {
		if consumed != i {
			t.Fatalf("started before previous consume completed")
		}
		return i
	}, func(int) { consumed++ })
	if consumed != 3 {
		t.Fatalf("consumed %d", consumed)
	}
	RunWorkers([]int(nil), 4, func(i int) int { t.Fatal("unexpected work"); return i }, func(int) { t.Fatal("unexpected result") })
}
