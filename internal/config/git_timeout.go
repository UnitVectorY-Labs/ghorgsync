package config

import (
	"fmt"
	"time"
)

const DefaultGitIdleTimeout = 2 * time.Minute

// ResolveGitIdleTimeout gives an explicit flag precedence over the environment.
// Zero explicitly disables automatic cancellation.
func ResolveGitIdleTimeout(flagValue, envValue string, flagSet bool) (time.Duration, error) {
	value, source := envValue, "GHORGSYNC_GIT_IDLE_TIMEOUT"
	if flagSet {
		value, source = flagValue, "--git-idle-timeout"
	} else if value == "" {
		return DefaultGitIdleTimeout, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("%s must be a non-negative duration (for example 2m or 0 to disable)", source)
	}
	return d, nil
}
