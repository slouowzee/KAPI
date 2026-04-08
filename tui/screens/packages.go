package screens

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slouowzee/kapi/internal/packages"
	"github.com/slouowzee/kapi/internal/registry"
	"github.com/slouowzee/kapi/tui/styles"
)

const (
	PACKAGES_DEBOUNCE = 300 * time.Millisecond
)

type defaultsLoadedMsg struct {
	results []packages.Package
}

func loadDefaultsCmd(frameworkID string, isPhp bool) tea.Cmd {
	return func() tea.Msg {
		names, ok := packages.DefaultsByFramework[frameworkID]
		if !ok || len(names) == 0 {
			return defaultsLoadedMsg{}
		}
		return defaultsLoadedMsg{results: packages.FetchDefaults(context.Background(), names, isPhp)}
	}
}

type searchResultMsg struct {
	query   string
	results []packages.Package
	err     error
}

type debounceMsg struct {
	query string
}

func searchCmd(query string, isPhp bool) tea.Cmd {
	return func() tea.Msg {
		var results []packages.Package
		var err error
		if isPhp {
			results, err = packages.SearchPackagist(context.Background(), query)
		} else {
			results, err = packages.SearchNpm(context.Background(), query)
		}
		return searchResultMsg{query: query, results: results, err: err}
	}
}

func debounceCmd(query string) tea.Cmd {
	return tea.Tick(PACKAGES_DEBOUNCE, func(time.Time) tea.Msg {
		return debounceMsg{query: query}
	})
}

type PackagesModel struct {
	width  int
	height int

	framework registry.Framework
	isPhp     bool
	targetDir string

	query           string
	queryPos        int
	results         []packages.Package
	defaults        []packages.Package
	cart            []packages.Package
	cursor          int
	searching       bool
	loadingDefaults bool
	searchErr       error
	initialPrompt   bool

	done          bool
	backPressed   bool
	backCancelled bool

	focusCart  bool
	cartCursor int

	savedCart []packages.Package
}

func NewPackages(width, height int, framework registry.Framework, targetDir string) PackagesModel {
	isPhp := framework.Ecosystem == "php"
	return PackagesModel{
		width:           width,
		height:          height,
		framework:       framework,
		isPhp:           isPhp,
		targetDir:       targetDir,
		initialPrompt:   true,
		loadingDefaults: true,
	}
}

func NewPackagesFromCart(width, height int, framework registry.Framework, targetDir string, cart []packages.Package) PackagesModel {
	isPhp := framework.Ecosystem == "php"
	saved := make([]packages.Package, len(cart))
	copy(saved, cart)
	return PackagesModel{
		width:           width,
		height:          height,
		framework:       framework,
		isPhp:           isPhp,
		targetDir:       targetDir,
		initialPrompt:   true,
		loadingDefaults: true,
		cart:            append([]packages.Package{}, cart...),
		savedCart:       saved,
	}
}

func (m *PackagesModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m PackagesModel) SelectedPackages() []packages.Package { return m.cart }
func (m PackagesModel) SavedPackages() []packages.Package    { return m.savedCart }
func (m PackagesModel) Done() bool                           { return m.done }
func (m PackagesModel) IsBack() bool                         { return m.backPressed }
func (m PackagesModel) IsBackCancelled() bool                { return m.backCancelled }

func (m *PackagesModel) ConsumeBack()          { m.backPressed = false }
func (m *PackagesModel) ConsumeBackCancelled() { m.backCancelled = false }
func (m *PackagesModel) ConsumeDone()          { m.done = false }

func (m PackagesModel) Init() tea.Cmd {
	return loadDefaultsCmd(m.framework.ID, m.isPhp)
}

func (m PackagesModel) currentPackage() (packages.Package, bool) {
	if len(m.results) == 0 || m.cursor >= len(m.results) {
		return packages.Package{}, false
	}
	return m.results[m.cursor], true
}

