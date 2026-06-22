package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type modalKind int

const (
	modalNone modalKind = iota
	modalConfirm
	modalInput        // single text input
	modalInputDouble  // two text inputs (field + value)
	modalNewKey       // create key: name + type + value
	modalRename
	modalTTL
	modalCommand // raw command
	modalConnect // connection settings
)

type ModalResult struct {
	Confirmed bool
	Values    []string // inputs in order
}

type Modal struct {
	Kind    modalKind
	Title   string
	Prompt  string
	Warning string

	inputs  []textinput.Model
	focused int

	// for type picker (new key)
	typeOptions []string
	typeIdx     int

	// for connect modal
	tlsEnabled    bool
	tlsSkipVerify bool
	refreshIdx    int // index into refreshLabels

	onDone func(ModalResult) tea.Cmd
}

var refreshLabels = []string{"off", "1s", "2s", "5s", "10s", "30s"}

func newInput(placeholder string, limit int) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = limit
	return ti
}

func NewConfirmModal(title, warning string, onDone func(ModalResult) tea.Cmd) *Modal {
	return &Modal{
		Kind:    modalConfirm,
		Title:   title,
		Warning: warning,
		onDone:  onDone,
	}
}

func NewInputModal(title, prompt, placeholder string, onDone func(ModalResult) tea.Cmd) *Modal {
	in := newInput(placeholder, 512)
	in.Focus()
	return &Modal{
		Kind:   modalInput,
		Title:  title,
		Prompt: prompt,
		inputs: []textinput.Model{in},
		onDone: onDone,
	}
}

func NewDoubleInputModal(title, p1, p2 string, onDone func(ModalResult) tea.Cmd) *Modal {
	a := newInput(p1, 256)
	a.Focus()
	b := newInput(p2, 512)
	return &Modal{
		Kind:   modalInputDouble,
		Title:  title,
		inputs: []textinput.Model{a, b},
		onDone: onDone,
	}
}

func NewRenameModal(oldKey string, onDone func(ModalResult) tea.Cmd) *Modal {
	in := newInput("new key name", 512)
	in.SetValue(oldKey)
	in.Focus()
	return &Modal{
		Kind:   modalRename,
		Title:  "Rename Key",
		Prompt: fmt.Sprintf("Rename '%s' to:", oldKey),
		inputs: []textinput.Model{in},
		onDone: onDone,
	}
}

func NewTTLModal(key string, onDone func(ModalResult) tea.Cmd) *Modal {
	in := newInput("seconds (0 = remove TTL)", 20)
	in.Focus()
	return &Modal{
		Kind:   modalTTL,
		Title:  "Set TTL",
		Prompt: fmt.Sprintf("Expire '%s' in (seconds):", key),
		inputs: []textinput.Model{in},
		onDone: onDone,
	}
}

func NewCommandModal(onDone func(ModalResult) tea.Cmd) *Modal {
	in := newInput("e.g. SET foo bar / GET foo / HGETALL myhash", 512)
	in.Focus()
	return &Modal{
		Kind:   modalCommand,
		Title:  "Run Command",
		Prompt: "Redis command:",
		inputs: []textinput.Model{in},
		onDone: onDone,
	}
}

// NewConnectModal opens the connection-settings dialog pre-filled with the
// current connection values.  focused cycles: host→port→pass→db→TLS→(skipVerify)→host.
// ModalResult.Values: [host, port, password, db, tls "true"/"false", skipVerify "true"/"false"]
func NewConnectModal(host string, port int, password string, db int, tlsOn, skipVerify bool, refreshIdx int, onDone func(ModalResult) tea.Cmd) *Modal {
	hostIn := newInput("hostname or IP", 256)
	hostIn.SetValue(host)
	hostIn.Focus()

	portIn := newInput("6379", 6)
	portIn.SetValue(fmt.Sprintf("%d", port))

	passIn := newInput("(blank = no auth)", 256)
	passIn.SetValue(password)
	passIn.EchoMode = textinput.EchoPassword

	dbIn := newInput("0–15", 2)
	dbIn.SetValue(fmt.Sprintf("%d", db))

	if refreshIdx < 0 || refreshIdx >= len(refreshLabels) {
		refreshIdx = 0
	}
	return &Modal{
		Kind:          modalConnect,
		Title:         "Connect to Redis",
		inputs:        []textinput.Model{hostIn, portIn, passIn, dbIn},
		focused:       0,
		tlsEnabled:    tlsOn,
		tlsSkipVerify: skipVerify,
		refreshIdx:    refreshIdx,
		onDone:        onDone,
	}
}

var keyTypeChoices = []string{"string", "list", "hash", "set", "zset"}

func NewNewKeyModal(onDone func(ModalResult) tea.Cmd) *Modal {
	name := newInput("key name", 512)
	name.Focus()
	val := newInput("initial value", 512)
	return &Modal{
		Kind:        modalNewKey,
		Title:       "New Key",
		inputs:      []textinput.Model{name, val},
		typeOptions: keyTypeChoices,
		typeIdx:     0,
		onDone:      onDone,
	}
}

