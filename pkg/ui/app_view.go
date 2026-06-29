package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (a *App) View() string {
	if a.width == 0 {
		return "Loading…"
	}

	header := a.renderHeader()
	tabs := a.renderTabs()
	statusBar := a.renderStatusBar()

	bodyH := a.height - lipgloss.Height(header) - lipgloss.Height(tabs) - lipgloss.Height(statusBar)

	var body string
	switch a.tab {
	case tabKeys:
		body = a.renderKeysLayout(bodyH)
	case tabPubSub:
		body = a.pubsub.Render(a.width, bodyH)
	case tabServer:
		body = a.server.Render(a.width, bodyH)
	case tabHelp:
		body = a.renderHelp(bodyH)
	}

	view := lipgloss.JoinVertical(lipgloss.Left, header, tabs, body, statusBar)

	if a.modal != nil {
		return overlayCenter(view, a.modal.View(a.width))
	}

	return view
}

func (a *App) renderHeader() string {
	connStr := styleError.Render("● DISCONNECTED")
	if a.connected {
		connStr = styleSuccess.Render("● CONNECTED")
	}

	dbStr := styleWarning.Render(fmt.Sprintf("db%d", a.currentDB))
	keysStr := styleMuted.Render(fmt.Sprintf("%d keys", a.dbSize))
	scheme := "redis://"
	if a.cfg.TLS {
		scheme = "rediss://"
	}
	addrStr := styleInfo.Render(scheme + a.cfg.Addr())

	left := styleTitle.Render("LazyRedis") + "  " + connStr + "  " + dbStr + "  " + keysStr
	right := addrStr

	gap := a.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat(" ", gap) + right

	return lipgloss.NewStyle().
		Background(colorBg2).
		Width(a.width).
		Render(line)
}

func (a *App) renderTabs() string {
	var parts []string
	for i, label := range tabLabels {
		if tabID(i) == a.tab {
			parts = append(parts, styleTabActive.Render(label))
		} else {
			parts = append(parts, styleTabInactive.Render(label))
		}
	}
	tabs := strings.Join(parts, "")
	pad := a.width - lipgloss.Width(tabs)
	if pad > 0 {
		tabs += lipgloss.NewStyle().Background(colorSurface).Width(pad).Render("")
	}
	return tabs
}

func (a *App) renderKeysLayout(bodyH int) string {
	leftW := a.width * 28 / 100
	rightW := a.width - leftW
	topH := bodyH * 65 / 100
	botH := bodyH - topH

	leftPanel := a.keys.Render(leftW, bodyH, a.focus == focusKeyList, a.typeCache, a.profileColor)
	valuePanel := a.value.Render(rightW, topH, a.focus == focusValue)
	infoPanel := a.info.Render(rightW, botH, a.value.Info, a.cmdLog)

	right := lipgloss.JoinVertical(lipgloss.Left, valuePanel, infoPanel)
	layout := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, right)

	if !a.connected {
		layout = overlayCenter(layout, a.renderDisconnectedBox())
	}
	return layout
}

func (a *App) renderDisconnectedBox() string {
	scheme := "redis://"
	if a.cfg.TLS {
		scheme = "rediss://"
	}
	addr := scheme + a.cfg.Addr()

	lines := []string{
		styleError.Render("  ✗  Cannot connect to Redis  "),
		"",
		styleInfo.Render("  Host:  ") + styleWarning.Render(addr),
		"",
		styleMuted.Render("  Retrying automatically every tick…"),
		"",
		styleHintKey.Render("S") + "  " + styleHintDesc.Render("change connection settings"),
		styleHintKey.Render("p") + "  " + styleHintDesc.Render("switch profile"),
		styleHintKey.Render("q") + "  " + styleHintDesc.Render("quit"),
	}

	content := strings.Join(lines, "\n")
	w := a.width * 50 / 100
	if w < 52 {
		w = 52
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorRed).
		Padding(1, 2).
		Width(w).
		Render(content)
}