func (m PackagesModel) isInCart(name string) bool {
	for _, p := range m.cart {
		if p.Name == name {
			return true
		}
	}
	return false
}

func (m *PackagesModel) toggleCart(pkg packages.Package) {
	for i, p := range m.cart {
		if p.Name == pkg.Name {
			m.cart = append(m.cart[:i], m.cart[i+1:]...)
			return
		}
	}
	m.cart = append(m.cart, pkg)
}

func (m PackagesModel) Update(msg tea.Msg) (PackagesModel, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case defaultsLoadedMsg:
		m.loadingDefaults = false
		m.defaults = msg.results
		if m.initialPrompt {
			m.results = m.defaults
		}

	case debounceMsg:
		if msg.query == m.query {
			m.searching = true
			return m, searchCmd(m.query, m.isPhp)
		}

	case searchResultMsg:
		m.searching = false
		if msg.query != m.query {
			break
		}
		m.searchErr = msg.err
		m.results = msg.results
		m.cursor = 0

	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			if len(m.cart) > 0 {
				m.focusCart = !m.focusCart
			}
		case "esc":
			if m.focusCart {
				m.focusCart = false
			} else if m.query != "" {
				m.query = ""
				m.queryPos = 0
				m.results = m.defaults
				m.searching = false
				m.searchErr = nil
				m.initialPrompt = true
				m.cursor = 0
			} else if m.savedCart != nil {
				m.cart = append([]packages.Package{}, m.savedCart...)
				m.backCancelled = true
			} else {
				m.backPressed = true
			}
		case "enter":
			m.done = true
		case " ":
			if m.focusCart {
				if len(m.cart) > 0 {
					pkg := m.cart[m.cartCursor]
					m.toggleCart(pkg)
					if m.cartCursor >= len(m.cart) {
						m.cartCursor = len(m.cart) - 1
					}
					if m.cartCursor < 0 {
						m.cartCursor = 0
					}
					if len(m.cart) == 0 {
						m.focusCart = false
					}
				}
			} else {
				if pkg, ok := m.currentPackage(); ok {
					m.toggleCart(pkg)
				}
			}
		case "up", "k":
			if msg.String() == "k" && !m.focusCart && m.query != "" {
				runes := []rune(m.query)
				m.query = string(append(runes[:m.queryPos], append([]rune{'k'}, runes[m.queryPos:]...)...))
				m.queryPos++
				m.initialPrompt = false
				m.searching = true
				m.cursor = 0
				return m, debounceCmd(m.query)
			}
			if m.focusCart {
				if m.cartCursor > 0 {
					m.cartCursor--
				}
			} else {
				if m.cursor > 0 {
					m.cursor--
				}
			}
		case "down", "j":
			if msg.String() == "j" && !m.focusCart && m.query != "" {
				runes := []rune(m.query)
				m.query = string(append(runes[:m.queryPos], append([]rune{'j'}, runes[m.queryPos:]...)...))
				m.queryPos++
				m.initialPrompt = false
				m.searching = true
				m.cursor = 0
				return m, debounceCmd(m.query)
			}
			if m.focusCart {
				if m.cartCursor < len(m.cart)-1 {
					m.cartCursor++
				}
			} else {
				if m.cursor < len(m.results)-1 {
					m.cursor++
				}
			}
		case "left":
			if !m.focusCart && m.queryPos > 0 {
				m.queryPos--
			}
		case "right":
			if !m.focusCart && m.queryPos < len([]rune(m.query)) {
				m.queryPos++
			}
		case "backspace":
			m.focusCart = false
			if m.queryPos > 0 {
				runes := []rune(m.query)
				m.query = string(append(runes[:m.queryPos-1], runes[m.queryPos:]...))
				m.queryPos--
				m.initialPrompt = m.query == ""
				if m.query == "" {
					m.results = m.defaults
					m.searching = false
					m.searchErr = nil
					m.cursor = 0
				} else {
					return m, debounceCmd(m.query)
				}
			}
		case "delete":
			m.focusCart = false
			runes := []rune(m.query)
			if m.queryPos < len(runes) {
				m.query = string(append(runes[:m.queryPos], runes[m.queryPos+1:]...))
				m.initialPrompt = m.query == ""
				if m.query == "" {
					m.results = m.defaults
					m.searching = false
					m.searchErr = nil
					m.cursor = 0
				} else {
					return m, debounceCmd(m.query)
				}
			}
		default:
			if len(msg.Runes) > 0 {
				m.focusCart = false
				runes := []rune(m.query)
				m.query = string(append(runes[:m.queryPos], append(msg.Runes, runes[m.queryPos:]...)...))
				m.queryPos += len(msg.Runes)
				m.initialPrompt = false
				m.searching = true
				m.cursor = 0
				return m, debounceCmd(m.query)
			}
		}
	}

	return m, nil
}

