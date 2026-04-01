package styles

import "github.com/charmbracelet/lipgloss"

var (
	COLOR_PRIMARY   = lipgloss.Color("#8B6542")
	COLOR_SUCCESS   = lipgloss.Color("#7C9A6B")
	COLOR_MUTED     = lipgloss.Color("#E8DCC8")
	COLOR_DIM       = lipgloss.Color("#6B6B6B")
	COLOR_ERROR     = lipgloss.Color("#C0392B")
	COLOR_HIGHLIGHT = lipgloss.Color("#A0785A")
	COLOR_LINK      = lipgloss.Color("#5B9BD5")
)

var (
	TitleStyle = lipgloss.NewStyle().
			Foreground(COLOR_PRIMARY).
			Bold(true)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(COLOR_HIGHLIGHT)

	MutedStyle = lipgloss.NewStyle().
			Foreground(COLOR_MUTED)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(COLOR_SUCCESS)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(COLOR_ERROR)

	SelectedStyle = lipgloss.NewStyle().
			Foreground(COLOR_SUCCESS).
			Bold(true)

	DimStyle = lipgloss.NewStyle().
			Foreground(COLOR_DIM)

	LinkStyle = lipgloss.NewStyle().
			Foreground(COLOR_LINK).
			Underline(true)

	CursorStyle = lipgloss.NewStyle().
			Foreground(COLOR_PRIMARY).
			Bold(true)
)
