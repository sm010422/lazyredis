package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
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
	bgVisible := lipgloss.Width(bg)
	if bgVisible < bgW {
		bg += strings.Repeat(" ", bgW-bgVisible)
	}

	left := ansiCut(bg, 0, at)
	right := ansiCut(bg, at+lipgloss.Width(fg), bgW)

	// Reset between sections to prevent ANSI colour bleeding.
	return left + "\x1b[0m" + fg + "\x1b[0m" + right
}

// ansiCut returns visible characters in [from, to) from an ANSI string.
func ansiCut(s string, from, to int) string {
	if from >= to {
		return ""
	}
	return ansiTake(ansiDrop(s, from), to-from)
}

// ansiTake returns the first n visible characters of s, preserving ANSI codes.
func ansiTake(s string, n int) string {
	if n <= 0 {
		return ""
	}
	var buf strings.Builder
	vis := 0
	runes := []rune(s)
	for i := 0; i < len(runes); {
		if esc, j := parseEscape(runes, i); esc {
			buf.WriteString(string(runes[i:j]))
			i = j
		} else {
			if vis >= n {
				break
			}
			buf.WriteRune(runes[i])
			vis++
			i++
		}
	}
	return buf.String()
}

// ansiDrop skips the first n visible characters of s, preserving ANSI state
// by keeping the escape codes from the dropped portion as a prefix.
func ansiDrop(s string, n int) string {
	if n <= 0 {
		return s
	}
	var codes strings.Builder
	vis := 0
	runes := []rune(s)
	i := 0
	for i < len(runes) && vis < n {
		if esc, j := parseEscape(runes, i); esc {
			codes.WriteString(string(runes[i:j]))
			i = j
		} else {
			vis++
			i++
		}
	}
	return codes.String() + string(runes[i:])
}

// parseEscape checks if position i is the start of an ANSI CSI escape sequence.
// Returns (true, end) or (false, i).
func parseEscape(runes []rune, i int) (bool, int) {
	if i+1 >= len(runes) || runes[i] != '\x1b' || runes[i+1] != '[' {
		return false, i
	}
	j := i + 2
	for j < len(runes) && !isANSIFinal(runes[j]) {
		j++
	}
	if j < len(runes) {
		j++
	}
	return true, j
}

func isANSIFinal(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
}
