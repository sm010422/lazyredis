package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tidwall/pretty"

	redisclient "github.com/parksangmin/lazyredis/pkg/redis"
)

type ZSetMemberRow struct {
	Member string
	Score  float64
}

// ValueView is the right-side panel showing a key's value.
// It is purely a renderer — all data is set by the app model.
type ValueView struct {
	Key     string
	Info    *redisclient.KeyInfo
	scroll  int

	// Structured data for type-specific sub-selection
	listItems  []string
	hashKeys   []string
	hashFields map[string]string
	setMembers []string
	zsetItems  []ZSetMemberRow
	subCursor  int // selected row within the value (hash field, list index…)
}

func (v *ValueView) Reset() {
	v.Key = ""
	v.Info = nil
	v.scroll = 0
	v.subCursor = 0
	v.listItems = nil
	v.hashKeys = nil
	v.hashFields = nil
	v.setMembers = nil
	v.zsetItems = nil
}

func (v *ValueView) SetList(items []string)              { v.listItems = items; v.subCursor = 0 }
func (v *ValueView) SetHash(fields map[string]string, keys []string) {
	v.hashFields = fields
	v.hashKeys = keys
	v.subCursor = 0
}
func (v *ValueView) SetSet(members []string)             { v.setMembers = members; v.subCursor = 0 }
func (v *ValueView) SetZSet(items []ZSetMemberRow) { v.zsetItems = items; v.subCursor = 0 }

func (v *ValueView) ScrollDown() { v.scroll++ }
func (v *ValueView) ScrollUp() {
	if v.scroll > 0 {
		v.scroll--
	}
}
func (v *ValueView) ScrollTop() { v.scroll = 0 }

func (v *ValueView) SubDown(max int) {
	if v.subCursor < max-1 {
		v.subCursor++
	}
}
func (v *ValueView) SubUp() {
	if v.subCursor > 0 {
		v.subCursor--
	}
}

func (v *ValueView) SelectedListIdx() int    { return v.subCursor }
func (v *ValueView) SelectedHashField() string {
	if v.hashKeys == nil || v.subCursor >= len(v.hashKeys) {
		return ""
	}
	return v.hashKeys[v.subCursor]
}
func (v *ValueView) SelectedSetMember() string {
	if v.setMembers == nil || v.subCursor >= len(v.setMembers) {
		return ""
	}
	return v.setMembers[v.subCursor]
}
func (v *ValueView) SelectedZSetMember() *ZSetMemberRow {
	if v.zsetItems == nil || v.subCursor >= len(v.zsetItems) {
		return nil
	}
	return &v.zsetItems[v.subCursor]
}

// StringValue is set externally by app.
var stringValueCache = map[string]string{}

func (v *ValueView) SetStringValue(s string) {
	if v.Key != "" {
		stringValueCache[v.Key] = s
	}
}

func (v *ValueView) Render(width, height int, active bool) string {
	border := styleBorder
	if active {
		border = styleBorderActive
	}

	innerW := width - 4
	innerH := height - 4

	if v.Key == "" || v.Info == nil {
		placeholder := lipgloss.Place(innerW, innerH,
			lipgloss.Center, lipgloss.Center,
			styleMuted.Render("select a key to view its value"),
		)
		return border.Width(width - 2).Height(height - 2).Render(placeholder)
	}

	typ := string(v.Info.Type)
	badge := keyTypeBadge(typ)
	title := stylePanelTitle.Render(v.Key) + "  " + badge

	var body string
	switch v.Info.Type {
	case redisclient.TypeString:
		body = v.renderString(innerW, innerH)
	case redisclient.TypeList:
		body = v.renderList(innerW, innerH)
	case redisclient.TypeHash:
		body = v.renderHash(innerW, innerH)
	case redisclient.TypeSet:
		body = v.renderSet(innerW, innerH)
	case redisclient.TypeZSet:
		body = v.renderZSet(innerW, innerH)
	case redisclient.TypeStream:
		body = v.renderStream(innerW, innerH)
	default:
		body = styleMuted.Render("unsupported type: " + typ)
	}

	content := title + "\n" + body
	return border.Width(width - 2).Height(height - 2).Render(content)
}

func (v *ValueView) renderString(w, h int) string {
	raw := stringValueCache[v.Key]
	if raw == "" {
		return styleMuted.Render("(empty)")
	}

	// JSON pretty print
	if redisclient.IsJSON(raw) {
		p := string(pretty.Color(pretty.Pretty([]byte(raw)), nil))
		lines := strings.Split(strings.TrimRight(p, "\n"), "\n")
		return v.paginate(lines, w, h)
	}

	lines := strings.Split(raw, "\n")
	return v.paginate(lines, w, h)
}