func (a *App) renderStatusBar() string {
	var content string
	if a.statusErr {
		content = styleStatusError.Render(" ✗ " + a.statusText)
	} else if a.statusText != "" {
		content = styleStatusSuccess.Render(" ✓ " + a.statusText)
	} else {
		hints := [][]string{
			{"j/k", "navigate"}, {"/", "search"}, {"n", "new"},
			{"d", "delete"}, {"e", "edit"}, {"a", "add"},
			{"D", "del item"}, {"R", "rename"}, {"t", "TTL"},
			{"y", "copy key"}, {"Y", "copy val"}, {":", "cmd"},
			{"J/K", "range sel"}, {"ctrl+space", "select"},
			{"enter", "enter dir"}, {"←", "up dir"},
			{"[/]", "DB"}, {"p", "profile"}, {"S", "connect"}, {"q", "quit"},
		}
		var parts []string
		for _, h := range hints {
			parts = append(parts, styleHintKey.Render(h[0])+" "+styleHintDesc.Render(h[1]))
		}
		content = strings.Join(parts, " ")
	}
	return lipgloss.NewStyle().
		Width(a.width).
		Background(colorSurface).
		Render(content)
}

func (a *App) renderHelp(height int) string {
	sections := []struct {
		title string
		rows  [][]string
	}{
		{"Navigation", [][]string{
			{"j / k", "move down / up in key list"},
			{"g / G", "jump to top / bottom"},
			{"ctrl+d / ctrl+u", "page down / up"},
			{"tab / l / h", "switch focus (keys ↔ value)"},
			{"J / K", "move sub-item cursor (list/hash/set/zset)"},
		}},
		{"Key Operations", [][]string{
			{"n", "new key (type selector)"},
			{"d", "delete key / folder / batch selection (confirm)"},
			{"R", "rename key"},
			{"t", "set / remove TTL"},
			{"y", "copy key name to clipboard"},
			{"Y / c", "copy value to clipboard"},
		}},
		{"Tree Navigation", [][]string{
			{"enter", "enter folder"},
			{"backspace", "go up one level"},
			{"esc", "go to root"},
		}},
		{"Multi-Select", [][]string{
			{"ctrl+space", "toggle selection on current item"},
			{"J / K", "extend selection range down / up"},
			{"d", "delete all selected items (batch)"},
		}},
		{"Value Editing", [][]string{
			{"e", "edit selected item (string / list item / hash field / zset member)"},
			{"a", "add item (list / hash / set / zset)"},
			{"D", "delete selected sub-item"},
		}},
		{"Filter & Search", [][]string{
			{"/", "open filter (fuzzy or Redis glob pattern)"},
			{"enter", "confirm filter"},
			{"esc", "clear filter"},
		}},
		{"Pub/Sub (tab 2)", [][]string{
			{"j / k", "navigate channel list"},
			{"tab / l / h", "switch focus (channels ↔ messages)"},
			{"s", "subscribe / unsubscribe selected channel"},
			{"P", "publish a message to a channel"},
			{"r", "refresh channel list"},
		}},
		{"Global", [][]string{
			{":", "run raw Redis command"},
			{"p", "switch connection profile"},
			{"S", "open connection settings (host / port / pass / db / TLS)"},
			{"[  ]", "switch database (db0-db15)"},
			{"r", "refresh keys + server info"},
			{"1 / 2 / 3 / 4", "tab: Keys / PubSub / Server / Help"},
			{"q / ctrl+c", "quit"},
		}},
	}

	var lines []string
	lines = append(lines, styleTitle.Render("LazyRedis — Keyboard Reference"), "")

	for _, sec := range sections {
		lines = append(lines, styleWarning.Render("  ── "+sec.title+" ──"))
		for _, r := range sec.rows {
			key := styleHintKey.Render(r[0])
			desc := styleInfo.Render(r[1])
			lines = append(lines, fmt.Sprintf("  %-28s  %s", key, desc))
		}
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")
	return styleBorder.Width(a.width - 2).Height(height - 2).Render(content)
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
