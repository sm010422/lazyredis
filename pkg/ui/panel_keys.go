package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

type KeysPanel struct {
	allKeys      []string   // all keys loaded from Redis (sorted)
	pathSegs     []string   // current breadcrumb in tree
	nodes        []treeNode // visible nodes at current path (tree mode)
	cursor       int
	scrollOffset int

	// multi-select: value is the "id" of the node:
	//   leaf  → fullKey
	//   dir   → prefix
	selected    map[string]bool
	rangeAnchor int // -1 when no range in progress

	// search/filter mode
	filterInput  textinput.Model
	filtering    bool
	filterVal    string
	flatFiltered []string // flat key list when filtering
}

var styleFilterPrompt = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)

func newKeysPanel() KeysPanel {
	fi := textinput.New()
	fi.Placeholder = "search keys (fuzzy)"
	fi.CharLimit = 256
	fi.PromptStyle = styleFilterPrompt
	fi.Prompt = "/"
	return KeysPanel{
		filterInput: fi,
		selected:    make(map[string]bool),
		rangeAnchor: -1,
	}
}

// ---- data ----

func (p *KeysPanel) SetKeys(keys []string) {
	p.allKeys = keys
	p.rebuildNodes()
}

func (p *KeysPanel) rebuildNodes() {
	p.nodes = buildNodes(p.allKeys, p.pathSegs)
	// clamp cursor
	if p.cursor >= len(p.nodes) {
		p.cursor = max(0, len(p.nodes)-1)
	}
	p.applyFilter()
}

func (p *KeysPanel) applyFilter() {
	pattern := strings.ToLower(p.filterVal)
	if pattern == "" {
		p.flatFiltered = make([]string, len(p.allKeys))
		copy(p.flatFiltered, p.allKeys)
		return
	}
	var out []string
	for _, k := range p.allKeys {
		if strings.Contains(strings.ToLower(k), pattern) {
			out = append(out, k)
		}
	}
	p.flatFiltered = out
}

// ---- selection API ----

// SelectedKey returns the full Redis key when cursor is on a leaf (in either
// mode). Returns "" when cursor is on a directory node.
func (p *KeysPanel) SelectedKey() string {
	if p.filtering {
		if p.cursor < len(p.flatFiltered) {
			return p.flatFiltered[p.cursor]
		}
		return ""
	}
	if p.cursor < len(p.nodes) && p.nodes[p.cursor].kind == nodeLeaf {
		return p.nodes[p.cursor].fullKey
	}
	return ""
}

// SelectedNode returns the current tree node or nil in filter mode.
func (p *KeysPanel) SelectedNode() *treeNode {
	if p.filtering || p.cursor >= len(p.nodes) {
		return nil
	}
	n := p.nodes[p.cursor]
	return &n
}

// Selected is kept for backward compatibility; identical to SelectedKey.
func (p *KeysPanel) Selected() string { return p.SelectedKey() }

// SelectedKeyName returns the display name of the current item for copy.
func (p *KeysPanel) SelectedKeyName() string {
	if p.filtering {
		return p.SelectedKey()
	}
	if p.cursor < len(p.nodes) {
		n := p.nodes[p.cursor]
		if n.kind == nodeLeaf {
			return n.fullKey
		}
		// dir: return the prefix (without trailing delimiter)
		return strings.TrimSuffix(n.prefix, treeDelimiter)
	}
	return ""
}

// ---- tree navigation ----

func (p *KeysPanel) EnterDir() {
	if p.cursor >= len(p.nodes) {
		return
	}
	n := p.nodes[p.cursor]
	if n.kind != nodeDir {
		return
	}
	p.pathSegs = append(p.pathSegs, n.name)
	p.cursor = 0
	p.scrollOffset = 0
	p.rangeAnchor = -1
	p.rebuildNodes()
}

func (p *KeysPanel) ExitDir() {
	if len(p.pathSegs) == 0 {
		return
	}
	p.pathSegs = parentPath(p.pathSegs)
	p.cursor = 0
	p.scrollOffset = 0
	p.rangeAnchor = -1
	p.rebuildNodes()
}

func (p *KeysPanel) GoRoot() {
	p.pathSegs = nil
	p.cursor = 0
	p.scrollOffset = 0
	p.rangeAnchor = -1
	p.rebuildNodes()
}

// ---- cursor movement ----

func (p *KeysPanel) listLen() int {
	if p.filtering {
		return len(p.flatFiltered)
	}
	return len(p.nodes)
}

func (p *KeysPanel) MoveDown() bool {
	if p.cursor < p.listLen()-1 {
		p.cursor++
		p.rangeAnchor = -1
		return true
	}
	return false
}

func (p *KeysPanel) MoveUp() bool {
	if p.cursor > 0 {
		p.cursor--
		p.rangeAnchor = -1
		return true
	}
	return false
}

func (p *KeysPanel) MoveTop() {
	p.cursor = 0
	p.scrollOffset = 0
	p.rangeAnchor = -1
}

func (p *KeysPanel) MoveBottom() {
	n := p.listLen()
	if n > 0 {
		p.cursor = n - 1
	}
	p.rangeAnchor = -1
}

