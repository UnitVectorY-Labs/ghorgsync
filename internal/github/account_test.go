package github

import "testing"

func TestAccountPlan(t *testing.T) {
	const accounts = `{"hosts":{"github.com":[{"login":"personal","active":true,"state":"success"},{"login":"work","active":false,"state":"success"},{"login":"expired","state":"error"}]}}`
	for _, tt := range []struct {
		name, data, user string
		back             bool
		target, previous string
		wantErr          bool
	}{
		{"already active", accounts, "personal", false, "", "", false},
		{"already active with restore", accounts, "PERSONAL", true, "", "", false},
		{"switch and leave", accounts, "work", false, "work", "", false},
		{"switch and restore", accounts, "WORK", true, "work", "personal", false},
		{"missing", accounts, "missing", true, "", "", true},
		{"unhealthy", accounts, "expired", false, "", "", true},
		{"invalid JSON", `bad`, "work", false, "", "", true},
		{"no accounts", `{"hosts":{}}`, "work", false, "", "", true},
		{"wrong host", `{"hosts":{"example.com":[{"login":"work","state":"success"}]}}`, "work", false, "", "", true},
		{"no previous with restore", `{"hosts":{"github.com":[{"login":"work","state":"success"}]}}`, "work", true, "", "", true},
		{"no previous without restore", `{"hosts":{"github.com":[{"login":"work","state":"success"}]}}`, "work", false, "work", "", false},
		{"active invalid", `{"hosts":{"github.com":[{"login":"work","active":true,"state":"error"}]}}`, "work", true, "", "", true},
		{"timeout", `{"hosts":{"github.com":[{"login":"work","state":"timeout"}]}}`, "work", false, "", "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			target, previous, err := accountPlan([]byte(tt.data), tt.user, tt.back)
			if (err != nil) != tt.wantErr || target != tt.target || previous != tt.previous {
				t.Fatalf("got (%q, %q, %v), want (%q, %q), error=%v", target, previous, err, tt.target, tt.previous, tt.wantErr)
			}
		})
	}
}
