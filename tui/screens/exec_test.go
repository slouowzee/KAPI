package screens

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestCleanupPartial_RemovesNewEntries(t *testing.T) {
	dir := t.TempDir()

	existing := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(existing, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	added := filepath.Join(dir, "added.txt")
	if err := os.WriteFile(added, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	pre := []string{"keep.txt"}
	if err := cleanupPartial(dir, pre); err != nil {
		t.Fatalf("cleanupPartial returned error: %v", err)
	}

	if _, err := os.Stat(existing); err != nil {
		t.Error("existing file was removed, expected it to be kept")
	}
	if _, err := os.Stat(added); err == nil {
		t.Error("added file still exists, expected it to be removed")
	}
}

func TestCleanupPartial_KeepsPreexistingEntries(t *testing.T) {
	tests := []struct {
		name  string
		entry string
		isDir bool
	}{
		{name: "git repository", entry: ".git", isDir: true},
		{name: "node modules", entry: "node_modules", isDir: true},
		{name: "vendor", entry: "vendor", isDir: true},
		{name: "composer lockfile", entry: "composer.lock"},
		{name: "npm lockfile", entry: "package-lock.json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tt.entry)
			if tt.isDir {
				if err := os.Mkdir(path, 0o755); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
				t.Fatal(err)
			}

			if err := cleanupPartial(dir, []string{tt.entry}); err != nil {
				t.Fatalf("cleanupPartial returned error: %v", err)
			}

			if _, err := os.Stat(path); err != nil {
				t.Errorf("%s existed before scaffolding and was removed", tt.entry)
			}
		})
	}
}

func TestCleanupPartial_RemovesNewDependencyDirs(t *testing.T) {
	dir := t.TempDir()

	nm := filepath.Join(dir, "node_modules")
	if err := os.Mkdir(nm, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := cleanupPartial(dir, []string{"README.md"}); err != nil {
		t.Fatalf("cleanupPartial returned error: %v", err)
	}

	if _, err := os.Stat(nm); err == nil {
		t.Error("node_modules was created by the scaffold and should have been removed")
	}
}

func TestCleanupPartial_EmptyPreservesList(t *testing.T) {
	dir := t.TempDir()

	f := filepath.Join(dir, "something.txt")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := cleanupPartial(dir, nil); err != nil {
		t.Fatalf("cleanupPartial returned error: %v", err)
	}

	if _, err := os.Stat(f); err == nil {
		t.Error("file still exists, expected it to be removed")
	}
}

func TestAppendLine_Basic(t *testing.T) {
	m := ExecModel{}
	m.appendLine("hello")
	m.appendLine("world")

	if len(m.outputLines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(m.outputLines))
	}
	if m.outputLines[0] != "hello" || m.outputLines[1] != "world" {
		t.Errorf("unexpected lines: %v", m.outputLines)
	}
}

func TestAppendLine_RingBufferCaps(t *testing.T) {
	m := ExecModel{}
	for i := range outputRingSize + 10 {
		m.appendLine(string(rune('a' + i%26)))
	}

	if len(m.outputLines) != outputRingSize {
		t.Fatalf("expected ring buffer size %d, got %d", outputRingSize, len(m.outputLines))
	}
}

func newExecModelPromptCD() ExecModel {
	m := ExecModel{
		steps:              []ExecStep{{Label: "step1", Fn: func() error { return nil }}},
		promptCD:           true,
		cdCursor:           0,
		shellWrapperActive: true, // simulate wrapper present
	}
	return m
}

func sendKey(m ExecModel, key string) ExecModel {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	switch key {
	case "up":
		msg = tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		msg = tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case " ":
		msg = tea.KeyMsg{Type: tea.KeySpace}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	}
	updated, _ := m.Update(msg)
	return updated
}

func TestExecUpdate_PromptCD_NavigateDown(t *testing.T) {
	m := newExecModelPromptCD()
	if m.cdCursor != 0 {
		t.Fatalf("initial cursor should be 0, got %d", m.cdCursor)
	}
	m = sendKey(m, "down")
	if m.cdCursor != 1 {
		t.Errorf("after down cursor should be 1, got %d", m.cdCursor)
	}
}

func TestExecUpdate_PromptCD_NavigateUp(t *testing.T) {
	m := newExecModelPromptCD()
	m.cdCursor = 1
	m = sendKey(m, "up")
	if m.cdCursor != 0 {
		t.Errorf("after up cursor should be 0, got %d", m.cdCursor)
	}
}

func TestExecUpdate_PromptCD_CursorClamped(t *testing.T) {
	m := newExecModelPromptCD()
	m = sendKey(m, "up")
	if m.cdCursor != 0 {
		t.Errorf("cursor should stay 0 when at top, got %d", m.cdCursor)
	}

	m.cdCursor = 1
	m = sendKey(m, "down")
	if m.cdCursor != 1 {
		t.Errorf("cursor should stay 1 when at bottom, got %d", m.cdCursor)
	}
}

func TestExecUpdate_PromptCD_EnterOnCdOption_SetsCdRequested(t *testing.T) {
	m := newExecModelPromptCD()
	m = sendKey(m, "enter")

	if !m.cdRequested {
		t.Error("cdRequested should be true after enter on 'cd into project'")
	}
	if !m.done {
		t.Error("done should be true after confirming")
	}
}

func TestExecUpdate_PromptCD_SpaceOnCdOption_SetsCdRequested(t *testing.T) {
	m := newExecModelPromptCD()
	m = sendKey(m, " ")

	if !m.cdRequested {
		t.Error("cdRequested should be true after space on 'cd into project'")
	}
	if !m.done {
		t.Error("done should be true after confirming with space")
	}
}

func TestExecUpdate_PromptCD_EnterOnQuitOption_NoCdRequested(t *testing.T) {
	m := newExecModelPromptCD()
	m.cdCursor = 1
	m = sendKey(m, "enter")

	if m.cdRequested {
		t.Error("cdRequested should be false when 'quit' option is selected")
	}
	if !m.done {
		t.Error("done should be true after confirming quit")
	}
}

func TestExecUpdate_PromptCD_EscQuits(t *testing.T) {
	m := newExecModelPromptCD()
	m = sendKey(m, "esc")

	if m.cdRequested {
		t.Error("cdRequested should be false after esc")
	}
	if !m.done {
		t.Error("done should be true after esc")
	}
}

func TestExecUpdate_PromptCD_JKeyNavigatesDown(t *testing.T) {
	m := newExecModelPromptCD()
	m = sendKey(m, "j")
	if m.cdCursor != 1 {
		t.Errorf("'j' should move cursor down to 1, got %d", m.cdCursor)
	}
}

func TestExecUpdate_PromptCD_KKeyNavigatesUp(t *testing.T) {
	m := newExecModelPromptCD()
	m.cdCursor = 1
	m = sendKey(m, "k")
	if m.cdCursor != 0 {
		t.Errorf("'k' should move cursor up to 0, got %d", m.cdCursor)
	}
}

func TestExecModel_CdRequestedAccessor(t *testing.T) {
	m := ExecModel{cdRequested: true}
	if !m.CdRequested() {
		t.Error("CdRequested() should return true when cdRequested is true")
	}
	m.cdRequested = false
	if m.CdRequested() {
		t.Error("CdRequested() should return false when cdRequested is false")
	}
}

// ---------------------------------------------------------------------------
// shellWrapperActive — détection du wrapper
// ---------------------------------------------------------------------------

func TestExecUpdate_AllDone_AlwaysSetsPromptCD(t *testing.T) {
	// execAllDoneMsg always enables promptCD regardless of wrapper presence.
	for _, wrapper := range []bool{false, true} {
		m := ExecModel{shellWrapperActive: wrapper}
		updated, _ := m.Update(execAllDoneMsg{})

		if !updated.promptCD {
			t.Errorf("promptCD should be true after execAllDoneMsg (shellWrapperActive=%v)", wrapper)
		}
		if updated.done {
			t.Errorf("done should be false until user confirms (shellWrapperActive=%v)", wrapper)
		}
	}
}

// ---------------------------------------------------------------------------
// View() — prompt CD rendering based on shell wrapper presence
// ---------------------------------------------------------------------------

func newExecModelPromptCDNoWrapper() ExecModel {
	return ExecModel{
		steps:              []ExecStep{{Label: "step1", Fn: func() error { return nil }}},
		promptCD:           true,
		cdCursor:           0,
		shellWrapperActive: false,
	}
}

func TestExecView_PromptCD_WithWrapper_ShowsNavigationMenu(t *testing.T) {
	m := newExecModelPromptCD()
	view := m.View()

	if !contains(view, "cd into project") {
		t.Error("View() with wrapper should contain 'cd into project'")
	}
	if !contains(view, "quit") {
		t.Error("View() with wrapper should contain 'quit'")
	}
	if !contains(view, "[←→]") {
		t.Error("View() with wrapper should contain navigation hint '[←→]'")
	}
}

func TestExecView_PromptCD_NoWrapper_ShowsSimpleHint(t *testing.T) {
	m := newExecModelPromptCDNoWrapper()
	view := m.View()

	if contains(view, "cd into project") {
		t.Error("View() without wrapper must not show 'cd into project'")
	}
	if contains(view, "[←→]") {
		t.Error("View() without wrapper must not show navigation hint '[←→]'")
	}
	if !contains(view, "[↵] quit") {
		t.Error("View() without wrapper should show '[↵] quit'")
	}
}

func TestExecView_PromptCD_NoWrapper_AnyKeyQuits(t *testing.T) {
	for _, key := range []string{"enter", " ", "esc", "q"} {
		m := newExecModelPromptCDNoWrapper()
		m = sendKey(m, key)
		if !m.done {
			t.Errorf("key %q should set done=true without wrapper", key)
		}
		if m.cdRequested {
			t.Errorf("key %q should not set cdRequested without wrapper", key)
		}
	}
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && (s == sub || len(s) > 0 && containsRune(s, sub))
}

func containsRune(s, sub string) bool {
	for i := range s {
		if i+len(sub) <= len(s) && s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Navigation left/right dans le promptCD
// ---------------------------------------------------------------------------

func TestExecUpdate_PromptCD_RightKeyNavigatesDown(t *testing.T) {
	m := newExecModelPromptCD()
	m = sendKey(m, "right")
	if m.cdCursor != 1 {
		t.Errorf("'right' should move cursor to 1, got %d", m.cdCursor)
	}
}

func TestExecUpdate_PromptCD_LeftKeyNavigatesUp(t *testing.T) {
	m := newExecModelPromptCD()
	m.cdCursor = 1
	m = sendKey(m, "left")
	if m.cdCursor != 0 {
		t.Errorf("'left' should move cursor to 0, got %d", m.cdCursor)
	}
}

func TestExecUpdate_PromptCD_HKeyNavigatesUp(t *testing.T) {
	m := newExecModelPromptCD()
	m.cdCursor = 1
	m = sendKey(m, "h")
	if m.cdCursor != 0 {
		t.Errorf("'h' should move cursor to 0, got %d", m.cdCursor)
	}
}

func TestExecUpdate_PromptCD_LKeyNavigatesDown(t *testing.T) {
	m := newExecModelPromptCD()
	m = sendKey(m, "l")
	if m.cdCursor != 1 {
		t.Errorf("'l' should move cursor to 1, got %d", m.cdCursor)
	}
}

func newFailedExecModel(t *testing.T, existedBefore bool) (ExecModel, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "app")
	if existedBefore {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := NewExec(80, 24, []ExecStep{{Label: "step1", Fn: func() error { return nil }}}, dir)
	return m, dir
}

func TestExecUpdate_StepError_AsksBeforeCleanup(t *testing.T) {
	m, dir := newFailedExecModel(t, false)
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}

	updated, cmd := m.Update(execStepDoneMsg{err: os.ErrPermission})

	if !updated.confirmCleanup {
		t.Fatal("confirmCleanup should be true after a failed step that created files")
	}
	if updated.cleaningUp || cmd != nil {
		t.Error("cleanup must not start before the user confirms")
	}
	if updated.cleanupCursor != cleanupOptionKeep {
		t.Errorf("default choice should keep files, got %d", updated.cleanupCursor)
	}
}

func TestExecUpdate_StepError_NothingCreated_Done(t *testing.T) {
	m, _ := newFailedExecModel(t, false)

	updated, _ := m.Update(execStepDoneMsg{err: os.ErrPermission})

	if updated.confirmCleanup {
		t.Error("confirmCleanup should be false when nothing was created")
	}
	if !updated.done || !updated.HasErr() {
		t.Error("model should be done with an error")
	}
}

func TestExecUpdate_ConfirmCleanup(t *testing.T) {
	tests := []struct {
		name        string
		keys        []string
		wantRemoved bool
	}{
		{name: "remove created files", keys: []string{"left", "enter"}, wantRemoved: true},
		{name: "keep files with enter", keys: []string{"enter"}, wantRemoved: false},
		{name: "keep files with esc", keys: []string{"left", "esc"}, wantRemoved: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, dir := newFailedExecModel(t, true)
			created := filepath.Join(dir, "created.txt")
			if err := os.WriteFile(created, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			m, _ = m.Update(execStepDoneMsg{err: os.ErrPermission})

			var cmd tea.Cmd
			for _, k := range tt.keys {
				msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
				switch k {
				case "left":
					msg = tea.KeyMsg{Type: tea.KeyLeft}
				case "enter":
					msg = tea.KeyMsg{Type: tea.KeyEnter}
				case "esc":
					msg = tea.KeyMsg{Type: tea.KeyEsc}
				}
				m, cmd = m.Update(msg)
			}
			if cmd != nil {
				m, _ = m.Update(cmd())
			}

			_, err := os.Stat(created)
			if removed := os.IsNotExist(err); removed != tt.wantRemoved {
				t.Errorf("created file removed = %v, want %v", removed, tt.wantRemoved)
			}
			if _, err := os.Stat(filepath.Join(dir, "keep.txt")); err != nil {
				t.Error("pre-existing file must always be kept")
			}
			if !m.done {
				t.Error("model should be done after the cleanup choice")
			}
		})
	}
}

func ctrlC() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyCtrlC} }

