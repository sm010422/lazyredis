package ui

import (
	"testing"
	"time"
)

func TestRefreshIntervalRoundTrip(t *testing.T) {
	// The connect modal stores an index into refreshLabels and sends back the
	// label; both directions must agree for every option.
	for i, label := range refreshLabels {
		d := parseRefreshInterval(label)
		if got := refreshIntervalIdx(d); got != i {
			t.Errorf("label %q -> %v -> index %d, want %d", label, d, got, i)
		}
	}
	if d := parseRefreshInterval("off"); d != 0 {
		t.Errorf(`parseRefreshInterval("off") = %v, want 0`, d)
	}
	if got := refreshIntervalIdx(7 * time.Second); got != 0 {
		t.Errorf("unknown interval index = %d, want 0 (off)", got)
	}
}
