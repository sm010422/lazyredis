package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	redisclient "github.com/sm010422/lazyredis/pkg/redis"
)

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.modal != nil {
		newModal, cmd := a.modal.Update(msg)
		a.modal = newModal
		return a, cmd
	}

	if a.keys.filtering {
		return a.handleFilterKey(msg)
	}

	key := msg.String()

	switch key {
	case "ctrl+c", "q":
		if a.pubsub.Sub != nil {
			a.pubsub.Sub.Close()
			a.pubsub.Sub = nil
		}
		return a, tea.Quit
	case "1":
		a.tab = tabKeys
		return a, nil
	case "2":
		a.tab = tabPubSub
		return a, a.loadPubSubStats()
	case "3":
		a.tab = tabServer
		return a, a.loadServerInfo()
	case "4":
		a.tab = tabHelp
		return a, nil
	}

	switch a.tab {
	case tabKeys:
		return a.handleKeysTab(key)
	case tabPubSub:
		return a.handlePubSubTab(key)
	case tabServer:
		return a.handleServerTab(key)
	case tabHelp:
		if key == "esc" || key == "q" {
			a.tab = tabKeys
		}
		return a, nil
	}

	return a, nil
}

func (a *App) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		val := a.keys.filterInput.Value()
		a.keys.StopFilter(false)
		if strings.ContainsAny(val, "*?[]") {
			return a, a.loadKeysPattern(val)
		}
		a.keys.UpdateFilter(val)
		if sel := a.keys.Selected(); sel != "" {
			return a, a.selectKey(sel)
		}
		return a, nil

	case "esc":
		a.keys.StopFilter(true)
		return a, nil

	default:
		var cmd tea.Cmd
		a.keys.filterInput, cmd = a.keys.filterInput.Update(msg)
		a.keys.UpdateFilter(a.keys.filterInput.Value())
		if sel := a.keys.Selected(); sel != "" {
			return a, tea.Batch(cmd, a.selectKey(sel))
		}
		return a, cmd
	}
}

