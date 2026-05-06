package screens

import (
	"context"
	"fmt"
	"strings"
	"github.com/slouowzee/kapi/internal/config"
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

type favoritesLoadedMsg struct {
	favorites []packages.Package
}

func loadFavoritesCmd(frameworkID string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return favoritesLoadedMsg{}
		}
		var favs []packages.Package
		for _, f := range cfg.Favorites[frameworkID] {
			favs = append(favs, packages.Package{
				Name:        f.Name,
				Description: f.Description,
			})
		}
		return favoritesLoadedMsg{favorites: favs}
	}
}

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
	favorites       []packages.Package
	cursor          int
	searching       bool
	loadingDefaults bool
	searchErr       error
	initialPrompt   bool

	inCartMode      bool
	inFavoritesMode bool
	cartCursor      int
	favoritesCursor int

	done          bool
	backPressed   bool
	backCancelled bool

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
		favorites:       []packages.Package{},
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
		favorites:       []packages.Package{},
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


func (m PackagesModel) getFilteredFavorites() []packages.Package {
	if m.query == "" {
		return m.favorites
	}
	var filtered []packages.Package
	q := strings.ToLower(m.query)
	for _, p := range m.favorites {
		if strings.Contains(strings.ToLower(p.Name), q) || strings.Contains(strings.ToLower(p.Description), q) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func (m PackagesModel) Init() tea.Cmd {
	return tea.Batch(loadDefaultsCmd(m.framework.ID, m.isPhp), loadFavoritesCmd(m.framework.ID))
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

func (m PackagesModel) isFavorite(name string) bool {
	for _, p := range m.favorites {
		if p.Name == name {
			return true
		}
	}
	return false
}

func (m *PackagesModel) toggleFavorite(pkg packages.Package) tea.Cmd {
	isFav := false
	for i, p := range m.favorites {
		if p.Name == pkg.Name {
			m.favorites = append(m.favorites[:i], m.favorites[i+1:]...)
			isFav = true
			break
		}
	}
	if !isFav {
		m.favorites = append(m.favorites, pkg)
	}

	fwID := m.framework.ID
	favsToSave := m.favorites

	return func() tea.Msg {
		cfg, _ := config.Load()
		if cfg.Favorites == nil {
			cfg.Favorites = make(map[string][]config.FavoritePackage)
		}
		var newFavs []config.FavoritePackage
		for _, f := range favsToSave {
			newFavs = append(newFavs, config.FavoritePackage{
				Name:        f.Name,
				Description: f.Description,
			})
		}
		cfg.Favorites[fwID] = newFavs
		_ = config.Save(cfg)
		return nil
	}
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

	case favoritesLoadedMsg:
		m.favorites = msg.favorites

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
				m.inCartMode = !m.inCartMode
				if m.inCartMode {
					m.cartCursor = 0
				}
			}
		case "ctrl+f":
			m.inFavoritesMode = !m.inFavoritesMode
			m.inCartMode = false
			m.favoritesCursor = 0
			m.query = ""
			m.queryPos = 0
			m.initialPrompt = m.query == ""
			m.searching = false

		case "esc":
			if m.inCartMode || m.inFavoritesMode {
				m.inCartMode = false
				m.inFavoritesMode = false
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
			if m.inCartMode {
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
						m.inCartMode = false
						m.inFavoritesMode = true
					}
				}
			} else if m.inFavoritesMode {
				favs := m.getFilteredFavorites()
				if len(favs) > 0 && m.favoritesCursor < len(favs) {
					pkg := favs[m.favoritesCursor]
					m.toggleCart(pkg)
				}
			} else {
				if pkg, ok := m.currentPackage(); ok {
					m.toggleCart(pkg)
				}
			}
		case "f", "F":
			if m.inFavoritesMode {
				favs := m.getFilteredFavorites()
				if len(favs) > 0 && m.favoritesCursor < len(favs) {
					pkg := favs[m.favoritesCursor]
					cmd := m.toggleFavorite(pkg)
					favs = m.getFilteredFavorites()
					if m.favoritesCursor >= len(favs) {
						m.favoritesCursor = len(favs) - 1
					}
					if m.favoritesCursor < 0 {
						m.favoritesCursor = 0
					}
					return m, cmd
				}
			} else if m.inCartMode {
				if len(m.cart) > 0 {
					pkg := m.cart[m.cartCursor]
					return m, m.toggleFavorite(pkg)
				}
			} else {
				if !m.initialPrompt && m.query != "" && (msg.String() == "f" || msg.String() == "F") {
					runes := []rune(m.query)
					m.query = string(append(runes[:m.queryPos], append([]rune{rune(msg.String()[0])}, runes[m.queryPos:]...)...))
					m.queryPos++
					m.initialPrompt = false
					m.searching = true
					m.cursor = 0
					return m, debounceCmd(m.query)
				}
				if pkg, ok := m.currentPackage(); ok {
					return m, m.toggleFavorite(pkg)
				}
			}
		case "up", "k":
			if msg.String() == "k" && !m.inCartMode && !m.inFavoritesMode {
				runes := []rune(m.query)
				m.query = string(append(runes[:m.queryPos], append([]rune{'k'}, runes[m.queryPos:]...)...))
				m.queryPos++
				m.initialPrompt = false
				m.searching = true
				m.cursor = 0
				return m, debounceCmd(m.query)
			}
			if m.inCartMode {
				if m.cartCursor > 0 {
					m.cartCursor--
				}
			} else if m.inFavoritesMode {
				if m.favoritesCursor > 0 {
					m.favoritesCursor--
				}
			} else {
				if m.cursor > 0 {
					m.cursor--
				}
			}
		case "down", "j":
			if msg.String() == "j" && !m.inCartMode && !m.inFavoritesMode {
				runes := []rune(m.query)
				m.query = string(append(runes[:m.queryPos], append([]rune{'j'}, runes[m.queryPos:]...)...))
				m.queryPos++
				m.initialPrompt = false
				m.searching = true
				m.cursor = 0
				return m, debounceCmd(m.query)
			}
			if m.inCartMode {
				if m.cartCursor < len(m.cart)-1 {
					m.cartCursor++
				}
			} else if m.inFavoritesMode {
				favs := m.getFilteredFavorites()
				if m.favoritesCursor < len(favs)-1 {
					m.favoritesCursor++
				}
			} else {
				if m.cursor < len(m.results)-1 {
					m.cursor++
				}
			}
		case "left":
			if !m.inCartMode && m.queryPos > 0 {
				m.queryPos--
			}
		case "right":
			if !m.inCartMode {
				runes := []rune(m.query)
				if m.queryPos < len(runes) {
					m.queryPos++
				}
			}
		case "backspace":
			if m.inCartMode {
				break
			}
			runes := []rune(m.query)
			if m.queryPos > 0 {
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
			m.inCartMode = false
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
				m.inCartMode = false
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
	if m.inCartMode {
		hints = "  [↑↓] navigate cart   [space] remove   [f] favorite   [tab] focus search   [ctrl+f] favorites   [↵] confirm   [ctrl+c] quit"
	} else if m.inFavoritesMode {
		hints = "  [↑↓] navigate   [space] toggle   [f] remove   [tab] focus cart   [ctrl+f] close   [↵] confirm   [ctrl+c] quit"
	} else if m.query != "" {
		hints = "  [↑↓] navigate   [space] toggle   [f] favorite   [tab] focus cart   [ctrl+f] favorites   [esc] clear   [↵] confirm   [ctrl+c] quit"
	} else {
		hints = "  [↑↓] navigate   [space] toggle   [f] favorite   [tab] focus cart   [ctrl+f] favorites   [esc] back   [↵] confirm   [ctrl+c] quit"
	}
	sb.WriteString(styles.MutedStyle.Render(hints) + "\n")

	return sb.String()
}

func (m PackagesModel) renderList(visible, listWidth int) string {
	var sb strings.Builder

	sepWidth := listWidth - 2

	if m.inFavoritesMode {
		sb.WriteString(styles.SelectedStyle.Render(fmt.Sprintf(" Favorites (%d)", len(m.favorites))) + "\n")
		if m.query == "" {
			sb.WriteString(styles.MutedStyle.Render(" Search favs: ") + styles.DimStyle.Render("type to filter…") + styles.TitleStyle.Render("_") + "\n")
		} else {
			sb.WriteString(styles.MutedStyle.Render(" Search favs: ") + renderTextInput(m.query, m.queryPos) + "\n")
		}
		separator := styles.DimStyle.Render(strings.Repeat("─", sepWidth))
		sb.WriteString(separator + "\n")

		favs := m.getFilteredFavorites()
		if len(favs) == 0 {
			sb.WriteString(styles.DimStyle.Render(" No matching favorites.") + "\n")
			return sb.String()
		}

		total := len(favs)
		windowStart, windowEnd := scrollWindow(m.favoritesCursor, total, visible)

		if windowStart > 0 {
			sb.WriteString(styles.DimStyle.Render(fmt.Sprintf("    ↑ %d more", windowStart)) + "\n")
		}

		if windowEnd > total {
			windowEnd = total
		}
		
		for i, pkg := range favs[windowStart:windowEnd] {
			absIdx := windowStart + i
			inCart := m.isInCart(pkg.Name)

			var checkbox string
			if inCart {
				checkbox = styles.SelectedStyle.Render("[x]")
			} else {
				checkbox = styles.DimStyle.Render("[ ]")
			}
			
			favIcon := styles.SelectedStyle.Render(" ★")

			if absIdx == m.favoritesCursor {
				cur := styles.CursorStyle.Render(" ❯❯")
				label := styles.SelectedStyle.Render(pkg.Name)
				line := fmt.Sprintf("%s %s %s%s", cur, checkbox, label, favIcon)
				sb.WriteString(lipgloss.NewStyle().Width(listWidth).Render(line) + "\n")
			} else {
				label := styles.DimStyle.Render(pkg.Name)
				line := fmt.Sprintf("    %s %s%s", checkbox, label, favIcon)
				sb.WriteString(lipgloss.NewStyle().Width(listWidth).Render(line) + "\n")
			}
		}

		if windowEnd < total {
			sb.WriteString(styles.DimStyle.Render(fmt.Sprintf("    ↓ %d more", total-windowEnd)) + "\n")
		}

		return sb.String()
	}

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
		
		isFav := m.isFavorite(pkg.Name)
		var favIcon string
		if isFav {
			favIcon = styles.SelectedStyle.Render(" ★")
		} else {
			favIcon = "  "
		}

		if absIdx == m.cursor {
			var cur string
			if m.inCartMode || m.inFavoritesMode {
				cur = styles.DimStyle.Render(" ❯❯")
			} else {
				cur = styles.CursorStyle.Render(" ❯❯")
			}
			label := styles.SelectedStyle.Render(pkg.Name)
			line := fmt.Sprintf("%s %s %s%s", cur, checkbox, label, favIcon)
			sb.WriteString(lipgloss.NewStyle().Width(listWidth).Render(line) + "\n")
		} else {
			label := styles.DimStyle.Render(pkg.Name)
			line := fmt.Sprintf("    %s %s%s", checkbox, label, favIcon)
			sb.WriteString(lipgloss.NewStyle().Width(listWidth).Render(line) + "\n")
		}
	}

	if windowEnd < total {
		sb.WriteString(styles.DimStyle.Render(fmt.Sprintf("    ↓ %d more", total-windowEnd)) + "\n")
	}

	return sb.String()
}

func (m PackagesModel) renderDetail(panelWidth int) string {
	var sb strings.Builder

	pkg, ok := m.currentPackage()
	if ok {
		sb.WriteString(styles.TitleStyle.Render(pkg.Name) + "\n")
		sb.WriteString(styles.DimStyle.Render(pkg.Description) + "\n")
		sb.WriteString("\n")
		if pkg.Version != "" {
			sb.WriteString(styles.MutedStyle.Render("Version  ") + styles.SelectedStyle.Render(pkg.Version) + "\n")
		}
		if pkg.Stars > 0 {
			sb.WriteString(styles.MutedStyle.Render("Stars    ") + styles.SubtitleStyle.Render(formatNum(pkg.Stars)) + "\n")
		}
		if pkg.Weekly > 0 {
			label := "Weekly   "
			if m.isPhp {
				label = "Total    "
			}
			sb.WriteString(styles.MutedStyle.Render(label) + styles.MutedStyle.Render(formatNum(pkg.Weekly)) + "\n")
		}
		if pkg.GithubRepo != "" {
			sb.WriteString("\n" + styles.LinkStyle.Render("github.com/"+pkg.GithubRepo) + "\n")
		}
	} else {
		sb.WriteString(styles.DimStyle.Render("No package selected") + "\n")
	}

	sb.WriteString("\n")
	separator := styles.DimStyle.Render(strings.Repeat("─", panelWidth-4))
	sb.WriteString(separator + "\n")

	cartLabel := fmt.Sprintf("Cart (%d)", len(m.cart))
	if m.inCartMode {
		cartLabel = styles.SelectedStyle.Render(cartLabel)
	} else {
		cartLabel = styles.MutedStyle.Render(cartLabel)
	}
	
	sb.WriteString(cartLabel + "\n")

	if len(m.cart) == 0 {
		sb.WriteString(styles.DimStyle.Render("  empty") + "\n")
	} else {
		const visibleCart = 6
		total := len(m.cart)

		windowStart, windowEnd := scrollWindow(m.cartCursor, total, visibleCart)

		if windowStart > 0 {
			sb.WriteString(styles.DimStyle.Render(fmt.Sprintf("  ↑ %d more", windowStart)) + "\n")
		}
		for i, p := range m.cart[windowStart:windowEnd] {
			absIdx := windowStart + i
			if m.inCartMode && absIdx == m.cartCursor {
				cur := styles.CursorStyle.Render("❯")
				fmt.Fprintf(&sb, "%s %s\n", cur, styles.SelectedStyle.Render("· " + p.Name))
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
