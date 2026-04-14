package screens

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slouowzee/kapi/internal/config"
	"github.com/slouowzee/kapi/internal/ecosystem"
	"github.com/slouowzee/kapi/internal/registry"
	"github.com/slouowzee/kapi/internal/trends"
	"github.com/slouowzee/kapi/tui/styles"
)

type frameworksLoadedMsg struct {
	frameworks []registry.Framework
	err        error
}

type trendsLoadedMsg struct {
	frameworkID string
	stats       trends.Stats
}

func loadFrameworksCmd() tea.Cmd {
	return func() tea.Msg {
		fw, err := registry.Load()
		return frameworksLoadedMsg{frameworks: fw, err: err}
	}
}

func loadTrendsCmd(fw registry.Framework) tea.Cmd {
	return func() tea.Msg {
		stats := trends.Fetch(context.Background(), fw.NpmPackage, fw.PackagistPackage, fw.GithubRepo, config.GithubToken())
		return trendsLoadedMsg{frameworkID: fw.ID, stats: stats}
	}
}

type FrameworkModel struct {
	width  int
	height int

	ecosystem string
	targetDir string

	all     []registry.Framework
	visible []registry.Framework

	query  string
	cursor int

	loading bool
	loadErr error

	statsCache map[string]trends.Stats

	selected registry.Framework
	done     bool

	backPressed bool
}

func NewFramework(width, height int, eco ecosystem.Ecosystem, targetDir string) FrameworkModel {
	ecoStr := "php"
	if eco == ecosystem.ECOSYSTEM_JS {
		ecoStr = "js"
	}
	return FrameworkModel{
		width:      width,
		height:     height,
		ecosystem:  ecoStr,
		targetDir:  targetDir,
		loading:    true,
		statsCache: make(map[string]trends.Stats),
	}
}

func (m *FrameworkModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m FrameworkModel) SelectedFramework() registry.Framework { return m.selected }
func (m FrameworkModel) Done() bool                            { return m.done }
func (m FrameworkModel) IsBack() bool                          { return m.backPressed }

func (m *FrameworkModel) ConsumeBack() { m.backPressed = false }
func (m *FrameworkModel) ConsumeDone() { m.done = false }

func (m FrameworkModel) Init() tea.Cmd {
	return loadFrameworksCmd()
}

func (m FrameworkModel) currentFramework() (registry.Framework, bool) {
	if len(m.visible) == 0 || m.cursor >= len(m.visible) {
		return registry.Framework{}, false
	}
	return m.visible[m.cursor], true
}

func (m FrameworkModel) Update(msg tea.Msg) (FrameworkModel, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case frameworksLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.loadErr = msg.err
			break
		}
		for _, fw := range msg.frameworks {
			if fw.Ecosystem == m.ecosystem {
				m.all = append(m.all, fw)
			}
		}
		m.visible = m.all
		m.cursor = 0
		if len(m.all) > 0 {
			cmds := make([]tea.Cmd, len(m.all))
			for i, fw := range m.all {
				cmds[i] = loadTrendsCmd(fw)
			}
			return m, tea.Batch(cmds...)
		}

	case trendsLoadedMsg:
		m.statsCache[msg.frameworkID] = msg.stats

	case tea.KeyMsg:
		if m.loading {
			if msg.String() == "esc" {
				m.backPressed = true
			}
			break
		}
		switch msg.String() {
		case "esc":
			if m.query != "" {
				m.query = ""
				m.visible = m.all
				m.cursor = 0
			} else {
				m.backPressed = true
			}
		case "enter":
			if len(m.visible) > 0 {
				m.selected = m.visible[m.cursor]
				m.done = true
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.visible)-1 {
				m.cursor++
			}
		case "backspace":
			if len(m.query) > 0 {
				runes := []rune(m.query)
				m.query = string(runes[:len(runes)-1])
				m.visible = fuzzyFilter(m.all, m.query)
				m.cursor = 0
			}
		default:
			if len(msg.Runes) > 0 {
				m.query += string(msg.Runes)
				m.visible = fuzzyFilter(m.all, m.query)
				m.cursor = 0
			}
		}
	}

	return m, nil
}

func layoutWidths(totalWidth int) (listWidth, panelWidth int) {
	const margin = 6
	avail := totalWidth - margin
	listWidth = (avail * 6) / 10
	if listWidth < 30 {
		listWidth = 30
	}
	panelWidth = avail - listWidth - 3
	if panelWidth < 20 {
		panelWidth = 20
	}
	return
}

func (m FrameworkModel) View() string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(styles.TitleStyle.Render("  Which framework?") + "\n")
	sb.WriteString(styles.DimStyle.Render("  "+truncatePath(m.targetDir)) + "\n")
	sb.WriteString("\n")

	if m.loading {
		sb.WriteString(styles.DimStyle.Render("  Loading…") + "\n")
		sb.WriteString("\n")
		sb.WriteString(styles.MutedStyle.Render("  [esc] back   [ctrl+c] quit") + "\n")
		return sb.String()
	}

	if m.loadErr != nil {
		sb.WriteString(styles.ErrorStyle.Render("  ✗ Failed to load frameworks: "+m.loadErr.Error()) + "\n")
		sb.WriteString("\n")
		sb.WriteString(styles.MutedStyle.Render("  [esc] back   [ctrl+c] quit") + "\n")
		return sb.String()
	}

	listWidth, panelWidth := layoutWidths(m.width)
	boxHeight := 13

	leftCol := m.renderList(listWidth)
	rightCol := m.renderStats()

	leftStyle := lipgloss.NewStyle().
		Width(listWidth).
		Height(boxHeight).
		PaddingRight(2).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(styles.COLOR_MUTED)

	rightStyle := lipgloss.NewStyle().
		Width(panelWidth).
		Height(boxHeight).
		PaddingLeft(2)

	row := lipgloss.JoinHorizontal(lipgloss.Top,
		leftStyle.Render(leftCol),
		rightStyle.Render(rightCol),
	)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.COLOR_MUTED).
		MarginLeft(2).
		Padding(0, 1).
		Render(row)

	sb.WriteString(box + "\n\n")

	var hints string
	if m.query != "" {
		hints = "  [↑↓] navigate   [↵] select   [esc] clear   [ctrl+c] quit"
	} else {
		hints = "  [↑↓] navigate   [↵] select   [esc] back   [ctrl+c] quit"
	}
	sb.WriteString(styles.MutedStyle.Render(hints) + "\n")

	return sb.String()
}

