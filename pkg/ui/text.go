package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// All widths here are terminal cells, not bytes. Hangul, CJK and most emoji
// take two cells, and styled strings carry ANSI escapes that take none, so
// slicing by byte length breaks both: it splits multi-byte characters into
// invalid UTF-8 and cuts escape sequences in half.

// truncate flattens newlines and shortens s to at most n cells, ending with
// "…" when anything was cut.
func truncate(s string, n int) string {
	return truncateWidth(strings.ReplaceAll(s, "\n", " "), n)
}

// truncateWidth shortens s to at most w cells, ending with "…" when anything
// was cut. ANSI escapes in s are kept intact.
func truncateWidth(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return ansi.Truncate(s, w, "…")
}

// clipWidth cuts s to at most w cells without adding an ellipsis.
func clipWidth(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return ansi.Truncate(s, w, "")
}

// padRight pads s with spaces to w cells. It never cuts s.
func padRight(s string, w int) string {
	if gap := w - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}
