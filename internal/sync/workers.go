package sync

import "sync"

// RunWorkers processes items with at most workerCount concurrent workers. consume runs
// serially in the caller, in completion order, and all workers finish before
// RunWorkers returns. With one worker, consume finishes before the next item starts.
func RunWorkers[T, R any](items []T, workerCount int, work func(T) R, consume func(R)) {
	if workerCount < 1 {
		panic("worker count must be positive")
	}
	if workerCount == 1 {
		for _, item := range items {
			consume(work(item))
		}
		return
	}
	queue := make(chan T)
	results := make(chan R)
	var workers sync.WaitGroup
	for range min(workerCount, len(items)) {
		workers.Go(func() {
			for item := range queue {
				results <- work(item)
			}
		})
	}
	go func() {
		for _, item := range items {
			queue <- item
		}
		close(queue)
		workers.Wait()
		close(results)
	}()
	for result := range results {
		consume(result)
	}
}
