package screens

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slouowzee/kapi/internal/config"
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
		token := os.Getenv("GITHUB_TOKEN")
		if token == "" {
			if cfg, err := config.Load(); err == nil {
				token = cfg.GithubToken
			}
		}
		stats := trends.Fetch(fw.NpmPackage, fw.PackagistPackage, fw.GithubRepo, token)
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

	statsCache   map[string]trends.Stats
	statsLoading bool

	selected registry.Framework
	done     bool

	// backPressed is a one-shot flag set when the user presses esc.
	backPressed bool
}

func NewFramework(width, height int, ecosystemIdx int, targetDir string) FrameworkModel {
	eco := "php"
	if ecosystemIdx == ECOSYSTEM_JS {
		eco = "js"
	}
	return FrameworkModel{
		width:      width,
		height:     height,
		ecosystem:  eco,
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
			return m, loadTrendsCmd(m.all[0])
		}

	case trendsLoadedMsg:
		m.statsLoading = false
		m.statsCache[msg.frameworkID] = msg.stats

	case tea.KeyMsg:
		if m.loading {
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
				return m, m.maybeLoadStats()
			}
		case "down", "j":
			if m.cursor < len(m.visible)-1 {
				m.cursor++
				return m, m.maybeLoadStats()
			}
		case "backspace":
			if len(m.query) > 0 {
				runes := []rune(m.query)
				m.query = string(runes[:len(runes)-1])
				m.visible = fuzzyFilter(m.all, m.query)
				m.cursor = 0
				return m, m.maybeLoadStats()
			}
		default:
			if len(msg.Runes) > 0 {
				m.query += string(msg.Runes)
				m.visible = fuzzyFilter(m.all, m.query)
				m.cursor = 0
				return m, m.maybeLoadStats()
			}
		}
	}

	return m, nil
}

func (m *FrameworkModel) maybeLoadStats() tea.Cmd {
	fw, ok := m.currentFramework()
	if !ok {
		return nil
	}
	if _, cached := m.statsCache[fw.ID]; cached {
		return nil
	}
	m.statsLoading = true
	return loadTrendsCmd(fw)
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
		sb.WriteString(styles.MutedStyle.Render("  [esc] back   [q] quit") + "\n")
		return sb.String()
	}

	if m.loadErr != nil {
		sb.WriteString(styles.ErrorStyle.Render("  ✗ Failed to load frameworks: "+m.loadErr.Error()) + "\n")
		sb.WriteString("\n")
		sb.WriteString(styles.MutedStyle.Render("  [esc] back   [q] quit") + "\n")
		return sb.String()
	}

	leftCol := m.renderList()
	rightCol := m.renderStats()

	margin := 6
	availWidth := m.width - margin
	listWidth := (availWidth * 6) / 10
	if listWidth < 30 {
		listWidth = 30
	}
	panelWidth := availWidth - listWidth - 3
	if panelWidth < 20 {
		panelWidth = 20
	}

	boxHeight := 9

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

func (m FrameworkModel) renderList() string {
	var sb strings.Builder

	margin := 6
	availWidth := m.width - margin
	listWidth := (availWidth * 6) / 10
	if listWidth < 30 {
		listWidth = 30
	}
	sepWidth := listWidth

	// Search input line
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

	const VISIBLE = 7
	total := len(m.visible)

	windowStart := m.cursor - VISIBLE/2
	if windowStart < 0 {
		windowStart = 0
	}
	if windowStart+VISIBLE > total {
		windowStart = total - VISIBLE
	}
	if windowStart < 0 {
		windowStart = 0
	}
	windowEnd := windowStart + VISIBLE
	if windowEnd > total {
		windowEnd = total
	}

	if windowStart > 0 {
		sb.WriteString(styles.DimStyle.Render(fmt.Sprintf("    ↑ %d more", windowStart)) + "\n")
	}

	for i, fw := range m.visible[windowStart:windowEnd] {
		absIdx := windowStart + i
		if absIdx == m.cursor {
			cur := lipgloss.NewStyle().Foreground(styles.COLOR_PRIMARY).Bold(true).Render(" ❯❯")
			label := styles.SelectedStyle.Render(" " + fw.Name)
			sb.WriteString(fmt.Sprintf("%s%s\n", cur, label))
		} else {
			sb.WriteString(fmt.Sprintf("    %s\n", styles.DimStyle.Render(fw.Name)))
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

// fuzzyFilter returns frameworks matching the query against name and tags only.
// It first tries a case-insensitive substring match on the name, then falls back
// to a Levenshtein distance check to tolerate typos (e.g. "laravl" → Laravel).
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

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			curr[j] = min3(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