func (m FrameworkModel) renderList(listWidth int) string {
	var sb strings.Builder

	sepWidth := listWidth

	if m.query == "" {
		sb.WriteString(styles.MutedStyle.Render(" Search: ") +
			styles.DimStyle.Render("type to filter…") +
			styles.TitleStyle.Render("_") + "\n")
	} else {
		sb.WriteString(styles.MutedStyle.Render(" Search: ") +
			m.query +
			styles.TitleStyle.Render("_") + "\n")
	}

	if sepWidth > 2 {
		sepWidth -= 2
	}
	separator := styles.DimStyle.Render(strings.Repeat("─", sepWidth))

	if len(m.visible) == 0 {
		sb.WriteString(separator + "\n")
		sb.WriteString(styles.DimStyle.Render(" No match") + "\n")
		return sb.String()
	}

	sb.WriteString(separator + "\n")

	const VISIBLE = 9
	total := len(m.visible)

	windowStart, windowEnd := scrollWindow(m.cursor, total, VISIBLE)

	if windowStart > 0 {
		sb.WriteString(styles.DimStyle.Render(fmt.Sprintf("    ↑ %d more", windowStart)) + "\n")
	}

	for i, fw := range m.visible[windowStart:windowEnd] {
		absIdx := windowStart + i
		if absIdx == m.cursor {
			cur := styles.CursorStyle.Render(" ❯❯")
			label := styles.SelectedStyle.Render(" " + fw.Name)
			fmt.Fprintf(&sb, "%s%s\n", cur, label)
		} else {
			fmt.Fprintf(&sb, "    %s\n", styles.DimStyle.Render(fw.Name))
		}
	}

	if windowEnd < total {
		sb.WriteString(styles.DimStyle.Render(fmt.Sprintf("    ↓ %d more", total-windowEnd)) + "\n")
	}

	return sb.String()
}

func (m FrameworkModel) renderStats() string {
	fw, ok := m.currentFramework()
	if !ok {
		return styles.DimStyle.Render("No framework selected")
	}

	var sb strings.Builder

	sb.WriteString(styles.TitleStyle.Render(fw.Name) + "\n")
	sb.WriteString(styles.DimStyle.Render(fw.Description) + "\n")

	if len(fw.Tags) > 0 {
		tags := strings.Join(fw.Tags, "  ")
		sb.WriteString("\n" + styles.DimStyle.Render(tags) + "\n")
	}

	sb.WriteString("\n")

	stats, cached := m.statsCache[fw.ID]
	if !cached {
		sb.WriteString(styles.DimStyle.Render("Loading stats…") + "\n")
		return sb.String()
	}

	if stats.Err != nil && stats.Stars == 0 && stats.WeeklyDownloads == 0 {
		sb.WriteString(styles.DimStyle.Render("Stats unavailable") + "\n")
		return sb.String()
	}

	if stats.LatestVersion != "" {
		sb.WriteString(styles.MutedStyle.Render("Version  ") +
			styles.SelectedStyle.Render(stats.LatestVersion) + "\n")
	}
	if stats.Stars > 0 {
		sb.WriteString(styles.MutedStyle.Render("Stars    ") +
			styles.SubtitleStyle.Render(formatNum(stats.Stars)) + "\n")
	}
	if stats.WeeklyDownloads > 0 {
		label := "Weekly   "
		if fw.PackagistPackage != "" {
			label = "Total    "
		}
		sb.WriteString(styles.MutedStyle.Render(label) +
			styles.MutedStyle.Render(formatNum(stats.WeeklyDownloads)) + "\n")
	}
	if fw.GithubRepo != "" {
		sb.WriteString("\n" + styles.LinkStyle.Render("github.com/"+fw.GithubRepo) + "\n")
	}

	return sb.String()
}

func formatNum(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func fuzzyFilter(frameworks []registry.Framework, query string) []registry.Framework {
	if query == "" {
		return frameworks
	}
	q := strings.ToLower(query)
	var results []registry.Framework
	for _, fw := range frameworks {
		name := strings.ToLower(fw.Name)
		if strings.Contains(name, q) {
			results = append(results, fw)
			continue
		}
		matched := false
		for _, tag := range fw.Tags {
			if strings.Contains(strings.ToLower(tag), q) {
				matched = true
				break
			}
		}
		if matched {
			results = append(results, fw)
			continue
		}
		if levenshtein(q, name) <= 2 {
			results = append(results, fw)
		}
	}
	return results
}

func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	la, lb := len(ra), len(rb)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	row := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		row[j] = j
	}

	for i := 1; i <= la; i++ {
		prevDiag := row[0]
		row[0] = i
		for j := 1; j <= lb; j++ {
			prevDiagTemp := row[j]
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			row[j] = min(row[j-1]+1, row[j]+1, prevDiag+cost)
			prevDiag = prevDiagTemp
		}
	}
	return row[lb]
}
