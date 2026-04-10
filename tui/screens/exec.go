package screens

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slouowzee/kapi/tui/styles"
)

type ExecStep struct {
	Label    string
	Cmd      *exec.Cmd
	Fn       func() error
	StreamFn func(onLine func(string)) error
}

type execStepDoneMsg struct{ err error }
type execAllDoneMsg struct{}
type execOutputLineMsg struct{ line string }
type execCleanupDoneMsg struct{ err error }
type execStreamStartMsg struct{ ch chan streamResult }

const outputRingSize = 1000

type ExecModel struct {
	width  int
	height int

	steps   []ExecStep
	current int

	streamChan  chan streamResult
	outputLines []string

	targetDir        string
	dirExistedBefore bool
	preDirEntries    []string

	shellWrapperActive bool
	cleaningUp         bool
	promptCD           bool
	cdCursor           int
	cdRequested        bool
	done               bool
	lastErr            error
	returnToRecap      bool
}

type streamResult struct {
	line string
	done bool
	err  error
}

func NewExec(width, height int, steps []ExecStep, targetDir string) ExecModel {
	m := ExecModel{
		width:              width,
		height:             height,
		steps:              steps,
		targetDir:          targetDir,
		shellWrapperActive: os.Getenv("KAPI_SHELL_WRAPPER") == "1",
	}

	if targetDir != "" {
		_, err := os.Stat(targetDir)
		m.dirExistedBefore = err == nil
		if m.dirExistedBefore {
			if entries, readErr := os.ReadDir(targetDir); readErr == nil {
				for _, e := range entries {
					m.preDirEntries = append(m.preDirEntries, e.Name())
				}
			}
		}
	}

	return m
}

func (m *ExecModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m ExecModel) Done() bool                { return m.done }
func (m ExecModel) HasErr() bool              { return m.lastErr != nil }
func (m ExecModel) Err() error                { return m.lastErr }
func (m ExecModel) CdRequested() bool         { return m.cdRequested }
func (m ExecModel) ShouldReturnToRecap() bool { return m.returnToRecap }
func (m *ExecModel) ConsumeReturnToRecap()    { m.returnToRecap = false }

func (m ExecModel) Init() tea.Cmd {
	if len(m.steps) == 0 {
		return func() tea.Msg { return execAllDoneMsg{} }
	}
	return m.runCurrentStep()
}

func (m ExecModel) Update(msg tea.Msg) (ExecModel, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case execOutputLineMsg:
		m.appendLine(msg.line)
		return m, m.readOneResult()

	case execStreamStartMsg:
		m.streamChan = msg.ch
		return m, m.readOneResult()

	case execAllDoneMsg:
		m.promptCD = true
		return m, nil

	case execStepDoneMsg:
		m.streamChan = nil
		if msg.err != nil {
			m.lastErr = msg.err
			if m.targetDir == "" {
				m.done = true
				return m, nil
			}
			m.cleaningUp = true
			return m, m.runCleanup()
		}
		m.current++
		if m.current >= len(m.steps) {
			return m, func() tea.Msg { return execAllDoneMsg{} }
		}
		if len(m.outputLines) > 0 {
			m.outputLines = append(m.outputLines, "")
		}
		return m, m.runCurrentStep()

	case execCleanupDoneMsg:
		m.cleaningUp = false
		m.done = true
		return m, nil

	case tea.KeyMsg:
		if m.promptCD {
			if !m.shellWrapperActive {
				// No wrapper: any key quits
				switch msg.String() {
				case " ", "enter", "esc", "q":
					m.done = true
				}
				break
			}
			switch msg.String() {
			case "up", "k", "left", "h":
				if m.cdCursor > 0 {
					m.cdCursor--
				}
			case "down", "j", "right", "l":
				if m.cdCursor < 1 {
					m.cdCursor++
				}
			case " ", "enter":
				if m.cdCursor == 0 {
					m.cdRequested = true
				}
				m.done = true
			case "esc":
				m.done = true
			}
			break
		}
		if !m.done || m.cleaningUp {
			break
		}
		if m.lastErr != nil && msg.String() == "enter" {
			m.returnToRecap = true
		}
	}

	return m, nil
}

func (m ExecModel) runCurrentStep() tea.Cmd {
	step := m.steps[m.current]

	switch {
	case step.StreamFn != nil:
		ch := make(chan streamResult, 64)
		fn := step.StreamFn
		go func() {
			err := fn(func(line string) {
				ch <- streamResult{line: line}
			})
			ch <- streamResult{done: true, err: err}
			close(ch)
		}()
		return func() tea.Msg { return execStreamStartMsg{ch: ch} }

	case step.Fn != nil:
		fn := step.Fn
		return func() tea.Msg {
			return execStepDoneMsg{err: fn()}
		}

	default:
		c := step.Cmd
		return tea.ExecProcess(c, func(err error) tea.Msg {
			return execStepDoneMsg{err: err}
		})
	}
}

func (m ExecModel) readOneResult() tea.Cmd {
	ch := m.streamChan
	return func() tea.Msg {
		r, ok := <-ch
		if !ok {
			return execStepDoneMsg{}
		}
		if r.done {
			return execStepDoneMsg{err: r.err}
		}
		return execOutputLineMsg{line: r.line}
	}
}