func (m PackagesModel) View() string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(styles.TitleStyle.Render("  Additional packages") + "\n")
	sb.WriteString(styles.DimStyle.Render("  "+m.framework.Name+"  ·  "+truncatePath(m.targetDir)) + "\n")
	sb.WriteString("\n")

	listWidth, panelWidth := layoutWidths(m.width)

	boxHeight := 19
	packagesVisible := 15

	leftCol := m.renderList(packagesVisible, listWidth)
	rightCol := m.renderDetail(panelWidth)

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
	if m.focusCart {
		hints = "  [↑↓] navigate cart   [space] remove   [tab] back to search   [↵] confirm   [ctrl+c] quit"
	} else if m.query != "" {
		hints = "  [↑↓] navigate   [space] toggle   [tab] cart   [esc] clear   [↵] confirm   [ctrl+c] quit"
	} else {
		hints = "  [↑↓] navigate   [space] toggle   [tab] cart   [esc] back   [↵] confirm   [ctrl+c] quit"
	}
	sb.WriteString(styles.MutedStyle.Render(hints) + "\n")

	return sb.String()
}

func (m PackagesModel) renderList(visible, listWidth int) string {
	var sb strings.Builder

	sepWidth := listWidth - 2

	if m.query == "" {
		sb.WriteString(styles.MutedStyle.Render(" Search: ") +
			styles.DimStyle.Render("type to search…") +
			styles.TitleStyle.Render("_") + "\n")
	} else {
		sb.WriteString(styles.MutedStyle.Render(" Search: ") +
			renderTextInput(m.query, m.queryPos) + "\n")
	}

	separator := styles.DimStyle.Render(strings.Repeat("─", sepWidth))
	sb.WriteString(separator + "\n")

	if m.loadingDefaults && m.initialPrompt {
		sb.WriteString(styles.DimStyle.Render(" Loading popular packages…") + "\n")
		return sb.String()
	}

	if m.initialPrompt && len(m.results) == 0 {
		sb.WriteString(styles.DimStyle.Render(" Start typing to search…") + "\n")
		return sb.String()
	}

	if m.searching {
		sb.WriteString(styles.DimStyle.Render(" Searching…") + "\n")
		return sb.String()
	}

	if m.searchErr != nil {
		sb.WriteString(styles.ErrorStyle.Render(" Search failed") + "\n")
		return sb.String()
	}

	if len(m.results) == 0 {
		sb.WriteString(styles.DimStyle.Render(" No results") + "\n")
		return sb.String()
	}

	total := len(m.results)
	windowStart, windowEnd := scrollWindow(m.cursor, total, visible)

	if windowStart > 0 {
		sb.WriteString(styles.DimStyle.Render(fmt.Sprintf("    ↑ %d more", windowStart)) + "\n")
	}

	for i, pkg := range m.results[windowStart:windowEnd] {
		absIdx := windowStart + i
		inCart := m.isInCart(pkg.Name)

		var checkbox string
		if inCart {
			checkbox = styles.SelectedStyle.Render("[x]")
		} else {
			checkbox = styles.DimStyle.Render("[ ]")
		}

		if absIdx == m.cursor {
			var cur string
			if m.focusCart {
				cur = styles.DimStyle.Render(" ❯❯")
			} else {
				cur = styles.CursorStyle.Render(" ❯❯")
			}
			label := styles.SelectedStyle.Render(" " + pkg.Name)
			sb.WriteString(fmt.Sprintf("%s %s%s\n", cur, checkbox, label))
		} else {
			sb.WriteString(fmt.Sprintf("     %s %s\n", checkbox, styles.DimStyle.Render(pkg.Name)))
		}
	}

	if windowEnd < total {
		sb.WriteString(styles.DimStyle.Render(fmt.Sprintf("    ↓ %d more", total-windowEnd)) + "\n")
	}

	return sb.String()
}

