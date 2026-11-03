package ui

import (
	"reflect"
	"testing"
)

func newTestKeysPanel(keys ...string) *KeysPanel {
	p := newKeysPanel()
	p.SetKeys(keys)
	return &p
}

func TestKeysPanelTreeNavigation(t *testing.T) {
	p := newTestKeysPanel(treeKeys...)

	// Root: [cache/ user/ config]
	if p.SelectedKey() != "" {
		t.Errorf("cursor on a dir: SelectedKey = %q, want empty", p.SelectedKey())
	}
	if got := p.SelectedKeyName(); got != "cache" {
		t.Errorf("SelectedKeyName on dir = %q, want cache", got)
	}

	p.MoveDown()
	p.EnterDir() // user/
	if got := breadcrumbString(p.pathSegs); got != "/user/" {
		t.Fatalf("path = %s, want /user/", got)
	}

	p.MoveBottom() // user:admin
	if got := p.SelectedKey(); got != "user:admin" {
		t.Errorf("SelectedKey = %q, want user:admin", got)
	}

	p.ExitDir()
	if len(p.pathSegs) != 0 || p.cursor != 0 {
		t.Errorf("after ExitDir: path=%v cursor=%d, want root and 0", p.pathSegs, p.cursor)
	}

	p.MoveDown()
	p.EnterDir()
	p.EnterDir() // user/1/
	p.GoRoot()
	if len(p.pathSegs) != 0 {
		t.Errorf("after GoRoot: path = %v, want root", p.pathSegs)
	}
}

func TestKeysPanelEnterDirOnLeafDoesNothing(t *testing.T) {
	p := newTestKeysPanel("a", "b")
	p.EnterDir()
	if len(p.pathSegs) != 0 {
		t.Errorf("EnterDir on a leaf changed path to %v", p.pathSegs)
	}
}

func TestKeysPanelCursorBounds(t *testing.T) {
	p := newTestKeysPanel("a", "b", "c")

	if p.MoveUp() {
		t.Error("MoveUp at top returned true")
	}
	p.MoveBottom()
	if p.MoveDown() {
		t.Error("MoveDown at bottom returned true")
	}

	p.MoveTop()
	p.PageDown(100)
	if p.cursor != 2 {
		t.Errorf("PageDown past end: cursor = %d, want 2", p.cursor)
	}
	p.PageUp(100)
	if p.cursor != 0 {
		t.Errorf("PageUp past start: cursor = %d, want 0", p.cursor)
	}

	empty := newTestKeysPanel()
	empty.PageDown(5)
	empty.MoveBottom()
	if empty.cursor != 0 || empty.SelectedKey() != "" {
		t.Errorf("empty panel: cursor=%d key=%q", empty.cursor, empty.SelectedKey())
	}
}

func TestKeysPanelFilterIsCaseInsensitiveAndFlat(t *testing.T) {
	p := newTestKeysPanel(treeKeys...)
	p.StartFilter()
	p.UpdateFilter("PROFILE")

	if want := []string{"user:1:profile", "user:2:profile"}; !reflect.DeepEqual(p.flatFiltered, want) {
		t.Fatalf("filtered = %v, want %v", p.flatFiltered, want)
	}
	p.MoveDown()
	if got := p.SelectedKey(); got != "user:2:profile" {
		t.Errorf("SelectedKey = %q, want user:2:profile", got)
	}

	p.StopFilter(true)
	if p.filtering || p.filterVal != "" || len(p.flatFiltered) != len(treeKeys) {
		t.Errorf("after StopFilter(clear): filtering=%v val=%q n=%d", p.filtering, p.filterVal, len(p.flatFiltered))
	}
}

func TestKeysPanelSelectDirExpandsToLeafKeys(t *testing.T) {
	p := newTestKeysPanel(treeKeys...)

	p.MoveDown()     // user/
	p.ToggleSelect() // select the whole dir
	p.MoveDown()     // config
	p.ToggleSelect()

	want := []string{"config", "user:1:profile", "user:1:session", "user:2:profile", "user:admin"}
	if got := p.SelectedLeafKeys(); !reflect.DeepEqual(got, want) {
		t.Errorf("SelectedLeafKeys = %v, want %v", got, want)
	}

	p.ToggleSelect() // unselect config
	if got := len(p.SelectedLeafKeys()); got != 4 {
		t.Errorf("after unselect: %d keys, want 4", got)
	}

	p.ClearSelection()
	if p.HasSelection() {
		t.Error("HasSelection after ClearSelection")
	}
}

func TestKeysPanelRangeSelect(t *testing.T) {
	p := newTestKeysPanel("a", "b", "c", "d")
	p.MoveDown() // b
	p.ExtendSelectDown()
	p.ExtendSelectDown() // b..d

	if want := []string{"b", "c", "d"}; !reflect.DeepEqual(p.SelectedLeafKeys(), want) {
		t.Errorf("range = %v, want %v", p.SelectedLeafKeys(), want)
	}
}

func TestKeysPanelRemoveKeyKeepsCursorInRange(t *testing.T) {
	p := newTestKeysPanel("a", "b", "c")
	p.MoveBottom()
	p.RemoveKey("c")
	if got := p.SelectedKey(); got != "b" {
		t.Errorf("after removing the last key, SelectedKey = %q, want b", got)
	}
}

func TestKeysPanelRemoveLastKeyLeavesEmptyDirs(t *testing.T) {
	// Deleting the only key under a:b:c leaves both a:b and a empty.
	// The panel should climb out of every empty level, not just one.
	p := newTestKeysPanel("a:b:c", "z")
	p.EnterDir() // a/
	p.EnterDir() // a/b/

	p.RemoveKey("a:b:c")
	if len(p.pathSegs) != 0 {
		t.Errorf("path = %s after deleting the only nested key, want /", breadcrumbString(p.pathSegs))
	}
	if got := p.SelectedKey(); got != "z" {
		t.Errorf("SelectedKey = %q, want z", got)
	}

	q := newTestKeysPanel("a:b:c", "a:b:d", "z")
	q.EnterDir()
	q.EnterDir()
	q.RemoveKeys([]string{"a:b:c", "a:b:d"})
	if len(q.pathSegs) != 0 {
		t.Errorf("RemoveKeys: path = %s, want /", breadcrumbString(q.pathSegs))
	}
}

func TestKeysPanelFindAndSelect(t *testing.T) {
	p := newTestKeysPanel("a", "b", "c")
	p.FindAndSelect("c")
	if p.SelectedKey() != "c" {
		t.Errorf("FindAndSelect(c): SelectedKey = %q", p.SelectedKey())
	}
	p.FindAndSelect("missing")
	if p.cursor != 0 {
		t.Errorf("FindAndSelect(missing): cursor = %d, want 0", p.cursor)
	}
}
