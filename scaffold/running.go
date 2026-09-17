package scaffold

import (
	"os/exec"
	"sync"
)

// running tracks the streamed commands in progress so they can be stopped
// when kapi itself exits: they run in their own process group and would not
// receive the terminal's signals.
var running = struct {
	mu    sync.Mutex
	procs map[*exec.Cmd]struct{}
}{procs: make(map[*exec.Cmd]struct{})}

func trackRunning(c *exec.Cmd) (untrack func()) {
	running.mu.Lock()
	running.procs[c] = struct{}{}
	running.mu.Unlock()
	return func() {
		running.mu.Lock()
		delete(running.procs, c)
		running.mu.Unlock()
	}
}

// StopRunningCommands terminates every streamed command still running,
// together with the processes it spawned.
func StopRunningCommands() {
	running.mu.Lock()
	defer running.mu.Unlock()
	for c := range running.procs {
		if c.Process != nil {
			_ = terminateTree(c)
		}
	}
}
