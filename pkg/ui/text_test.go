package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		in   string
		n    int
		want string
	}{
		{"fits", "hello", 10, "hello"},
		{"exact", "hello", 5, "hello"},
		{"cut ascii", "hello world", 6, "hello…"},
		{"newlines become spaces", "a\nb", 5, "a b"},
		// Hangul is two cells wide. Six cells hold two syllables plus "…" and
		// one cell of space that a third syllable would not fit into.
		{"cut korean", "사용자세션", 6, "사용…"},
		{"korean fits", "세션", 4, "세션"},
		{"zero width", "hello", 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.in, tt.n)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.in, tt.n, got, tt.want)
			}
			if !utf8.ValidString(got) {
				t.Errorf("truncate(%q, %d) returned invalid UTF-8: %q", tt.in, tt.n, got)
			}
			if w := lipgloss.Width(got); w > tt.n {
				t.Errorf("truncate(%q, %d) is %d cells wide", tt.in, tt.n, w)
			}
		})
	}
}

func TestTruncateKeepsStyledTextIntact(t *testing.T) {
	styled := "\x1b[31mredis_version\x1b[0m:7.2.4"
	got := truncate(styled, 8)
	if !strings.HasPrefix(got, "\x1b[31m") {
		t.Errorf("escape sequence was cut: %q", got)
	}
	if w := lipgloss.Width(got); w > 8 {
		t.Errorf("width = %d, want <= 8", w)
	}
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		in   string
		w    int
		want string
	}{
		{"ab", 4, "ab  "},
		{"세션", 6, "세션  "},
		{"abcdef", 4, "abcdef"}, // never cuts; callers truncate first
	}
	for _, tt := range tests {
		if got := padRight(tt.in, tt.w); got != tt.want {
			t.Errorf("padRight(%q, %d) = %q, want %q", tt.in, tt.w, got, tt.want)
		}
	}
}

func TestOverlayCenterKeepsLineWidth(t *testing.T) {
	tests := []struct {
		name string
		bg   []string
	}{
		{"ascii", []string{
			"│ user:1             │",
			"│ session:abc        │",
			"│ cache:home         │",
		}},
		// Hangul under and around the overlay. Each syllable is 2 cells, so
		// counting runes instead of cells misplaces both cuts.
		{"korean under overlay", []string{
			"│ 사용자:가나다라마바 │",
			"│ 세션:가나다라마바사 │",
			"│ 캐시:가나다라마바사 │",
		}},
	}
	fg := "[ok]"
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bg := strings.Join(tt.bg, "\n")
			bgW := lipgloss.Width(tt.bg[0])

			lines := strings.Split(overlayCenter(bg, fg), "\n")
			for i, l := range lines {
				if !utf8.ValidString(l) {
					t.Errorf("line %d is not valid UTF-8: %q", i, l)
				}
				if w := lipgloss.Width(l); w != bgW {
					t.Errorf("line %d is %d cells wide, want %d: %q", i, w, bgW, ansi.Strip(l))
				}
			}

			mid := ansi.Strip(lines[1])
			if !strings.HasSuffix(mid, "│") {
				t.Errorf("right border moved: %q", mid)
			}
			// fg must start at the centre column, counted in cells.
			wantX := (bgW - lipgloss.Width(fg)) / 2
			if gotX := lipgloss.Width(mid[:strings.Index(mid, fg)]); gotX != wantX {
				t.Errorf("overlay starts at cell %d, want %d: %q", gotX, wantX, mid)
			}
		})
	}
}
