//go:build windows

package scaffold

import "os/exec"

// configureCancel keeps the default behaviour on Windows, where
// exec.CommandContext kills the process on cancellation.
func configureCancel(*exec.Cmd) {}

// terminateTree kills a started command.
func terminateTree(c *exec.Cmd) error {
	return c.Process.Kill()
}
