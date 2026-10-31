package config

import (
	"testing"
	"time"
)

func TestResolveGitIdleTimeout(t *testing.T) {
	for _, tc := range []struct {
		name, flag, env string
		set             bool
		want            time.Duration
		wantErr         bool
	}{
		{name: "two-minute default", want: 2 * time.Minute},
		{name: "environment", env: "90s", want: 90 * time.Second},
		{name: "flag overrides invalid environment", flag: "2m", env: "bad", set: true, want: 2 * time.Minute},
		{name: "explicit disable", flag: "0", env: "2m", set: true},
		{name: "environment disable", env: "0"},
		{name: "invalid environment", env: "bad", wantErr: true},
		{name: "negative", flag: "-1s", set: true, wantErr: true},
		{name: "missing unit", flag: "60", set: true, wantErr: true},
		{name: "explicit empty flag", env: "2m", set: true, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveGitIdleTimeout(tc.flag, tc.env, tc.set)
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("got (%v, %v), want (%v, error=%t)", got, err, tc.want, tc.wantErr)
			}
		})
	}
}