func (a *App) handleKeysTab(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "tab", "right", "l":
		if a.focus == focusKeyList {
			a.focus = focusValue
		} else {
			a.focus = focusKeyList
		}
		return a, nil

	case "shift+tab", "left", "h":
		a.focus = focusKeyList
		return a, nil

	case "j", "down":
		if a.focus == focusKeyList {
			if a.keys.MoveDown() {
				if sel := a.keys.SelectedKey(); sel != "" {
					return a, a.selectKey(sel)
				}
			}
		} else {
			a.value.ScrollDown()
		}
		return a, nil

	case "k", "up":
		if a.focus == focusKeyList {
			if a.keys.MoveUp() {
				if sel := a.keys.SelectedKey(); sel != "" {
					return a, a.selectKey(sel)
				}
			}
		} else {
			a.value.ScrollUp()
		}
		return a, nil

	case "g":
		if a.focus == focusKeyList {
			a.keys.MoveTop()
			if sel := a.keys.Selected(); sel != "" {
				return a, a.selectKey(sel)
			}
		} else {
			a.value.ScrollTop()
		}
		return a, nil

	case "G":
		if a.focus == focusKeyList {
			a.keys.MoveBottom()
			if sel := a.keys.Selected(); sel != "" {
				return a, a.selectKey(sel)
			}
		}
		return a, nil

	case "ctrl+d":
		a.keys.PageDown(10)
		if sel := a.keys.Selected(); sel != "" {
			return a, a.selectKey(sel)
		}
		return a, nil

	case "ctrl+u":
		a.keys.PageUp(10)
		if sel := a.keys.Selected(); sel != "" {
			return a, a.selectKey(sel)
		}
		return a, nil

	case "J":
		if a.focus == focusValue {
			a.value.SubDown(a.subItemCount())
		} else if a.focus == focusKeyList {
			a.keys.ExtendSelectDown()
			if sel := a.keys.SelectedKey(); sel != "" {
				return a, a.selectKey(sel)
			}
		}
		return a, nil

	case "K":
		if a.focus == focusValue {
			a.value.SubUp()
		} else if a.focus == focusKeyList {
			a.keys.ExtendSelectUp()
			if sel := a.keys.SelectedKey(); sel != "" {
				return a, a.selectKey(sel)
			}
		}
		return a, nil

	case "enter":
		if a.focus == focusKeyList && !a.keys.filtering {
			node := a.keys.SelectedNode()
			if node != nil && node.kind == nodeDir {
				a.keys.EnterDir()
				return a, nil
			}
		}
		return a, nil

	case "backspace":
		if a.focus == focusKeyList && !a.keys.filtering {
			a.keys.ExitDir()
			if sel := a.keys.SelectedKey(); sel != "" {
				return a, a.selectKey(sel)
			}
			return a, nil
		}
		return a, nil

	case "esc":
		if a.focus == focusKeyList && !a.keys.filtering {
			a.keys.GoRoot()
			if sel := a.keys.SelectedKey(); sel != "" {
				return a, a.selectKey(sel)
			}
			return a, nil
		}
		return a, nil

	case "ctrl+ ", "ctrl+space", "ctrl+@":
		a.keys.ToggleSelect()
		return a, nil

	case "/":
		a.keys.StartFilter()
		return a, textinput.Blink

	case "r":
		statusCmd := a.setStatus("Refreshing…", false)
		return a, tea.Batch(statusCmd, a.loadKeys(), a.loadServerInfo())

	case "n":
		a.modal = NewNewKeyModal(func(r ModalResult) tea.Cmd {
			if !r.Confirmed || len(r.Values) < 3 {
				return nil
			}
			return a.createKey(r.Values[0], r.Values[1], r.Values[2])
		})
		return a, textinput.Blink

	case "d":
		if a.keys.HasSelection() {
			keys := a.keys.SelectedLeafKeys()
			msg := fmt.Sprintf("Delete %d selected keys? This cannot be undone.", len(keys))
			a.modal = NewConfirmModal("Batch Delete", msg, func(r ModalResult) tea.Cmd {
				if !r.Confirmed {
					return nil
				}
				a.keys.ClearSelection()
				return a.batchDeleteKeys(keys)
			})
			return a, nil
		}
		if node := a.keys.SelectedNode(); node != nil && node.kind == nodeDir {
			prefix := node.prefix
			count := node.count
			msg := fmt.Sprintf("Delete all %d keys under '%s'? This cannot be undone.", count, strings.TrimSuffix(prefix, ":"))
			a.modal = NewConfirmModal("Delete Folder", msg, func(r ModalResult) tea.Cmd {
				if !r.Confirmed {
					return nil
				}
				keys := keysWithPrefix(a.keys.allKeys, prefix)
				return a.batchDeleteKeys(keys)
			})
			return a, nil
		}
		sel := a.keys.SelectedKey()
		if sel == "" {
			return a, nil
		}
		a.modal = NewConfirmModal(
			"Delete Key",
			fmt.Sprintf("Delete '%s'? This cannot be undone.", sel),
			func(r ModalResult) tea.Cmd {
				if !r.Confirmed {
					return nil
				}
				return a.deleteKey(sel)
			},
		)
		return a, nil

	case "R":
		sel := a.keys.Selected()
		if sel == "" {
			return a, nil
		}
		a.modal = NewRenameModal(sel, func(r ModalResult) tea.Cmd {
			if !r.Confirmed || len(r.Values) == 0 {
				return nil
			}
			return a.renameKey(sel, r.Values[0])
		})
		return a, textinput.Blink

	case "t":
		sel := a.keys.Selected()
		if sel == "" {
			return a, nil
		}
		a.modal = NewTTLModal(sel, func(r ModalResult) tea.Cmd {
			if !r.Confirmed || len(r.Values) == 0 {
				return nil
			}
			return a.setTTL(sel, r.Values[0])
		})
		return a, textinput.Blink

	case "e":
		return a, a.startEdit()

	case "a":
		return a, a.startAdd()

	case "D":
		return a, a.startDeleteSubItem()

	case "y":
		return a.copyKeyName()

	case "c", "Y":
		return a.copyValue()

	case ":":
		a.modal = NewCommandModal(func(r ModalResult) tea.Cmd {
			if !r.Confirmed || len(r.Values) == 0 {
				return nil
			}
			return a.runCommand(r.Values[0])
		})
		return a, textinput.Blink

	case "[":
		return a, a.switchDB(a.currentDB - 1)

	case "]":
		return a, a.switchDB(a.currentDB + 1)

	case "?":
		a.tab = tabHelp
		return a, nil

	case "S":
		return a, a.openConnectModal()

	case "p":
		return a, a.openProfileModal()
	}

	return a, nil
}

func (a *App) handleServerTab(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "r":
		a.server.ToggleRaw()
	case "R":
		return a, a.loadServerInfo()
	case "S":
		return a, a.openConnectModal()
	case "p":
		return a, a.openProfileModal()
	}
	return a, nil
}

