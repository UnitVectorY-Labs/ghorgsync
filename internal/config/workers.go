package config

import (
	"fmt"
	"runtime"
	"strconv"
)

// ResolveWorkers selects the repository concurrency. An explicit flag overrides
// the environment, including an invalid environment value.
func ResolveWorkers(flagValue, envValue string, flagSet bool) (int, error) {
	value, source := envValue, "GHORGSYNC_WORKERS"
	if flagSet {
		value, source = flagValue, "--workers"
	} else if value == "" {
		return 4 * runtime.NumCPU(), nil
	}
	workerCount, err := strconv.Atoi(value)
	if err != nil || workerCount < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", source)
	}
	return workerCount, nil
}
