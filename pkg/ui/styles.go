package ui

import "github.com/charmbracelet/lipgloss"

var (
	colorGreen   = lipgloss.Color("#a6e3a1")
	colorBlue    = lipgloss.Color("#89b4fa")
	colorRed     = lipgloss.Color("#f38ba8")
	colorYellow  = lipgloss.Color("#f9e2af")
	colorPurple  = lipgloss.Color("#cba6f7")
	colorPeach   = lipgloss.Color("#fab387")
	colorTeal    = lipgloss.Color("#89dceb")
	colorMuted   = lipgloss.Color("#585b70")
	colorText    = lipgloss.Color("#cdd6f4")
	colorSubtext = lipgloss.Color("#a6adc8")
	colorBorder  = lipgloss.Color("#45475a")
	colorBorderActive = lipgloss.Color("#89b4fa")
	colorBg2     = lipgloss.Color("#181825")
	colorSurface = lipgloss.Color("#313244")

	styleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder)

	styleBorderActive = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBorderActive)

	stylePanelTitle = lipgloss.NewStyle().
			Foreground(colorBlue).
			Bold(true)

	styleSelected = lipgloss.NewStyle().
			Foreground(colorBg2).
			Background(colorBlue).
			Bold(true)

	styleSelectedAlt = lipgloss.NewStyle().
				Foreground(colorBg2).
				Background(colorPurple).
				Bold(true)

	styleKeyTypes = map[string]lipgloss.Style{
		"string": lipgloss.NewStyle().Foreground(colorGreen),
		"list":   lipgloss.NewStyle().Foreground(colorBlue),
		"set":    lipgloss.NewStyle().Foreground(colorYellow),
		"zset":   lipgloss.NewStyle().Foreground(colorPurple),
		"hash":   lipgloss.NewStyle().Foreground(colorPeach),
		"stream": lipgloss.NewStyle().Foreground(colorTeal),
		"none":   lipgloss.NewStyle().Foreground(colorMuted),
	}

	styleBadgeTypes = map[string]lipgloss.Style{
		"string": lipgloss.NewStyle().Foreground(colorBg2).Background(colorGreen).Bold(true).PaddingLeft(1).PaddingRight(1),
		"list":   lipgloss.NewStyle().Foreground(colorBg2).Background(colorBlue).Bold(true).PaddingLeft(1).PaddingRight(1),
		"set":    lipgloss.NewStyle().Foreground(colorBg2).Background(colorYellow).Bold(true).PaddingLeft(1).PaddingRight(1),
		"zset":   lipgloss.NewStyle().Foreground(colorBg2).Background(colorPurple).Bold(true).PaddingLeft(1).PaddingRight(1),
		"hash":   lipgloss.NewStyle().Foreground(colorBg2).Background(colorPeach).Bold(true).PaddingLeft(1).PaddingRight(1),
		"stream": lipgloss.NewStyle().Foreground(colorBg2).Background(colorTeal).Bold(true).PaddingLeft(1).PaddingRight(1),
	}

	styleError   = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	styleSuccess = lipgloss.NewStyle().Foreground(colorGreen)
	styleWarning = lipgloss.NewStyle().Foreground(colorYellow)
	styleInfo    = lipgloss.NewStyle().Foreground(colorSubtext)
	styleMuted   = lipgloss.NewStyle().Foreground(colorMuted)
	styleBold    = lipgloss.NewStyle().Foreground(colorText).Bold(true)
	styleTTL     = lipgloss.NewStyle().Foreground(colorYellow)
	styleTitle   = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)

	styleTabActive = lipgloss.NewStyle().
			Foreground(colorBg2).
			Background(colorBlue).
			Bold(true).
			PaddingLeft(2).
			PaddingRight(2)

	styleTabInactive = lipgloss.NewStyle().
				Foreground(colorSubtext).
				Background(colorSurface).
				PaddingLeft(2).
				PaddingRight(2)

	styleStatusNormal = lipgloss.NewStyle().
				Foreground(colorText).
				Background(colorSurface)

	styleStatusError = lipgloss.NewStyle().
				Foreground(colorRed).
				Background(colorSurface).
				Bold(true)

	styleStatusSuccess = lipgloss.NewStyle().
				Foreground(colorGreen).
				Background(colorSurface)

	styleHintKey = lipgloss.NewStyle().
			Foreground(colorBg2).
			Background(colorMuted).
			Bold(true).
			PaddingLeft(1).
			PaddingRight(1)

	styleHintDesc = lipgloss.NewStyle().
			Foreground(colorSubtext).
			Background(colorSurface).
			PaddingRight(1)

	styleModalBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorPurple).
				Padding(1, 2)
)

func keyTypeStyle(t string) lipgloss.Style {
	if s, ok := styleKeyTypes[t]; ok {
		return s
	}
	return styleMuted
}

func keyTypeBadge(t string) string {
	abbr := map[string]string{
		"string": "STR",
		"list":   "LST",
		"set":    "SET",
		"zset":   "ZST",
		"hash":   "HSH",
		"stream": "STM",
	}
	a, ok := abbr[t]
	if !ok {
		a = "???"
	}
	if s, ok := styleBadgeTypes[t]; ok {
		return s.Render(a)
	}
	return styleMuted.Render("[" + a + "]")
}
