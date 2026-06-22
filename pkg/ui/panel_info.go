package ui

import (
	"fmt"
	"strings"

	redisclient "github.com/parksangmin/lazyredis/pkg/redis"
)

// InfoPanel shows key metadata (type, TTL, memory, size).
type InfoPanel struct{}

func (p InfoPanel) Render(width, height int, info *redisclient.KeyInfo, cmdLog []string) string {
	innerW := width - 4

	var lines []string

	if info != nil {
		sizeLabel := fmt.Sprintf("%d elements", info.Size)
		if info.Type == redisclient.TypeString {
			sizeLabel = fmt.Sprintf("%d bytes", info.Size)
		}
		lines = append(lines,
			row("Key", styleBold.Render(info.Name), innerW),
			row("Type", keyTypeBadge(string(info.Type)), innerW),
			row("TTL", styleTTL.Render(redisclient.FormatTTL(info.TTL)), innerW),
			row("Size", sizeLabel, innerW),
		)
		if info.Memory > 0 {
			lines = append(lines, row("Memory", redisclient.FormatSize(info.Memory), innerW))
		}
		lines = append(lines, "")
	}

	// Command log
	if len(cmdLog) > 0 {
		lines = append(lines, styleMuted.Render("─── Command Log ─────────────────"))
		logH := height - len(lines) - 4
		if logH < 1 {
			logH = 1
		}
		start := 0
		if len(cmdLog) > logH {
			start = len(cmdLog) - logH
		}
		for _, l := range cmdLog[start:] {
			if len(l) > innerW {
				l = l[:innerW-1] + "…"
			}
			lines = append(lines, styleInfo.Render(l))
		}
	}

	content := stylePanelTitle.Render("Info") + "\n" + strings.Join(lines, "\n")
	return styleBorder.Width(width - 2).Height(height - 2).Render(content)
}

func row(label, value string, _ int) string {
	return styleInfo.Render(fmt.Sprintf("%-8s", label+":")) + " " + value
}
