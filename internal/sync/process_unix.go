//go:build linux || darwin

package sync

import (
	"errors"
	"os/exec"
	"syscall"
)

type processTree struct{ cmd *exec.Cmd }

func prepareProcessTree(cmd *exec.Cmd) (*processTree, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return &processTree{cmd: cmd}, nil
}

func (p *processTree) started() error { return nil }

func (p *processTree) kill() error {
	err := syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

func (p *processTree) close() {}
