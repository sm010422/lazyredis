package ui

import "github.com/charmbracelet/lipgloss"

var (
	colorPrimary   = lipgloss.Color("#a6e3a1") // green
	colorSecondary = lipgloss.Color("#89b4fa") // blue
	colorAccent    = lipgloss.Color("#f38ba8") // red
	colorWarning   = lipgloss.Color("#f9e2af") // yellow
	colorMuted     = lipgloss.Color("#585b70") // surface2
	colorText      = lipgloss.Color("#cdd6f4") // text
	colorSubtext   = lipgloss.Color("#a6adc8") // subtext1
	colorBg        = lipgloss.Color("#1e1e2e") // base
	colorBorder    = lipgloss.Color("#45475a") // surface1

	styleTitle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	stylePanelBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBorder)

	stylePanelBorderActive = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorSecondary)

	stylePanelTitle = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Bold(true).
			PaddingLeft(1)

	styleSelected = lipgloss.NewStyle().
			Foreground(colorBg).
			Background(colorSecondary).
			Bold(true)

	styleKeyType = map[string]lipgloss.Style{
		"string": lipgloss.NewStyle().Foreground(colorPrimary),
		"list":   lipgloss.NewStyle().Foreground(colorSecondary),
		"set":    lipgloss.NewStyle().Foreground(colorWarning),
		"zset":   lipgloss.NewStyle().Foreground(lipgloss.Color("#cba6f7")),
		"hash":   lipgloss.NewStyle().Foreground(lipgloss.Color("#fab387")),
		"stream": lipgloss.NewStyle().Foreground(lipgloss.Color("#89dceb")),
		"none":   lipgloss.NewStyle().Foreground(colorMuted),
	}

	styleStatusBar = lipgloss.NewStyle().
			Foreground(colorText).
			Background(colorMuted).
			PaddingLeft(1).
			PaddingRight(1)

	styleStatusKey = lipgloss.NewStyle().
			Foreground(colorBg).
			Background(colorSecondary).
			Bold(true).
			PaddingLeft(1).
			PaddingRight(1)

	styleStatusVal = lipgloss.NewStyle().
			Foreground(colorText).
			Background(colorMuted).
			PaddingLeft(1).
			PaddingRight(1)

	styleError = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	styleInfo = lipgloss.NewStyle().
			Foreground(colorSubtext)

	styleTTL = lipgloss.NewStyle().
			Foreground(colorWarning)

	styleFilterPrompt = lipgloss.NewStyle().
				Foreground(colorPrimary).
				Bold(true)
)

func keyTypeStyle(t string) lipgloss.Style {
	if s, ok := styleKeyType[t]; ok {
		return s
	}
	return lipgloss.NewStyle().Foreground(colorMuted)
}

func keyTypeIcon(t string) string {
	switch t {
	case "string":
		return "STR"
	case "list":
		return "LST"
	case "set":
		return "SET"
	case "zset":
		return "ZST"
	case "hash":
		return "HSH"
	case "stream":
		return "STM"
	}
	return "???"
}
