package ui

import (
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/charmbracelet/lipgloss"
	redisclient "github.com/sm010422/lazyredis/pkg/redis"
)

type pubSubMessage struct {
	Channel string
	Payload string
	At      time.Time
}

type PubSubPanel struct {
	Stats      *redisclient.PubSubStats
	cursor     int
	focusLeft  bool

	Sub        *goredis.PubSub
	SubChannel string
	Messages   []pubSubMessage
	msgScroll  int // -1 = pinned to bottom, else index of first visible message
}

func newPubSubPanel() PubSubPanel {
	return PubSubPanel{focusLeft: true, msgScroll: -1}
}

func (p *PubSubPanel) channels() []string {
	if p.Stats == nil {
		return nil
	}
	return p.Stats.Channels
}

func (p *PubSubPanel) MoveDown() {
	chs := p.channels()
	if p.cursor < len(chs)-1 {
		p.cursor++
	}
}

func (p *PubSubPanel) MoveUp() {
	if p.cursor > 0 {
		p.cursor--
	}
}

func (p *PubSubPanel) Selected() string {
	chs := p.channels()
	if len(chs) == 0 || p.cursor >= len(chs) {
		return ""
	}
	return chs[p.cursor]
}

func (p *PubSubPanel) AddMessage(ch, payload string) {
	p.Messages = append(p.Messages, pubSubMessage{Channel: ch, Payload: payload, At: time.Now()})
	if len(p.Messages) > 500 {
		p.Messages = p.Messages[len(p.Messages)-500:]
	}
	// keep pinned to bottom unless user has manually scrolled up
	if p.msgScroll == -1 {
		// stays pinned
	}
}

func (p *PubSubPanel) ScrollMsgUp() {
	visibleLines := 10 // approx; recalculated in Render
	if p.msgScroll == -1 {
		// unpin and start from near the end
		if len(p.Messages) > visibleLines {
			p.msgScroll = len(p.Messages) - visibleLines
		} else {
			p.msgScroll = 0
		}
	} else if p.msgScroll > 0 {
		p.msgScroll--
	}
}

func (p *PubSubPanel) ScrollMsgDown() {
	if p.msgScroll == -1 {
		return
	}
	p.msgScroll++
	if p.msgScroll >= len(p.Messages) {
		p.msgScroll = -1 // re-pin to bottom
	}
}

func (p *PubSubPanel) Render(width, height int) string {
	leftW := width * 35 / 100
	rightW := width - leftW
	left := p.renderChannelList(leftW, height)
	right := p.renderMessageLog(rightW, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (p *PubSubPanel) renderChannelList(width, height int) string {
	patStr := ""
	if p.Stats != nil && p.Stats.NumPatterns > 0 {
		patStr = "  " + styleMuted.Render(fmt.Sprintf("%d patterns", p.Stats.NumPatterns))
	}
	title := stylePanelTitle.Render("Channels") + patStr

	var lines []string
	lines = append(lines, title, "")

	chs := p.channels()
	if len(chs) == 0 {
		lines = append(lines, styleMuted.Render("  No active channels"))
		lines = append(lines, "")
		lines = append(lines, styleInfo.Render("  Publish a message to a channel"))
		lines = append(lines, styleInfo.Render("  or wait for clients to subscribe."))
	} else {
		innerW := width - 6
		for i, ch := range chs {
			subs := int64(0)
			if p.Stats != nil {
				subs = p.Stats.ChannelSubs[ch]
			}
			subStr := styleMuted.Render(fmt.Sprintf(" %d▾", subs))

			label := ch
			maxLabel := innerW - 6
			if maxLabel < 4 {
				maxLabel = 4
			}
			if len(label) > maxLabel {
				label = label[:maxLabel-1] + "…"
			}

			subscribed := ch == p.SubChannel

			var line string
			if i == p.cursor && p.focusLeft {
				line = styleSelected.Render(" "+label+" ") + subStr
			} else if i == p.cursor {
				line = styleBold.Render(" "+label) + subStr
			} else if subscribed {
				line = styleSuccess.Render(" ● "+label) + subStr
			} else {
				line = styleInfo.Render("   "+label) + subStr
			}
			lines = append(lines, line)
		}
	}

	content := strings.Join(lines, "\n")
	border := styleBorder
	if p.focusLeft {
		border = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorderActive)
	}
	return border.Width(width - 2).Height(height - 2).Render(content)
}

func (p *PubSubPanel) renderMessageLog(width, height int) string {
	innerH := height - 4

	subLabel := styleMuted.Render(" not subscribed")
	if p.SubChannel != "" {
		subLabel = styleSuccess.Render(" ● ") + styleBold.Render(p.SubChannel)
	}
	scrollHint := ""
	if p.msgScroll != -1 {
		scrollHint = "  " + styleWarning.Render("↑ scrolled")
	}
	title := stylePanelTitle.Render("Live Messages") + subLabel + scrollHint

	var msgLines []string
	for _, msg := range p.Messages {
		ts := msg.At.Format("15:04:05")
		chTag := styleWarning.Render("[" + msg.Channel + "]")
		timeStr := styleMuted.Render(ts + " ")
		payload := msg.Payload
		maxPay := width - 22
		if maxPay < 4 {
			maxPay = 4
		}
		if len(payload) > maxPay {
			payload = payload[:maxPay-1] + "…"
		}
		msgLines = append(msgLines, timeStr+chTag+" "+payload)
	}

	var visible []string
	if len(msgLines) == 0 {
		if p.SubChannel != "" {
			visible = append(visible, styleMuted.Render("  Waiting for messages…"))
		} else {
			visible = append(visible, styleMuted.Render("  Select a channel and press s to subscribe."))
			visible = append(visible, styleMuted.Render("  Press P to publish a message to any channel."))
		}
	} else {
		start := 0
		if p.msgScroll >= 0 {
			start = p.msgScroll
			if start > len(msgLines) {
				start = len(msgLines)
			}
		} else if len(msgLines) > innerH {
			start = len(msgLines) - innerH
		}
		end := start + innerH
		if end > len(msgLines) {
			end = len(msgLines)
		}
		visible = msgLines[start:end]
	}

	body := title + "\n\n" + strings.Join(visible, "\n")
	border := styleBorder
	if !p.focusLeft {
		border = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorderActive)
	}
	return border.Width(width - 2).Height(height - 2).Render(body)
}
