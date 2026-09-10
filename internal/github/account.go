package github

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type authAccount struct {
	Login  string `json:"login"`
	Active bool   `json:"active"`
	State  string `json:"state"`
}

// accountPlan interprets gh's JSON, whose exit status does not indicate whether
// individual accounts are authenticated. Unrelated account failures are ignored.
func accountPlan(data []byte, user string, switchBack bool) (target, previous string, err error) {
	var status struct {
		Hosts map[string][]authAccount `json:"hosts"`
	}
	if json.Unmarshal(data, &status) != nil {
		return "", "", fmt.Errorf("cannot parse gh auth status JSON; update GitHub CLI")
	}
	var selected *authAccount
	for _, account := range status.Hosts["github.com"] {
		if account.Active {
			previous = account.Login
		}
		if strings.EqualFold(account.Login, user) {
			selected = &account
		}
	}
	if selected == nil || selected.State != "success" {
		return "", "", fmt.Errorf("GitHub CLI account %q is not authenticated on github.com; run gh auth login --hostname github.com for that account", user)
	}
	if selected.Active {
		return "", "", nil
	}
	if switchBack && previous == "" {
		return "", "", fmt.Errorf("cannot switch accounts: no previous active github.com account to restore")
	}
	if !switchBack {
		previous = ""
	}
	return selected.Login, previous, nil
}

// SelectAccount selects a healthy stored github.com account and reads its token.
// The caller must defer restore even when an error is returned: token retrieval
// can fail after a successful switch. No command output containing secrets is logged.
func SelectAccount(user string, switchBack bool, switched func(string)) (token string, restore func() error, err error) {
	if os.Getenv("GH_TOKEN") != "" || os.Getenv("GITHUB_TOKEN") != "" {
		return "", nil, fmt.Errorf("auth_user requires GitHub CLI credentials; unset GH_TOKEN and GITHUB_TOKEN")
	}
	data, err := exec.Command("gh", "auth", "status", "--hostname", "github.com", "--json", "hosts").Output()
	if err != nil {
		return "", nil, fmt.Errorf("checking GitHub CLI authentication (requires gh auth status --json support): %w", err)
	}
	target, previous, err := accountPlan(data, user, switchBack)
	if err != nil {
		return "", nil, err
	}
	if target != "" {
		if err := switchAccount(target, switched); err != nil {
			return "", nil, err
		}
		if previous != "" {
			restore = func() error {
				if err := switchAccount(previous, switched); err != nil {
					return fmt.Errorf("restoring previous GitHub CLI account: %w", err)
				}
				return nil
			}
		}
	}
	data, err = exec.Command("gh", "auth", "token", "--hostname", "github.com").Output()
	if err != nil {
		return "", restore, fmt.Errorf("reading GitHub CLI token for %q: %w", user, err)
	}
	token = strings.TrimSpace(string(data))
	if token == "" {
		return "", restore, fmt.Errorf("GitHub CLI returned an empty token for %q", user)
	}
	return token, restore, nil
}

func switchAccount(user string, switched func(string)) error {
	if err := exec.Command("gh", "auth", "switch", "--hostname", "github.com", "--user", user).Run(); err != nil {
		return fmt.Errorf("switching GitHub CLI account to %q: %w", user, err)
	}
	if switched != nil {
		switched(user)
	}
	return nil
}