func (m PackagesModel) renderDetail(panelWidth int) string {
	var sb strings.Builder

	detailLines := 0
	pkg, ok := m.currentPackage()
	if ok {
		sb.WriteString(styles.TitleStyle.Render(pkg.Name) + "\n")
		detailLines++
		sb.WriteString(styles.DimStyle.Render(pkg.Description) + "\n")
		detailLines++
		sb.WriteString("\n")
		detailLines++
		if pkg.Version != "" {
			sb.WriteString(styles.MutedStyle.Render("Version  ") + styles.SelectedStyle.Render(pkg.Version) + "\n")
			detailLines++
		}
		if pkg.Stars > 0 {
			sb.WriteString(styles.MutedStyle.Render("Stars    ") + styles.SubtitleStyle.Render(formatNum(pkg.Stars)) + "\n")
			detailLines++
		}
		if pkg.Weekly > 0 {
			label := "Weekly   "
			if m.isPhp {
				label = "Total    "
			}
			sb.WriteString(styles.MutedStyle.Render(label) + styles.MutedStyle.Render(formatNum(pkg.Weekly)) + "\n")
			detailLines++
		}
		if pkg.GithubRepo != "" {
			sb.WriteString("\n" + styles.LinkStyle.Render("github.com/"+pkg.GithubRepo) + "\n")
			detailLines += 2
		}
	} else {
		sb.WriteString(styles.DimStyle.Render("No package selected") + "\n")
		detailLines++
	}

	sb.WriteString("\n")
	separator := styles.DimStyle.Render(strings.Repeat("─", panelWidth-4))
	sb.WriteString(separator + "\n")

	cartLabel := fmt.Sprintf("Cart (%d)", len(m.cart))
	if m.focusCart {
		cartLabel += styles.DimStyle.Render(" (focused)")
	}
	sb.WriteString(styles.MutedStyle.Render(cartLabel) + "\n")

	if len(m.cart) == 0 {
		sb.WriteString(styles.DimStyle.Render("  empty") + "\n")
	} else {
		const visibleCart = 4
		total := len(m.cart)

		windowStart, windowEnd := scrollWindow(m.cartCursor, total, visibleCart)

		if windowStart > 0 {
			sb.WriteString(styles.DimStyle.Render(fmt.Sprintf("  ↑ %d more", windowStart)) + "\n")
		}
		for i, p := range m.cart[windowStart:windowEnd] {
			absIdx := windowStart + i
			if m.focusCart && absIdx == m.cartCursor {
				cur := styles.CursorStyle.Render(" ❯")
				sb.WriteString(fmt.Sprintf("%s %s\n", cur, styles.SelectedStyle.Render(p.Name)))
			} else {
				sb.WriteString(styles.SelectedStyle.Render("  · ") + styles.DimStyle.Render(p.Name) + "\n")
			}
		}
		if windowEnd < total {
			sb.WriteString(styles.DimStyle.Render(fmt.Sprintf("  ↓ %d more", total-windowEnd)) + "\n")
		}
	}

	return sb.String()
}
