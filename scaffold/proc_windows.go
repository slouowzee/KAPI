//go:build windows

package scaffold

import (
	"os/exec"
	"strconv"
)

// configureCancel stops the whole process tree on cancellation: npm and
// composer run through .cmd wrappers whose children would otherwise survive.
func configureCancel(c *exec.Cmd) {
	c.Cancel = func() error { return terminateTree(c) }
}

// terminateTree kills a started command and all its descendants.
func terminateTree(c *exec.Cmd) error {
	return exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(c.Process.Pid)).Run()
}
