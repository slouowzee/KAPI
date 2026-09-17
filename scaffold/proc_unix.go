//go:build !windows

package scaffold

import (
	"os/exec"
	"syscall"
)

// configureCancel runs the command in its own process group so that aborting
// also stops the processes it spawned (npm, composer and git all fork).
func configureCancel(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Cancel = func() error { return terminateTree(c) }
}

// terminateTree signals the whole process group of a started command.
func terminateTree(c *exec.Cmd) error {
	return syscall.Kill(-c.Process.Pid, syscall.SIGTERM)
}
