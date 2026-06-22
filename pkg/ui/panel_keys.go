package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

type KeysPanel struct {
	keys         []string // all loaded keys (sorted)
	filtered     []string // after pattern/fuzzy filter
	cursor       int
	scrollOffset int
	filterInput  textinput.Model
	filtering    bool
	filterVal    string
}

func newKeysPanel() KeysPanel {
	fi := textinput.New()
	fi.Placeholder = "filter pattern  (wildcards: * ?)"
	fi.CharLimit = 256
	fi.PromptStyle = styleFilterPrompt
	fi.Prompt = "/"
	return KeysPanel{filterInput: fi}
}

var styleFilterPrompt = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)

func (p *KeysPanel) SetKeys(keys []string) {
	p.keys = keys
	p.applyFilter()
}

func (p *KeysPanel) applyFilter() {
	pattern := strings.ToLower(p.filterVal)
	if pattern == "" || pattern == "*" {
		p.filtered = make([]string, len(p.keys))
		copy(p.filtered, p.keys)
		return
	}
	var out []string
	for _, k := range p.keys {
		if strings.Contains(strings.ToLower(k), pattern) {
			out = append(out, k)
		}
	}
	p.filtered = out
}

func (p *KeysPanel) Selected() string {
	if len(p.filtered) == 0 || p.cursor >= len(p.filtered) {
		return ""
	}
	return p.filtered[p.cursor]
}

func (p *KeysPanel) MoveDown() bool {
	if p.cursor < len(p.filtered)-1 {
		p.cursor++
		return true
	}
	return false
}

func (p *KeysPanel) MoveUp() bool {
	if p.cursor > 0 {
		p.cursor--
		return true
	}
	return false
}

func (p *KeysPanel) MoveTop() {
	p.cursor = 0
	p.scrollOffset = 0
}

func (p *KeysPanel) MoveBottom() {
	if len(p.filtered) > 0 {
		p.cursor = len(p.filtered) - 1
	}
}

func (p *KeysPanel) PageDown(n int) {
	p.cursor += n
	if p.cursor >= len(p.filtered) {
		p.cursor = len(p.filtered) - 1
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
}

func (p *KeysPanel) PageUp(n int) {
	p.cursor -= n
	if p.cursor < 0 {
		p.cursor = 0
	}
}

func (p *KeysPanel) adjustScroll(listH int) {
	if p.cursor < p.scrollOffset {
		p.scrollOffset = p.cursor
	} else if p.cursor >= p.scrollOffset+listH {
		p.scrollOffset = p.cursor - listH + 1
	}
}

// FindAndSelect moves cursor to matching key if found.
func (p *KeysPanel) FindAndSelect(key string) {
	for i, k := range p.filtered {
		if k == key {
			p.cursor = i
			return
		}
	}
	p.cursor = 0
}

// RemoveSelected removes current cursor key from filtered list.
func (p *KeysPanel) RemoveKey(key string) {
	var newKeys []string
	for _, k := range p.keys {
		if k != key {
			newKeys = append(newKeys, k)
		}
	}
	p.keys = newKeys
	p.applyFilter()
	if p.cursor >= len(p.filtered) && p.cursor > 0 {
		p.cursor--
	}
}

func (p *KeysPanel) StartFilter() {
	p.filtering = true
	p.filterInput.Focus()
}

func (p *KeysPanel) StopFilter(clear bool) {
	p.filtering = false
	p.filterInput.Blur()
	if clear {
		p.filterInput.SetValue("")
		p.filterVal = ""
		p.applyFilter()
	}
}

func (p *KeysPanel) UpdateFilter(val string) {
	old := p.filterVal
	p.filterVal = val
	if old != val {
		prevSel := p.Selected()
		p.applyFilter()
		p.cursor = 0
		p.scrollOffset = 0
		// try to keep selection
		if prevSel != "" {
			p.FindAndSelect(prevSel)
		}
	}
}

func (p *KeysPanel) Render(width, height int, active bool, typeMap map[string]string) string {
	border := styleBorder
	if active {
		border = styleBorderActive
	}

	innerW := width - 4
	listH := height - 5 // border(2) + title(1) + filter(1) + pad(1)
	if p.filtering || p.filterVal != "" {
		listH--
	}

	p.adjustScroll(listH)

	// Title
	var titleStr string
	if len(p.filtered) != len(p.keys) {
		titleStr = fmt.Sprintf("Keys  %s/%s",
			styleWarning.Render(fmt.Sprintf("%d", len(p.filtered))),
			styleMuted.Render(fmt.Sprintf("%d", len(p.keys))),
		)
	} else {
		titleStr = fmt.Sprintf("Keys  %s", styleMuted.Render(fmt.Sprintf("%d", len(p.keys))))
	}
	title := stylePanelTitle.Render(titleStr)

	var lines []string
	lines = append(lines, title)

	// Filter row
	if p.filtering {
		lines = append(lines, p.filterInput.View())
	} else if p.filterVal != "" {
		lines = append(lines, styleFilterPrompt.Render("/")+styleInfo.Render(p.filterVal)+"  "+styleMuted.Render("[esc clear]"))
	}

	// Key rows
	if len(p.filtered) == 0 {
		lines = append(lines, styleMuted.Render("  no keys found"))
	} else {
		end := p.scrollOffset + listH
		if end > len(p.filtered) {
			end = len(p.filtered)
		}
		for i := p.scrollOffset; i < end; i++ {
			key := p.filtered[i]
			typ := typeMap[key]
			badge := keyTypeBadge(typ)

			maxKeyLen := innerW - lipgloss.Width(badge) - 2
			display := key
			if len(display) > maxKeyLen {
				display = display[:maxKeyLen-1] + "…"
			}

			row := badge + " " + display
			if i == p.cursor {
				if active {
					row = styleSelected.Width(innerW).Render(badge + " " + display)
				} else {
					row = styleSelectedAlt.Width(innerW).Render(badge + " " + display)
				}
			} else {
				row = badge + " " + keyTypeStyle(typ).Render(display)
			}
			lines = append(lines, row)
		}
	}

	// Scroll indicator
	if len(p.filtered) > listH {
		pct := 100 * p.cursor / (len(p.filtered) - 1)
		lines = append(lines, styleMuted.Render(fmt.Sprintf("  ↕ %d%%  [%d/%d]", pct, p.cursor+1, len(p.filtered))))
	}

	content := strings.Join(lines, "\n")
	return border.Width(width - 2).Height(height - 2).Render(content)
}
