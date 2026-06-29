package ui

import (
	"fmt"
	"strings"

	redisclient "github.com/sm010422/lazyredis/pkg/redis"
)

type ServerPanel struct {
	Info    *redisclient.ServerInfo
	RawInfo string
	showRaw bool
}

func (p *ServerPanel) ToggleRaw() { p.showRaw = !p.showRaw }

func (p *ServerPanel) Render(width, height int) string {
	innerW := width - 4
	innerH := height - 4

	if p.showRaw {
		return p.renderRaw(width, height, innerW, innerH)
	}
	return p.renderFormatted(width, height, innerW, innerH)
}

func (p *ServerPanel) renderFormatted(width, height, innerW, _ int) string {
	title := stylePanelTitle.Render("Server Info") + "  " + styleMuted.Render("[r] raw")

	if p.Info == nil {
		content := title + "\n" + styleMuted.Render("loading…")
		return styleBorder.Width(width - 2).Height(height - 2).Render(content)
	}

	si := p.Info

	hitRatio := "N/A"
	total := si.KeyspaceHits + si.KeyspaceMisses
	if total > 0 {
		hitRatio = fmt.Sprintf("%.1f%%", float64(si.KeyspaceHits)*100/float64(total))
	}

	sections := []struct {
		title string
		rows  [][2]string
	}{
		{"Server", [][2]string{
			{"Version", si.Version},
			{"Mode", si.Mode},
			{"Role", si.Role},
			{"OS", si.OS},
			{"Arch", si.Arch},
			{"Uptime", redisclient.FormatUptime(si.UptimeSecs)},
		}},
		{"Clients & Memory", [][2]string{
			{"Connected", fmt.Sprintf("%d clients", si.ConnectedClients)},
			{"Used Memory", si.UsedMemory},
		}},
		{"Stats", [][2]string{
			{"Total Commands", fmt.Sprintf("%d", si.TotalCommands)},
			{"Cache Hits", fmt.Sprintf("%d", si.KeyspaceHits)},
			{"Cache Misses", fmt.Sprintf("%d", si.KeyspaceMisses)},
			{"Hit Ratio", hitRatio},
		}},
		{"Pub/Sub", [][2]string{
			{"Channels", fmt.Sprintf("%d active", si.PubSubChannels)},
			{"Patterns", fmt.Sprintf("%d subscribed", si.PubSubPatterns)},
		}},
	}

	var lines []string
	lines = append(lines, title, "")

	for _, sec := range sections {
		lines = append(lines, styleWarning.Render("── "+sec.title+" ──"))
		for _, r := range sec.rows {
			if r[1] == "" {
				continue
			}
			label := styleInfo.Render(fmt.Sprintf("  %-18s", r[0]+":"))
			val := styleBold.Render(r[1])
			line := label + val
			if len(line) > innerW {
				line = line[:innerW]
			}
			lines = append(lines, line)
		}
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")
	return styleBorder.Width(width - 2).Height(height - 2).Render(content)
}

func (p *ServerPanel) renderRaw(width, height, innerW, innerH int) string {
	title := stylePanelTitle.Render("Server Info (raw)") + "  " + styleMuted.Render("[r] formatted")

	lines := strings.Split(p.RawInfo, "\n")
	var visible []string
	visible = append(visible, title, "")

	for i, l := range lines {
		if i >= innerH {
			break
		}
		if strings.HasPrefix(l, "#") {
			visible = append(visible, styleWarning.Render(l))
		} else if strings.Contains(l, ":") {
			parts := strings.SplitN(l, ":", 2)
			line := styleInfo.Render(parts[0]+":") + styleBold.Render(parts[1])
			if len(line) > innerW {
				line = line[:innerW]
			}
			visible = append(visible, line)
		} else {
			visible = append(visible, l)
		}
	}

	content := strings.Join(visible, "\n")
	return styleBorder.Width(width - 2).Height(height - 2).Render(content)
}