func (p *KeysPanel) PageDown(n int) {
	p.cursor += n
	if p.cursor >= p.listLen() {
		p.cursor = p.listLen() - 1
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
	p.rangeAnchor = -1
}

func (p *KeysPanel) PageUp(n int) {
	p.cursor -= n
	if p.cursor < 0 {
		p.cursor = 0
	}
	p.rangeAnchor = -1
}

func (p *KeysPanel) adjustScroll(listH int) {
	if p.cursor < p.scrollOffset {
		p.scrollOffset = p.cursor
	} else if p.cursor >= p.scrollOffset+listH {
		p.scrollOffset = p.cursor - listH + 1
	}
}

// ---- multi-select ----

func (p *KeysPanel) nodeID(idx int) string {
	if p.filtering {
		if idx < len(p.flatFiltered) {
			return p.flatFiltered[idx]
		}
		return ""
	}
	if idx < len(p.nodes) {
		n := p.nodes[idx]
		if n.kind == nodeLeaf {
			return n.fullKey
		}
		return n.prefix
	}
	return ""
}

func (p *KeysPanel) ToggleSelect() {
	id := p.nodeID(p.cursor)
	if id == "" {
		return
	}
	if p.selected[id] {
		delete(p.selected, id)
	} else {
		p.selected[id] = true
	}
	p.rangeAnchor = p.cursor
}

func (p *KeysPanel) ExtendSelectDown() {
	if p.rangeAnchor < 0 {
		p.rangeAnchor = p.cursor
	}
	if p.cursor < p.listLen()-1 {
		p.cursor++
	}
	p.applyRange()
}

func (p *KeysPanel) ExtendSelectUp() {
	if p.rangeAnchor < 0 {
		p.rangeAnchor = p.cursor
	}
	if p.cursor > 0 {
		p.cursor--
	}
	p.applyRange()
}

func (p *KeysPanel) applyRange() {
	if p.rangeAnchor < 0 {
		return
	}
	lo, hi := p.rangeAnchor, p.cursor
	if lo > hi {
		lo, hi = hi, lo
	}
	for i := lo; i <= hi; i++ {
		if id := p.nodeID(i); id != "" {
			p.selected[id] = true
		}
	}
}

func (p *KeysPanel) ClearSelection() {
	p.selected = make(map[string]bool)
	p.rangeAnchor = -1
}

func (p *KeysPanel) HasSelection() bool {
	return len(p.selected) > 0
}

// SelectedLeafKeys returns all selected leaf Redis keys.
// For dirs in selection, it expands to all keys with that prefix.
func (p *KeysPanel) SelectedLeafKeys() []string {
	seen := make(map[string]bool)
	var out []string
	for id := range p.selected {
		if strings.HasSuffix(id, treeDelimiter) {
			// dir — expand
			for _, k := range keysWithPrefix(p.allKeys, id) {
				if !seen[k] {
					seen[k] = true
					out = append(out, k)
				}
			}
		} else {
			if !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	sort.Strings(out)
	return out
}

// ---- filter / search ----

func (p *KeysPanel) StartFilter() {
	p.filtering = true
	p.cursor = 0
	p.scrollOffset = 0
	p.rangeAnchor = -1
	p.filterInput.Focus()
	p.applyFilter()
}

func (p *KeysPanel) StopFilter(clear bool) {
	p.filtering = false
	p.filterInput.Blur()
	if clear {
		p.filterInput.SetValue("")
		p.filterVal = ""
		p.applyFilter()
	}
	p.cursor = 0
	p.scrollOffset = 0
}

func (p *KeysPanel) UpdateFilter(val string) {
	old := p.filterVal
	p.filterVal = val
	if old != val {
		p.applyFilter()
		p.cursor = 0
		p.scrollOffset = 0
	}
}

// ---- key removal ----

func (p *KeysPanel) FindAndSelect(key string) {
	if p.filtering {
		for i, k := range p.flatFiltered {
			if k == key {
				p.cursor = i
				return
			}
		}
		p.cursor = 0
		return
	}
	for i, n := range p.nodes {
		if n.kind == nodeLeaf && n.fullKey == key {
			p.cursor = i
			return
		}
	}
	p.cursor = 0
}

func (p *KeysPanel) RemoveKey(key string) {
	var newKeys []string
	for _, k := range p.allKeys {
		if k != key {
			newKeys = append(newKeys, k)
		}
	}
	p.allKeys = newKeys
	delete(p.selected, key)
	p.rebuildNodes()

	// If current path is now empty, go up.
	if len(p.nodes) == 0 && len(p.pathSegs) > 0 {
		p.ExitDir()
	}

	if p.cursor >= p.listLen() && p.cursor > 0 {
		p.cursor--
	}
}

func (p *KeysPanel) RemoveKeys(keys []string) {
	rm := make(map[string]bool, len(keys))
	for _, k := range keys {
		rm[k] = true
		delete(p.selected, k)
	}
	var newKeys []string
	for _, k := range p.allKeys {
		if !rm[k] {
			newKeys = append(newKeys, k)
		}
	}
	p.allKeys = newKeys
	p.rebuildNodes()
	if len(p.nodes) == 0 && len(p.pathSegs) > 0 {
		p.ExitDir()
	}
	if p.cursor >= p.listLen() && p.cursor > 0 {
		p.cursor--
	}
}

// ---- render ----

func (p *KeysPanel) Render(width, height int, active bool, typeMap map[string]string, borderColor lipgloss.Color) string {
	border := styleBorder
	if active {
		border = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor)
	}

	innerW := width - 4
	listH := height - 5 // border(2) + title(1) + crumb/filter(1) + pad(1)

	p.adjustScroll(listH)

	// ---- title ----
	totalStr := styleMuted.Render(fmt.Sprintf("%d", len(p.allKeys)))
	var titleStr string
	if p.filtering {
		titleStr = fmt.Sprintf("Keys  %s/%s  %s",
			styleWarning.Render(fmt.Sprintf("%d", len(p.flatFiltered))),
			totalStr,
			styleMuted.Render("[search]"),
		)
	} else if len(p.pathSegs) > 0 {
		titleStr = fmt.Sprintf("Keys  %s  %s",
			totalStr,
			stylePanelTitle.Render(breadcrumbString(p.pathSegs)),
		)
	} else {
		titleStr = "Keys  " + totalStr
	}
	title := stylePanelTitle.Render(titleStr)

	var lines []string
	lines = append(lines, title)

	// ---- filter / breadcrumb row ----
	if p.filtering {
		lines = append(lines, p.filterInput.View())
	} else if p.filterVal != "" {
		lines = append(lines, styleFilterPrompt.Render("/")+styleInfo.Render(p.filterVal)+"  "+styleMuted.Render("[esc clear]"))
	} else if len(p.pathSegs) > 0 {
		lines = append(lines, styleMuted.Render("← backspace  esc root"))
	}

	// ---- multi-select summary ----
	if len(p.selected) > 0 {
		lines = append(lines, styleWarning.Render(fmt.Sprintf("● %d selected  (d=delete  ctrl+space=toggle)", len(p.selected))))
		listH--
	}

	// ---- items ----
	var items []string
	var itemIDs []string

	if p.filtering {
		for _, k := range p.flatFiltered {
			items = append(items, k)
			itemIDs = append(itemIDs, k)
		}
	} else {
		for _, n := range p.nodes {
			if n.kind == nodeDir {
				items = append(items, fmt.Sprintf("▶ %s/  (%d)", n.name, n.count))
			} else {
				items = append(items, n.name)
			}
			if n.kind == nodeLeaf {
				itemIDs = append(itemIDs, n.fullKey)
			} else {
				itemIDs = append(itemIDs, n.prefix)
			}
		}
	}

	if len(items) == 0 {
		lines = append(lines, styleMuted.Render("  (empty)"))
	} else {
		end := p.scrollOffset + listH
		if end > len(items) {
			end = len(items)
		}
		for i := p.scrollOffset; i < end; i++ {
			label := items[i]
			id := itemIDs[i]
			isSel := p.selected[id]
			isDir := !p.filtering && i < len(p.nodes) && p.nodes[i].kind == nodeDir

			// build type badge for flat filter mode or leaves
			var badge string
			if p.filtering || (!isDir) {
				// get the full key for type lookup
				var fk string
				if p.filtering {
					fk = p.flatFiltered[i]
				} else if i < len(p.nodes) {
					fk = p.nodes[i].fullKey
				}
				typ := typeMap[fk]
				badge = keyTypeBadge(typ)

				maxKeyLen := innerW - lipgloss.Width(badge) - 2
				if len(label) > maxKeyLen {
					label = label[:maxKeyLen-1] + "…"
				}
			} else {
				// dir
				if len(label) > innerW-1 {
					label = label[:innerW-2] + "…"
				}
			}

			selMark := "  "
			if isSel {
				selMark = styleWarning.Render("● ")
			}

			var row string
			if i == p.cursor {
				if isDir {
					row = (func() string {
						if active {
							return styleSelected.Width(innerW).Render(selMark + label)
						}
						return styleSelectedAlt.Width(innerW).Render(selMark + label)
					})()
				} else {
					inner := selMark + badge + " " + label
					if active {
						row = styleSelected.Width(innerW).Render(inner)
					} else {
						row = styleSelectedAlt.Width(innerW).Render(inner)
					}
				}
			} else {
				if isDir {
					row = selMark + styleWarning.Render(label)
				} else {
					typ := typeMap[itemIDs[i]]
					row = selMark + badge + " " + keyTypeStyle(typ).Render(label)
				}
			}
			lines = append(lines, row)
		}
	}

	// ---- scroll indicator ----
	n := len(items)
	if n > listH {
		pct := 0
		if n > 1 {
			pct = 100 * p.cursor / (n - 1)
		}
		lines = append(lines, styleMuted.Render(fmt.Sprintf("  ↕ %d%%  [%d/%d]", pct, p.cursor+1, n)))
	}

	content := strings.Join(lines, "\n")
	return border.Width(width - 2).Height(height - 2).Render(content)
}
