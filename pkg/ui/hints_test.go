package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// On a narrow terminal the bar drops hints instead of wrapping, and the
// "Keybindings: ?" pointer to the full list is never the one dropped.
func TestRenderHintsFitsWidthAndKeepsHelpHint(t *testing.T) {
	hints := [][2]string{
		{"Search", "/"}, {"New", "n"}, {"Delete", "d"},
		{"Rename", "R"}, {"Keybindings", "?"},
	}
	out := renderHints(hints, 40)
	if w := lipgloss.Width(out); w > 40 {
		t.Fatalf("width %d > 40: %q", w, out)
	}
	if !strings.Contains(out, "Keybindings") {
		t.Fatalf("help hint dropped: %q", out)
	}
}

// "?" opens the help tab from any tab and closes it back to where it came from.
func TestQuestionMarkTogglesHelpAndRestoresTab(t *testing.T) {
	a, _ := newTestApp(t)
	a.tab = tabServer
	q := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}

	a.Update(q)
	if a.tab != tabHelp {
		t.Fatalf("tab = %v, want help", a.tab)
	}
	a.Update(q)
	if a.tab != tabServer {
		t.Fatalf("tab = %v, want server", a.tab)
	}
}