func (m *Modal) Update(msg tea.KeyMsg) (*Modal, tea.Cmd) {
	switch m.Kind {
	case modalConfirm:
		switch msg.String() {
		case "y", "Y":
			return nil, m.onDone(ModalResult{Confirmed: true})
		case "n", "N", "esc", "q":
			return nil, m.onDone(ModalResult{Confirmed: false})
		}
		return m, nil

	case modalNewKey:
		switch msg.String() {
		case "esc":
			return nil, m.onDone(ModalResult{Confirmed: false})
		case "tab":
			m.inputs[m.focused].Blur()
			switch m.focused {
			case 0:
				m.focused = 1 // go to type selector
			case 1:
				m.focused = 2 // go to value input
				m.inputs[1].Focus()
			default:
				m.focused = 0
				m.inputs[0].Focus()
				m.inputs[1].Blur()
			}
			return m, textinput.Blink
		case "left", "h":
			if m.focused == 1 {
				if m.typeIdx > 0 {
					m.typeIdx--
				}
			}
		case "right", "l":
			if m.focused == 1 {
				if m.typeIdx < len(m.typeOptions)-1 {
					m.typeIdx++
				}
			}
		case "enter":
			name := strings.TrimSpace(m.inputs[0].Value())
			val := strings.TrimSpace(m.inputs[1].Value())
			if name == "" {
				return m, nil
			}
			return nil, m.onDone(ModalResult{
				Confirmed: true,
				Values:    []string{name, m.typeOptions[m.typeIdx], val},
			})
		default:
			var cmd tea.Cmd
			if m.focused == 0 {
				m.inputs[0], cmd = m.inputs[0].Update(msg)
			} else if m.focused == 2 {
				m.inputs[1], cmd = m.inputs[1].Update(msg)
			}
			return m, cmd
		}
		return m, nil

	case modalConnect:
		// focused 0-3 = text inputs (host/port/pass/db)
		// focused 4   = TLS toggle       (space)
		// focused 5   = Skip Verify      (space, only when tlsEnabled)
		// focused 6   = Refresh interval (←/→ or space)
		// Tab cycles 0→1→2→3→4→(5 if tls)→6→0
		switch msg.String() {
		case "esc":
			return nil, m.onDone(ModalResult{Confirmed: false})
		case "tab":
			if m.focused < 4 {
				m.inputs[m.focused].Blur()
			}
			next := (m.focused + 1) % 7
			if next == 5 && !m.tlsEnabled {
				next = 6 // skip Skip Verify when TLS is off
			}
			m.focused = next
			if m.focused < 4 {
				m.inputs[m.focused].Focus()
				return m, textinput.Blink
			}
			return m, nil
		case " ":
			switch m.focused {
			case 4:
				m.tlsEnabled = !m.tlsEnabled
				if !m.tlsEnabled {
					m.tlsSkipVerify = false
				}
			case 5:
				if m.tlsEnabled {
					m.tlsSkipVerify = !m.tlsSkipVerify
				}
			case 6:
				m.refreshIdx = (m.refreshIdx + 1) % len(refreshLabels)
			default:
				var cmd tea.Cmd
				m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
				return m, cmd
			}
			return m, nil
		case "left":
			if m.focused == 6 && m.refreshIdx > 0 {
				m.refreshIdx--
			}
			return m, nil
		case "right":
			if m.focused == 6 && m.refreshIdx < len(refreshLabels)-1 {
				m.refreshIdx++
			}
			return m, nil
		case "enter":
			vals := []string{
				m.inputs[0].Value(),
				m.inputs[1].Value(),
				m.inputs[2].Value(),
				m.inputs[3].Value(),
				fmt.Sprintf("%v", m.tlsEnabled),
				fmt.Sprintf("%v", m.tlsSkipVerify),
				refreshLabels[m.refreshIdx], // e.g. "off", "2s", "5s"
			}
			return nil, m.onDone(ModalResult{Confirmed: true, Values: vals})
		default:
			if m.focused < 4 {
				var cmd tea.Cmd
				m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
				return m, cmd
			}
		}
		return m, nil

	default: // input-based modals
		switch msg.String() {
		case "esc":
			return nil, m.onDone(ModalResult{Confirmed: false})
		case "tab":
			if len(m.inputs) > 1 {
				m.inputs[m.focused].Blur()
				m.focused = (m.focused + 1) % len(m.inputs)
				m.inputs[m.focused].Focus()
				return m, textinput.Blink
			}
		case "enter":
			if m.Kind == modalInputDouble && m.focused == 0 {
				m.inputs[0].Blur()
				m.focused = 1
				m.inputs[1].Focus()
				return m, textinput.Blink
			}
			vals := make([]string, len(m.inputs))
			for i, in := range m.inputs {
				vals[i] = in.Value()
			}
			return nil, m.onDone(ModalResult{Confirmed: true, Values: vals})
		default:
			var cmd tea.Cmd
			m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m *Modal) View(width int) string {
	w := width * 55 / 100
	if w < 50 {
		w = 50
	}

	var lines []string
	lines = append(lines, stylePanelTitle.Render(m.Title))
	lines = append(lines, "")

	if m.Warning != "" {
		lines = append(lines, styleWarning.Render("⚠  "+m.Warning))
		lines = append(lines, "")
	}

	switch m.Kind {
	case modalConnect:
		inputLabels := []string{"Host:", "Port:", "Password:", "DB:"}
		for i, label := range inputLabels {
			if m.focused == i {
				lines = append(lines, stylePanelTitle.Render(label))
			} else {
				lines = append(lines, styleInfo.Render(label))
			}
			lines = append(lines, m.inputs[i].View())
			lines = append(lines, "")
		}

		// TLS toggle
		tlsBadge := styleMuted.Render(" off ")
		if m.tlsEnabled {
			tlsBadge = styleSuccess.Render(" on ")
		}
		tlsLabel := styleInfo.Render("TLS")
		if m.focused == 4 {
			tlsLabel = styleSelected.Render(" TLS ")
		}
		lines = append(lines, tlsLabel+"   "+tlsBadge+"  "+styleMuted.Render("(space to toggle)"))

		// Skip Verify — only visible when TLS is on
		if m.tlsEnabled {
			skipBadge := styleSuccess.Render(" no ")
			if m.tlsSkipVerify {
				skipBadge = styleWarning.Render(" yes (insecure) ")
			}
			skipLabel := styleInfo.Render("Skip Verify")
			if m.focused == 5 {
				skipLabel = styleSelected.Render(" Skip Verify ")
			}
			lines = append(lines, skipLabel+"   "+skipBadge)
		}

		// Refresh interval
		refreshLabel := styleInfo.Render("Auto-refresh")
		if m.focused == 6 {
			refreshLabel = styleSelected.Render(" Auto-refresh ")
		}
		refreshBadge := styleMuted.Render(" off ")
		if m.refreshIdx > 0 {
			refreshBadge = styleWarning.Render(" " + refreshLabels[m.refreshIdx] + " ")
		}
		lines = append(lines, refreshLabel+"   "+refreshBadge+"  "+styleMuted.Render("(←/→ or space)"))

		lines = append(lines, "")
		w = width * 60 / 100
		if w < 55 {
			w = 55
		}
		lines = append(lines,
			styleHintKey.Render("tab")+" "+styleHintDesc.Render("next")+"  "+
				styleHintKey.Render("space")+" "+styleHintDesc.Render("toggle")+"  "+
				styleHintKey.Render("←/→")+" "+styleHintDesc.Render("interval")+"  "+
				styleHintKey.Render("enter")+" "+styleHintDesc.Render("apply")+"  "+
				styleHintKey.Render("esc")+" "+styleHintDesc.Render("cancel"))

	case modalConfirm:
		lines = append(lines, styleInfo.Render(m.Prompt))
		lines = append(lines, "")
		lines = append(lines, styleHintKey.Render("y")+" "+styleHintDesc.Render("confirm")+"  "+styleHintKey.Render("n")+" "+styleHintDesc.Render("cancel"))

	case modalNewKey:
		lines = append(lines, styleInfo.Render("Key name:"))
		lines = append(lines, m.inputs[0].View())
		lines = append(lines, "")

		lines = append(lines, styleInfo.Render("Type:"))
		var typeParts []string
		for i, t := range m.typeOptions {
			if m.focused == 1 && i == m.typeIdx {
				typeParts = append(typeParts, keyTypeBadge(t))
			} else {
				typeParts = append(typeParts, styleMuted.Render("["+t+"]"))
			}
		}
		lines = append(lines, strings.Join(typeParts, " "))
		lines = append(lines, "")

		lines = append(lines, styleInfo.Render("Initial value:"))
		lines = append(lines, m.inputs[1].View())
		lines = append(lines, "")

		focused := []string{"key name", "type (←/→)", "value"}
		hint := styleMuted.Render(fmt.Sprintf("tab to switch fields  (now: %s)", focused[min(m.focused, 2)]))
		lines = append(lines, hint)
		lines = append(lines, styleHintKey.Render("enter")+" "+styleHintDesc.Render("create")+"  "+styleHintKey.Render("esc")+" "+styleHintDesc.Render("cancel"))

	default:
		if m.Prompt != "" {
			lines = append(lines, styleInfo.Render(m.Prompt))
		}
		for _, in := range m.inputs {
			lines = append(lines, in.View())
		}
		lines = append(lines, "")
		lines = append(lines, styleHintKey.Render("enter")+" "+styleHintDesc.Render("confirm")+"  "+styleHintKey.Render("esc")+" "+styleHintDesc.Render("cancel"))
	}

	content := strings.Join(lines, "\n")
	return styleModalBorder.Width(w).Render(content)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
