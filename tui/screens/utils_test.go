package screens

import (
	"strings"
	"testing"
)

func TestTruncatePath_ShortPath(t *testing.T) {
	path := "/short/path"
	if got := truncatePath(path); got != path {
		t.Errorf("truncatePath(%q) = %q, want unchanged", path, got)
	}
}

func TestTruncatePath_ExactlyMaxLen(t *testing.T) {
	path := strings.Repeat("a", MAX_PATH_LEN)
	if got := truncatePath(path); got != path {
		t.Errorf("truncatePath at MAX_PATH_LEN should be unchanged, got %q", got)
	}
}

func TestTruncatePath_ExceedsMaxLen(t *testing.T) {
	path := strings.Repeat("a", MAX_PATH_LEN+10)
	got := truncatePath(path)
	if !strings.HasPrefix(got, "…") {
		t.Errorf("truncatePath long path should start with '…', got %q", got)
	}
	if len([]rune(got)) != MAX_PATH_LEN {
		t.Errorf("truncatePath result len = %d, want %d", len([]rune(got)), MAX_PATH_LEN)
	}
}

func TestTruncatePath_Empty(t *testing.T) {
	if got := truncatePath(""); got != "" {
		t.Errorf("truncatePath('') = %q, want empty", got)
	}
}

func TestDetectEasterEgg_FuckYou(t *testing.T) {
	for _, input := range []string{"fuck you", "FUCK YOU", "Fuck You", "  fuck you  "} {
		if got := detectEasterEgg(input); got == "" {
			t.Errorf("detectEasterEgg(%q) = empty, want easter egg message", input)
		}
	}
}

func TestDetectEasterEgg_TmpPath(t *testing.T) {
	for _, input := range []string{"/tmp", "/tmp/something", "/TMP"} {
		if got := detectEasterEgg(input); got == "" {
			t.Errorf("detectEasterEgg(%q) = empty, want easter egg message", input)
		}
	}
}

func TestDetectEasterEgg_RegularPath(t *testing.T) {
	if got := detectEasterEgg("/home/user/project"); got != "" {
		t.Errorf("detectEasterEgg regular path = %q, want empty", got)
	}
}

func TestDetectEasterEgg_Empty(t *testing.T) {
	if got := detectEasterEgg(""); got != "" {
		t.Errorf("detectEasterEgg('') = %q, want empty", got)
	}
}

func TestIsDangerous_Empty(t *testing.T) {
	if got := isDangerous(""); got != "" {
		t.Errorf("isDangerous('') = %q, want empty", got)
	}
}

func TestIsDangerous_Root(t *testing.T) {
	if got := isDangerous("/"); got == "" {
		t.Error("isDangerous('/') should return a danger message")
	}
}

func TestIsDangerous_SystemDirs(t *testing.T) {
	dirs := []string{
		"/usr", "/usr/local", "/usr/bin",
		"/etc", "/var", "/dev", "/proc", "/sys",
		"/boot", "/bin", "/sbin", "/run",
	}
	for _, d := range dirs {
		if got := isDangerous(d); got == "" {
			t.Errorf("isDangerous(%q) should return a danger message", d)
		}
	}
}

func TestIsDangerous_NodeModulesSegment(t *testing.T) {
	paths := []string{
		"/home/user/node_modules",
		"/home/user/node_modules/myapp",
	}
	for _, p := range paths {
		if got := isDangerous(p); got == "" {
			t.Errorf("isDangerous(%q) should return a danger message", p)
		}
	}
}

func TestIsDangerous_VendorSegment(t *testing.T) {
	if got := isDangerous("/home/user/vendor"); got == "" {
		t.Error("isDangerous('/home/user/vendor') should return a danger message")
	}
}

func TestIsDangerous_SafePaths(t *testing.T) {
	paths := []string{
		"/home/user/project",
		"/Users/me/code/app",
		"/home/user/workspace/kapi",
	}
	for _, p := range paths {
		if got := isDangerous(p); got != "" {
			t.Errorf("isDangerous(%q) = %q, want empty (safe path)", p, got)
		}
	}
}

func TestIsDangerous_DotGit(t *testing.T) {
	if got := isDangerous("/home/user/project/.git"); got == "" {
		t.Error("isDangerous inside .git should return a danger message")
	}
}

func TestScrollWindow_TotalSmallerThanVisible(t *testing.T) {
	start, end := scrollWindow(0, 3, 9)
	if start != 0 || end != 3 {
		t.Errorf("scrollWindow(0, 3, 9) = (%d, %d), want (0, 3)", start, end)
	}
}

func TestScrollWindow_CursorAtStart(t *testing.T) {
	start, end := scrollWindow(0, 20, 9)
	if start != 0 {
		t.Errorf("start = %d, want 0 when cursor at beginning", start)
	}
	if end != 9 {
		t.Errorf("end = %d, want 9", end)
	}
}

func TestScrollWindow_CursorAtEnd(t *testing.T) {
	start, end := scrollWindow(19, 20, 9)
	if end != 20 {
		t.Errorf("end = %d, want 20 (total)", end)
	}
	if start < 0 {
		t.Errorf("start should not be negative, got %d", start)
	}
}

func TestScrollWindow_WindowSizePreserved(t *testing.T) {
	for cursor := 0; cursor < 20; cursor++ {
		start, end := scrollWindow(cursor, 20, 9)
		size := end - start
		if size > 9 {
			t.Errorf("cursor=%d: window size = %d, must not exceed visible=9", cursor, size)
		}
		if start < 0 {
			t.Errorf("cursor=%d: start = %d, must not be negative", cursor, start)
		}
		if end > 20 {
			t.Errorf("cursor=%d: end = %d, must not exceed total=20", cursor, end)
		}
	}
}

func TestScrollWindow_CursorCentered(t *testing.T) {
	start, end := scrollWindow(10, 20, 9)
	if start < 0 || end > 20 || end-start > 9 {
		t.Errorf("scrollWindow(10, 20, 9) = (%d, %d): invalid window", start, end)
	}
	if 10 < start || 10 >= end {
		t.Errorf("cursor 10 outside window [%d, %d)", start, end)
	}
}

func TestIsDangerous_WindowsPaths(t *testing.T) {
	tests := []struct {
		input     string
		dangerous bool
	}{
		{input: `C:\`, dangerous: true},
		{input: `D:`, dangerous: true},
		{input: `C:\Windows\System32`, dangerous: true},
		{input: `c:/windows`, dangerous: true},
		{input: `C:\Program Files\app`, dangerous: true},
		{input: `C:\Program Files (x86)`, dangerous: true},
		{input: `C:\ProgramData\app`, dangerous: true},
		{input: `C:\Users\me\projects\app`, dangerous: false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isDangerous(tt.input) != ""; got != tt.dangerous {
				t.Errorf("isDangerous(%q) dangerous = %v, want %v", tt.input, got, tt.dangerous)
			}
		})
	}
}
