package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// overlayCenter renders fg centered over bg, keeping background content
// (panel borders, content) visible everywhere outside the fg area.
func overlayCenter(bg, fg string) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")

	bgH := len(bgLines)
	fgH := len(fgLines)

	fgW := 0
	for _, l := range fgLines {
		if w := lipgloss.Width(l); w > fgW {
			fgW = w
		}
	}

	bgW := 0
	for _, l := range bgLines {
		if w := lipgloss.Width(l); w > bgW {
			bgW = w
		}
	}

	startY := (bgH - fgH) / 2
	startX := (bgW - fgW) / 2
	if startX < 0 {
		startX = 0
	}
	if startY < 0 {
		startY = 0
	}

	result := make([]string, len(bgLines))
	copy(result, bgLines)

	for i, fgLine := range fgLines {
		bgIdx := startY + i
		if bgIdx < 0 || bgIdx >= len(bgLines) {
			continue
		}
		result[bgIdx] = overlayLine(bgLines[bgIdx], fgLine, startX, bgW)
	}

	return strings.Join(result, "\n")
}

// overlayLine overlays fg on top of bg at visual column `at`, keeping
// background characters outside the fg span.
func overlayLine(bg, fg string, at, bgW int) string {
	bg = padRight(bg, bgW)
	fgW := lipgloss.Width(fg)

	// A wide character (Hangul, CJK) that straddles either edge of fg is
	// dropped by the cut, so pad each side back to its expected width to
	// keep the right-hand border in place.
	left := padRight(ansi.Cut(bg, 0, at), at)
	right := ansi.Cut(bg, at+fgW, bgW)
	if gap := bgW - at - fgW - lipgloss.Width(right); gap > 0 {
		right = strings.Repeat(" ", gap) + right
	}

	// Reset between sections to prevent ANSI colour bleeding.
	return left + "\x1b[0m" + fg + "\x1b[0m" + right
}
