package tui

import "github.com/charmbracelet/lipgloss"

var (
	ColorHealthy     = lipgloss.Color("#00ff00")
	ColorError       = lipgloss.Color("#ff0000")
	ColorProgressing = lipgloss.Color("#ffaa00")
	ColorUnknown     = lipgloss.Color("#888888")
	ColorAccent      = lipgloss.Color("#00aaff")
	ColorMuted       = lipgloss.Color("#666666")

	IconHealthy     = "✔"
	IconError       = "✘"
	IconProgressing = "⟳"
	IconUnknown     = "?"

	StyleHealthy     = lipgloss.NewStyle().Foreground(ColorHealthy)
	StyleError       = lipgloss.NewStyle().Foreground(ColorError)
	StyleProgressing = lipgloss.NewStyle().Foreground(ColorProgressing)
	StyleUnknown     = lipgloss.NewStyle().Foreground(ColorUnknown)
	StyleAccent      = lipgloss.NewStyle().Foreground(ColorAccent)
	StyleMuted       = lipgloss.NewStyle().Foreground(ColorMuted)

	StyleSelected = lipgloss.NewStyle().
			Bold(true).
			Background(lipgloss.Color("#333333"))

	StyleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorAccent).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(ColorMuted)

	StyleStatusBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#333333")).
			Padding(0, 1)

	StyleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorAccent)

	StyleLabel = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#aaaaaa"))

	StyleValue = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff"))
)
