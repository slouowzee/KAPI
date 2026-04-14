package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slouowzee/kapi/tui/styles"
)

type UpdateInfoModel struct {
	width         int
	height        int
	latestVersion string
	done          bool
}

func NewUpdateInfo(width, height int, latestVersion string) UpdateInfoModel {
	return UpdateInfoModel{
		width:         width,
		height:        height,
		latestVersion: latestVersion,
	}
}

func (m *UpdateInfoModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m UpdateInfoModel) Init() tea.Cmd {
	return nil
}

func (m UpdateInfoModel) Update(msg tea.Msg) (UpdateInfoModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", "esc", "q":
			m.done = true
		}
	}
	return m, nil
}

func (m UpdateInfoModel) View() string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(styles.TitleStyle.Render("  Update KAPI") + "\n\n")

	sb.WriteString(styles.SuccessStyle.Render(fmt.Sprintf("  Version %s is available!", m.latestVersion)) + "\n\n")

	sb.WriteString(styles.DimStyle.Render("  To update KAPI, please run the command corresponding") + "\n")
	sb.WriteString(styles.DimStyle.Render("  to your installation method:") + "\n\n")

	sb.WriteString(styles.SelectedStyle.Render("  Homebrew (macOS / Linux):") + "\n")
	sb.WriteString("    brew upgrade kapi\n\n")

	sb.WriteString(styles.SelectedStyle.Render("  Scoop (Windows):") + "\n")
	sb.WriteString("    scoop update kapi\n\n")

	sb.WriteString(styles.SelectedStyle.Render("  Arch Linux (AUR):") + "\n")
	sb.WriteString("    yay -Syu kapi-bin\n\n")

	sb.WriteString(styles.SelectedStyle.Render("  Universal Bash Script:") + "\n")
	sb.WriteString("    curl -fsSL https://raw.githubusercontent.com/slouowzee/kapi/main/install.sh | bash\n\n")

	sb.WriteString(styles.SelectedStyle.Render("  Go Install:") + "\n")
	sb.WriteString(fmt.Sprintf("    go install github.com/slouowzee/kapi@%s\n\n", m.latestVersion))

	sb.WriteString(styles.MutedStyle.Render("  [↵ / esc] back to menu") + "\n")

	return sb.String()
}

func (m UpdateInfoModel) IsDone() bool {
	return m.done
}

func (m *UpdateInfoModel) ConsumeDone() {
	m.done = false
}