func (v *ValueView) renderList(w, h int) string {
	if len(v.listItems) == 0 {
		return styleMuted.Render("(empty list)")
	}
	lines := make([]string, len(v.listItems))
	for i, item := range v.listItems {
		idxStr := styleMuted.Render(fmt.Sprintf("%4d ", i))
		val := item
		if len(val) > w-6 {
			val = val[:w-7] + "…"
		}
		row := idxStr + styleInfo.Render(val)
		if i == v.subCursor {
			row = styleSelected.Width(w).Render(fmt.Sprintf("%4d %s", i, val))
		}
		lines[i] = row
	}
	return v.paginate(lines, w, h)
}

func (v *ValueView) renderHash(w, h int) string {
	if len(v.hashKeys) == 0 {
		return styleMuted.Render("(empty hash)")
	}
	colW := 22
	lines := make([]string, len(v.hashKeys))
	for i, k := range v.hashKeys {
		val := v.hashFields[k]
		fk := k
		if len(fk) > colW {
			fk = fk[:colW-1] + "…"
		}
		fv := val
		if len(fv) > w-colW-4 {
			fv = fv[:w-colW-5] + "…"
		}
		row := fmt.Sprintf("%-*s  %s", colW, fk, fv)
		if i == v.subCursor {
			row = styleSelected.Width(w).Render(fmt.Sprintf("%-*s  %s", colW, fk, fv))
		} else {
			row = keyTypeStyle("hash").Render(fmt.Sprintf("%-*s", colW, fk)) + "  " + styleInfo.Render(fv)
		}
		lines[i] = row
	}
	header := styleMuted.Render(fmt.Sprintf("%-*s  %s", colW, "FIELD", "VALUE"))
	all := append([]string{header}, lines...)
	return v.paginate(all, w, h)
}

func (v *ValueView) renderSet(w, h int) string {
	if len(v.setMembers) == 0 {
		return styleMuted.Render("(empty set)")
	}
	lines := make([]string, len(v.setMembers))
	for i, m := range v.setMembers {
		val := m
		if len(val) > w-2 {
			val = val[:w-3] + "…"
		}
		if i == v.subCursor {
			lines[i] = styleSelected.Width(w).Render(val)
		} else {
			lines[i] = styleInfo.Render(val)
		}
	}
	return v.paginate(lines, w, h)
}

func (v *ValueView) renderZSet(w, h int) string {
	if len(v.zsetItems) == 0 {
		return styleMuted.Render("(empty sorted set)")
	}
	lines := make([]string, len(v.zsetItems))
	scoreW := 12
	for i, z := range v.zsetItems {
		score := fmt.Sprintf("%-*g", scoreW, z.Score)
		member := z.Member
		if len(member) > w-scoreW-4 {
			member = member[:w-scoreW-5] + "…"
		}
		if i == v.subCursor {
			lines[i] = styleSelected.Width(w).Render(fmt.Sprintf("%4d  %-*s  %s", i+1, scoreW, score, member))
		} else {
			lines[i] = styleMuted.Render(fmt.Sprintf("%4d", i+1)) + "  " +
				keyTypeStyle("zset").Render(fmt.Sprintf("%-*s", scoreW, score)) + "  " +
				styleInfo.Render(member)
		}
	}
	header := styleMuted.Render(fmt.Sprintf("%4s  %-*s  %s", "RANK", scoreW, "SCORE", "MEMBER"))
	all := append([]string{header}, lines...)
	return v.paginate(all, w, h)
}

func (v *ValueView) renderStream(w, h int) string {
	raw := stringValueCache[v.Key]
	if raw == "" {
		return styleMuted.Render("(empty stream)")
	}
	lines := strings.Split(raw, "\n")
	return v.paginate(lines, w, h)
}

func (v *ValueView) paginate(lines []string, w, h int) string {
	if v.scroll >= len(lines) {
		v.scroll = max(0, len(lines)-1)
	}
	end := v.scroll + h
	if end > len(lines) {
		end = len(lines)
	}
	visible := lines[v.scroll:end]

	var result []string
	for _, l := range visible {
		if lipgloss.Width(l) > w {
			result = append(result, l[:w])
		} else {
			result = append(result, l)
		}
	}

	if len(lines) > h {
		pct := 0
		if len(lines) > 1 {
			pct = 100 * v.scroll / (len(lines) - 1)
		}
		result = append(result, styleMuted.Render(fmt.Sprintf("── %d%% (%d/%d lines) ──", pct, v.scroll+1, len(lines))))
	}

	return strings.Join(result, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