func (a *App) handlePubSubTab(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "tab", "l", "h":
		a.pubsub.focusLeft = !a.pubsub.focusLeft
		return a, nil

	case "j", "down":
		if a.pubsub.focusLeft {
			a.pubsub.MoveDown()
		} else {
			a.pubsub.ScrollMsgDown()
		}
		return a, nil

	case "k", "up":
		if a.pubsub.focusLeft {
			a.pubsub.MoveUp()
		} else {
			a.pubsub.ScrollMsgUp()
		}
		return a, nil

	case "r":
		return a, a.loadPubSubStats()

	case "s":
		sel := a.pubsub.Selected()
		if sel == "" {
			return a, a.setStatus("No channel selected", true)
		}
		if a.pubsub.SubChannel == sel {
			if a.pubsub.Sub != nil {
				a.pubsub.Sub.Close()
				a.pubsub.Sub = nil
			}
			a.pubsub.SubChannel = ""
			return a, a.setStatus("Unsubscribed from "+sel, false)
		}
		if a.pubsub.Sub != nil {
			a.pubsub.Sub.Close()
		}
		sub := a.redis.Subscribe(sel)
		a.pubsub.Sub = sub
		a.pubsub.SubChannel = sel
		a.pubsub.Messages = nil
		a.pubsub.msgScroll = -1
		return a, tea.Batch(a.setStatus("Subscribed to "+sel, false), a.waitPubSubMsg())

	case "P":
		sel := a.pubsub.Selected()
		a.modal = NewDoubleInputModal("Publish Message", "channel", "message", func(r ModalResult) tea.Cmd {
			if !r.Confirmed || len(r.Values) < 2 {
				return nil
			}
			ch, msg := strings.TrimSpace(r.Values[0]), r.Values[1]
			if ch == "" {
				return func() tea.Msg { return msgStatus{text: "Channel name is required", isError: true} }
			}
			return func() tea.Msg {
				if err := a.redis.Publish(ch, msg); err != nil {
					return msgStatus{text: "Publish error: " + err.Error(), isError: true}
				}
				return msgStatus{text: fmt.Sprintf("Published to '%s'", ch), isError: false}
			}
		})
		if sel != "" {
			a.modal.inputs[0].SetValue(sel)
			a.modal.inputs[1].Focus()
			a.modal.inputs[0].Blur()
		}
		return a, textinput.Blink

	case "S":
		return a, a.openConnectModal()

	case "p":
		return a, a.openProfileModal()
	}
	return a, nil
}

func (a *App) openProfileModal() tea.Cmd {
	if len(a.profiles) == 0 {
		return func() tea.Msg {
			return msgStatus{text: "No profiles found. Edit ~/.config/lazyredis/config.json", isError: true}
		}
	}
	names := make([]string, len(a.profiles))
	colors := make([]string, len(a.profiles))
	for i, p := range a.profiles {
		names[i] = p.Name
		colors[i] = p.Color
	}
	a.modal = NewProfileModal(names, colors, a.activeProfileIdx, func(r ModalResult) tea.Cmd {
		if !r.Confirmed || len(r.Values) == 0 {
			return nil
		}
		idx, _ := strconv.Atoi(r.Values[0])
		if idx < 0 || idx >= len(a.profiles) {
			return nil
		}
		p := a.profiles[idx]
		return func() tea.Msg {
			newCfg := p.ToConfig()
			client, err := redisclient.New(newCfg)
			if err != nil {
				return msgStatus{text: "Connection error: " + err.Error(), isError: true}
			}
			if err := client.Ping(); err != nil {
				client.Close()
				return msgStatus{text: "Connection failed: " + err.Error(), isError: true}
			}
			return msgConnected{client: client, cfg: newCfg, interval: a.refreshInterval, profileIdx: idx, profileColor: ProfileBorderColor(p.Color)}
		}
	})
	return textinput.Blink
}

func (a *App) openConnectModal() tea.Cmd {
	a.modal = NewConnectModal(
		a.cfg.Host, a.cfg.Port, a.cfg.Password, a.cfg.DB,
		a.cfg.TLS, a.cfg.TLSSkipVerify,
		refreshIntervalIdx(a.refreshInterval),
		func(r ModalResult) tea.Cmd {
			if !r.Confirmed || len(r.Values) < 7 {
				return nil
			}
			return a.applySettings(r.Values)
		},
	)
	return textinput.Blink
}
