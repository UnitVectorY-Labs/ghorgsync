package config

import (
	"runtime"
	"testing"
)

func TestResolveWorkers(t *testing.T) {
	for _, tt := range []struct {
		name, flag, env string
		set             bool
		want            int
		fail            bool
	}{
		{name: "four workers per CPU thread", want: 4 * runtime.NumCPU()},
		{name: "environment", env: "8", want: 8},
		{name: "flag wins", flag: "3", env: "8", set: true, want: 3},
		{name: "flag overrides invalid environment", flag: "1", env: "bad", set: true, want: 1},
		{name: "zero", env: "0", fail: true},
		{name: "negative", flag: "-1", set: true, fail: true},
		{name: "empty explicit flag", set: true, fail: true},
		{name: "invalid environment", env: "bad", fail: true},
		{name: "overflow", env: "9999999999999999999999999999", fail: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveWorkers(tt.flag, tt.env, tt.set)
			if (err != nil) != tt.fail || got != tt.want {
				t.Fatalf("got (%d, %v), want (%d, error=%t)", got, err, tt.want, tt.fail)
			}
		})
	}
}