func (m *ExecModel) appendLine(line string) {
	m.outputLines = append(m.outputLines, line)
	if len(m.outputLines) > outputRingSize {
		m.outputLines = m.outputLines[len(m.outputLines)-outputRingSize:]
	}
}

func (m ExecModel) runCleanup() tea.Cmd {
	targetDir := m.targetDir
	existedBefore := m.dirExistedBefore
	preDirEntries := m.preDirEntries
	return func() tea.Msg {
		var err error
		if !existedBefore {
			err = os.RemoveAll(targetDir)
		} else {
			err = cleanupPartial(targetDir, preDirEntries)
		}
		return execCleanupDoneMsg{err: err}
	}
}

func cleanupPartial(targetDir string, preDirEntries []string) error {
	pre := make(map[string]struct{}, len(preDirEntries))
	for _, name := range preDirEntries {
		pre[name] = struct{}{}
	}

	alwaysRemove := map[string]struct{}{
		"node_modules":      {},
		"vendor":            {},
		"composer.lock":     {},
		"package-lock.json": {},
		"yarn.lock":         {},
		"pnpm-lock.yaml":    {},
		"bun.lock":          {},
		".git":              {},
	}

	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		_, wasAlreadyThere := pre[name]
		_, alwaysDel := alwaysRemove[name]
		if !wasAlreadyThere || alwaysDel {
			if removeErr := os.RemoveAll(filepath.Join(targetDir, name)); removeErr != nil && err == nil {
				err = removeErr
			}
		}
	}
	return err
}

func (m ExecModel) View() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(styles.TitleStyle.Render("  Scaffolding") + "\n")
	sb.WriteString("\n")

	for i, step := range m.steps {
		var line string
		switch {
		case i < m.current:
			line = styles.SuccessStyle.Render("  ✓ ") + styles.DimStyle.Render(step.Label)
		case i == m.current:
			line = styles.CursorStyle.Render("  > ") + styles.SelectedStyle.Render(step.Label)
		default:
			line = styles.DimStyle.Render("    " + step.Label)
		}
		sb.WriteString(line + "\n")
	}

	sb.WriteString("\n")

	switch {
	case m.cleaningUp:
		sb.WriteString(styles.MutedStyle.Render("  Cleaning up...") + "\n")
	case m.done && m.lastErr != nil:
		sb.WriteString(styles.ErrorStyle.Render(fmt.Sprintf("  ✗ Error: %s", m.lastErr)) + "\n")
		sb.WriteString(styles.MutedStyle.Render("  [↵] back to summary") + "\n")
	case m.promptCD:
		sb.WriteString(styles.SuccessStyle.Render(fmt.Sprintf("  All %d steps completed.", len(m.steps))) + "\n")
		sb.WriteString("\n")
		if m.shellWrapperActive {
			cdOpts := []string{"cd into project", "quit"}
			var optStr strings.Builder
			for i, opt := range cdOpts {
				if i > 0 {
					optStr.WriteString(styles.DimStyle.Render("  ·  "))
				}
				if i == m.cdCursor {
					optStr.WriteString(styles.SelectedStyle.Render(opt))
				} else {
					optStr.WriteString(styles.DimStyle.Render(opt))
				}
			}
			sb.WriteString(fmt.Sprintf("%s%s\n",
				styles.CursorStyle.Render("  ❯❯"),
				styles.SelectedStyle.Render("  ")+optStr.String(),
			))
			sb.WriteString("\n")
			sb.WriteString(styles.MutedStyle.Render("  [←→] navigate   [space / ↵] confirm") + "\n")
		} else {
			sb.WriteString(styles.MutedStyle.Render("  [↵] quit") + "\n")
		}
	case m.done:
		sb.WriteString(styles.SuccessStyle.Render(fmt.Sprintf("  All %d steps completed.", len(m.steps))) + "\n")
	default:
		sb.WriteString(styles.DimStyle.Render(fmt.Sprintf("  Step %d / %d", m.current+1, len(m.steps))) + "\n")
	}

	if len(m.outputLines) > 0 && !m.promptCD {
		sb.WriteString("\n")
		sb.WriteString(m.renderOutputPanel() + "\n")
	}

	return sb.String()
}

func (m ExecModel) visibleLines() int {
	n := m.height - 8 - len(m.steps)
	if n < 5 {
		return 5
	}
	if n > 40 {
		return 40
	}
	return n
}

func (m ExecModel) renderOutputPanel() string {
	lines := m.outputLines
	if vis := m.visibleLines(); len(lines) > vis {
		lines = lines[len(lines)-vis:]
	}

	const hMargin = 2
	panelWidth := m.width - 2*hMargin - 2
	if panelWidth < 40 {
		panelWidth = 40
	}

	inner := panelWidth - 2
	rendered := make([]string, len(lines))
	for i, l := range lines {
		if len(l) > inner {
			l = l[:inner-1] + "…"
		}
		rendered[i] = l
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.COLOR_DIM).
		Padding(0, 1).
		Width(panelWidth).
		Render(strings.Join(rendered, "\n"))

	indent := strings.Repeat(" ", hMargin)
	indented := strings.ReplaceAll(box, "\n", "\n"+indent)
	return indent + indented
}