func TestExecUpdate_AbortNeedsConfirmation(t *testing.T) {
	m := NewExec(80, 24, []ExecStep{{Label: "step1", Fn: func() error { return nil }}}, "")

	m, _ = m.Update(ctrlC())
	if !m.abortPending || m.aborted {
		t.Fatalf("first ctrl+c should only ask for confirmation (pending=%v aborted=%v)", m.abortPending, m.aborted)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.abortPending {
		t.Fatal("esc should cancel the pending abort")
	}

	m, _ = m.Update(ctrlC())
	m, _ = m.Update(ctrlC())
	if !m.aborted {
		t.Fatal("second ctrl+c should abort")
	}
	if m.ctx.Err() == nil {
		t.Error("aborting should cancel the running commands")
	}

	m, _ = m.Update(execStepDoneMsg{})
	if !errors.Is(m.Err(), errAborted) {
		t.Errorf("Err() = %v, want errAborted even if the step succeeded", m.Err())
	}
	if m.IsBusy() {
		t.Error("model should not be busy after the abort")
	}
}

func TestExecUpdate_QIgnoredWhileBusy(t *testing.T) {
	m := NewExec(80, 24, []ExecStep{{Label: "step1", Fn: func() error { return nil }}}, "")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if m.done || m.abortPending {
		t.Error("q must not stop a running scaffold")
	}
}

func TestFitOutputLine(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		width int
		want  string
	}{
		{name: "short line unchanged", line: "added 12 packages", width: 40, want: "added 12 packages"},
		{name: "ascii truncated", line: "abcdefghij", width: 5, want: "abcd…"},
		{name: "multi-byte characters stay valid", line: "ééééééééé", width: 5, want: "éééé…"},
		{name: "carriage return keeps last update", line: "progress 10%\rprogress 90%\rdone", width: 40, want: "done"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fitOutputLine(tt.line, tt.width)
			if got != tt.want {
				t.Errorf("fitOutputLine(%q, %d) = %q, want %q", tt.line, tt.width, got, tt.want)
			}
			if !utf8.ValidString(got) {
				t.Errorf("result %q is not valid UTF-8", got)
			}
		})
	}
}

func TestFitOutputLine_KeepsANSISequencesIntact(t *testing.T) {
	line := "\x1b[32m" + strings.Repeat("x", 50) + "\x1b[0m"
	got := fitOutputLine(line, 10)
	if w := ansi.StringWidth(got); w > 10 {
		t.Errorf("display width = %d, want <= 10", w)
	}
	if !strings.HasPrefix(got, "\x1b[32m") {
		t.Errorf("color sequence was cut: %q", got)
	}
}
