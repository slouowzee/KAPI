package scaffold

import (
	"bytes"
	"sync"
)

const lineTooLongNotice = "[kapi: output line too long, truncated]"

// lineWriter splits command output into lines for onLine. It is used as
// Stdout and Stderr so that exec.Cmd.Wait owns the copy and honours WaitDelay:
// reading pipes manually would block forever when an orphaned child process
// keeps them open.
type lineWriter struct {
	mu       sync.Mutex
	onLine   func(string)
	buf      []byte
	maxLine  int
	skipping bool
	flushed  bool
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.flushed {
		// NOTE: an orphaned process may still write after Wait gave up on it;
		// onLine must not be called once the step is reported as finished.
		return len(p), nil
	}

	n := len(p)
	for len(p) > 0 {
		i := bytes.IndexByte(p, '\n')
		chunk := p
		if i >= 0 {
			chunk = p[:i]
		}
		if !w.skipping {
			w.buf = append(w.buf, chunk...)
			if len(w.buf) > w.maxLine {
				w.buf = w.buf[:0]
				w.skipping = true
				w.onLine(lineTooLongNotice)
			}
		}
		if i < 0 {
			break
		}
		if !w.skipping {
			w.onLine(string(w.buf))
		}
		w.buf = w.buf[:0]
		w.skipping = false
		p = p[i+1:]
	}
	return n, nil
}

// flush emits the last unterminated line and ignores any later write.
func (w *lineWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.buf) > 0 && !w.skipping {
		w.onLine(string(w.buf))
	}
	w.buf = nil
	w.flushed = true
}
