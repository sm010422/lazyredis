package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sm010422/lazyredis/pkg/config"
	redisclient "github.com/sm010422/lazyredis/pkg/redis"
)

// msgBatchDelete carries a list of keys to remove from the panel after deletion.
type msgBatchDeleted struct {
	keys   []string
	status string
	err    error
}

// ---- tab ----

type tabID int

const (
	tabKeys tabID = iota
	tabPubSub
	tabServer
	tabHelp
)

var tabLabels = []string{"  Keys  ", "  PubSub  ", "  Server  ", "  Help  "}

// ---- focus ----

type focusArea int

const (
	focusKeyList focusArea = iota
	focusValue
)

// ---- tea messages ----

type msgKeysLoaded struct {
	keys []string
	err  error
}
type msgValueLoaded struct {
	key  string
	info *redisclient.KeyInfo
	err  error
}
type msgStringValue struct{ val string }
type msgListValue struct{ items []string }
type msgHashValue struct {
	fields map[string]string
	keys   []string
}
type msgSetValue struct{ members []string }
type msgZSetValue struct{ items []ZSetMemberRow }
type msgServerInfo struct {
	info    *redisclient.ServerInfo
	rawInfo string
	err     error
}
type msgStatus struct {
	text    string
	isError bool
}
type msgCmdResult struct {
	cmd    string
	result string
	err    error
}
type msgRefreshTick time.Time
type msgOpDone struct {
	status      string
	err         error
	reload      bool // reload full key list
	reloadValue bool // reload current key's value
}

type msgOpDoneWithNext struct {
	msgOpDone
	nextKey string
}
type msgConnected struct {
	client       *redisclient.Client
	cfg          *config.Config
	interval     time.Duration
	profileIdx   int
	profileColor lipgloss.Color
}
type msgSettingsApplied struct {
	interval time.Duration
}
type msgClearStatus struct{ gen int }
type msgPubSubStats struct {
	stats *redisclient.PubSubStats
	err   error
}
type msgPubSubMessage struct {
	channel string
	payload string
}
type msgPubSubDone struct{}

// ---- App ----

type App struct {
	cfg   *config.Config
	redis *redisclient.Client

	width  int
	height int

	// navigation
	tab             tabID
	focus           focusArea
	currentDB       int
	connected       bool
	dbSize          int64
	typeCache       map[string]string // key -> type string
	refreshInterval time.Duration     // 0 = off

	// profiles
	profiles        []config.Profile
	activeProfileIdx int
	profileColor    lipgloss.Color // active border color

	// panels
	keys   KeysPanel
	value  ValueView
	info   InfoPanel
	server ServerPanel
	pubsub PubSubPanel

	// modal
	modal *Modal

	// status bar
	statusText string
	statusErr  bool
	statusGen  int // incremented each setStatus; used to cancel stale clear timers

	// command log (last N commands)
	cmdLog []string
}

func New(cfg *config.Config, r *redisclient.Client, profiles []config.Profile, activeIdx int) *App {
	color := colorBorderActive
	if activeIdx >= 0 && activeIdx < len(profiles) {
		color = ProfileBorderColor(profiles[activeIdx].Color)
	}
	return &App{
		cfg:              cfg,
		redis:            r,
		keys:             newKeysPanel(),
		pubsub:           newPubSubPanel(),
		typeCache:        make(map[string]string),
		currentDB:        cfg.DB,
		refreshInterval:  2 * time.Second,
		profiles:         profiles,
		activeProfileIdx: activeIdx,
		profileColor:     color,
	}
}

func (a *App) Init() tea.Cmd {
	return tea.Batch(
		a.loadKeys(),
		a.loadServerInfo(),
		a.tickCmd(),
	)
}

func (a *App) tickCmd() tea.Cmd {
	d := a.refreshInterval
	if d <= 0 {
		d = 10 * time.Second // fallback: keep dbSize fresh even when auto-refresh is off
	}
	return tea.Tick(d, func(t time.Time) tea.Msg { return msgRefreshTick(t) })
}

// ---- Update ----

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		return a, nil

	case msgRefreshTick:
		n, _ := a.redis.DBSize()
		a.dbSize = n
		if a.refreshInterval > 0 && a.connected {
			return a, tea.Batch(a.tickCmd(), a.loadKeys())
		}
		return a, a.tickCmd()

	case msgKeysLoaded:
		if msg.err != nil {
			a.connected = false
			a.setStatus("Connection error: "+msg.err.Error(), true)
		} else {
			a.connected = true
			if a.statusText == "Refreshing…" {
				a.setStatus("", false) // explicit clear, no timer needed
			}
			// Evict typeCache entries for keys that no longer exist.
			existing := make(map[string]bool, len(msg.keys))
			for _, k := range msg.keys {
				existing[k] = true
			}
			for k := range a.typeCache {
				if !existing[k] {
					delete(a.typeCache, k)
				}
			}
			prevSel := a.keys.Selected()
			a.keys.SetKeys(msg.keys)
			// restore cursor to previously selected key, or auto-select first
			if prevSel != "" {
				a.keys.FindAndSelect(prevSel)
			}
			sel := a.keys.Selected()
			if sel != "" && (a.value.Key == "" || a.value.Key != sel) {
				return a, tea.Batch(a.loadTypesFor(msg.keys), a.selectKey(sel))
			}
			return a, a.loadTypesFor(msg.keys)
		}
		return a, nil

	case msgValueLoaded:
		if msg.err != nil {
			a.setStatus("Error: "+msg.err.Error(), true)
			return a, nil
		}
		a.value.Key = msg.key
		a.value.Info = msg.info
		a.value.scroll = 0
		a.value.subCursor = 0
		if msg.info != nil {
			a.typeCache[msg.key] = string(msg.info.Type)
		}
		return a, a.loadValueData(msg.key, msg.info)

	case msgStringValue:
		a.value.SetStringValue(msg.val)
		return a, nil

	case msgListValue:
		a.value.SetList(msg.items)
		return a, nil

	case msgHashValue:
		a.value.SetHash(msg.fields, msg.keys)
		return a, nil

	case msgSetValue:
		a.value.SetSet(msg.members)
		return a, nil

	case msgZSetValue:
		a.value.SetZSet(msg.items)
		return a, nil

	case msgServerInfo:
		if msg.err == nil {
			a.server.Info = msg.info
			a.server.RawInfo = msg.rawInfo
		}
		return a, nil

	case msgClearStatus:
		if msg.gen == a.statusGen {
			a.statusText = ""
			a.statusErr = false
		}
		return a, nil

	case msgStatus:
		return a, a.setStatus(msg.text, msg.isError)

	case msgCmdResult:
		entry := "> " + msg.cmd
		var statusCmd tea.Cmd
		if msg.err != nil {
			entry += "\n  ERROR: " + msg.err.Error()
			a.setStatus("Command error: "+msg.err.Error(), true)
		} else {
			entry += "\n  " + truncate(msg.result, 200)
			statusCmd = a.setStatus("OK: "+truncate(msg.result, 60), false)
		}
		a.addCmdLog(entry)
		return a, statusCmd

	case msgOpDone:
		var cmds []tea.Cmd
		if msg.err != nil {
			a.setStatus(msg.err.Error(), true)
		} else {
			cmds = append(cmds, a.setStatus(msg.status, false))
		}
		if msg.reload {
			cmds = append(cmds, a.loadKeys())
			n, _ := a.redis.DBSize()
			a.dbSize = n
		}
		if msg.reloadValue && a.value.Key != "" {
			cmds = append(cmds, a.selectKey(a.value.Key))
		}
		return a, tea.Batch(cmds...)

	case msgOpDoneWithNext:
		statusCmd := a.setStatus(msg.status, false)
		n, _ := a.redis.DBSize()
		a.dbSize = n
		return a, tea.Batch(statusCmd, a.selectKey(msg.nextKey))

	case msgSettingsApplied:
		a.refreshInterval = msg.interval
		label := "off"
		if msg.interval > 0 {
			label = msg.interval.String()
		}
		statusCmd := a.setStatus("Auto-refresh: "+label, false)
		return a, tea.Batch(statusCmd, a.tickCmd())

	case msgConnected:
		a.redis.Close()
		a.redis = msg.client
		a.cfg = msg.cfg
		a.currentDB = msg.cfg.DB
		a.connected = true
		a.refreshInterval = msg.interval
		a.typeCache = make(map[string]string)
		a.keys = newKeysPanel()
		a.value.Reset()
		if a.pubsub.Sub != nil {
			a.pubsub.Sub.Close()
			a.pubsub.Sub = nil
		}
		a.pubsub = newPubSubPanel()
		if msg.profileIdx >= 0 {
			a.activeProfileIdx = msg.profileIdx
		}
		if msg.profileColor != "" {
			a.profileColor = msg.profileColor
		}
		tlsInfo := ""
		if msg.cfg.TLS {
			tlsInfo = " (TLS)"
		}
		statusCmd := a.setStatus(fmt.Sprintf("Connected to %s db%d%s", msg.cfg.Addr(), msg.cfg.DB, tlsInfo), false)
		return a, tea.Batch(statusCmd, a.loadKeys(), a.loadServerInfo(), a.tickCmd())

	case msgPubSubStats:
		if msg.err == nil {
			a.pubsub.Stats = msg.stats
		}
		return a, nil

	case msgPubSubMessage:
		a.pubsub.AddMessage(msg.channel, msg.payload)
		return a, a.waitPubSubMsg()

	case msgPubSubDone:
		// closed externally (connection lost, not manual unsubscribe)
		if a.pubsub.Sub != nil {
			a.pubsub.Sub = nil
			a.pubsub.SubChannel = ""
			return a, a.setStatus("Pub/Sub subscription ended", false)
		}
		return a, nil

	case msgBatchDeleted:
		if msg.err != nil {
			a.setStatus(msg.err.Error(), true)
			return a, nil
		}
		a.keys.RemoveKeys(msg.keys)
		n, _ := a.redis.DBSize()
		a.dbSize = n
		statusCmd := a.setStatus(msg.status, false)
		a.value.Reset()
		return a, statusCmd

	case typeCacheMsg:
		for k, t := range msg {
			a.typeCache[k] = t
		}
		n, _ := a.redis.DBSize()
		a.dbSize = n
		return a, nil

	case tea.KeyMsg:
		return a.handleKey(msg)
	}

	return a, nil
}

// ---- Key handling ----

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Modal takes priority
	if a.modal != nil {
		newModal, cmd := a.modal.Update(msg)
		a.modal = newModal
		return a, cmd
	}

	// Filter input
	if a.keys.filtering {
		return a.handleFilterKey(msg)
	}

	key := msg.String()

	// Global keys
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
		// If it's a Redis glob pattern, do server-side scan
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
	// --- navigation ---
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

	// sub-item navigation (for list/hash/set/zset) OR range-select in key list
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

	// tree navigation
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

	// multi-select (ctrl+space = ctrl+@ = NUL in many terminals)
	case "ctrl+ ", "ctrl+space", "ctrl+@":
		a.keys.ToggleSelect()
		return a, nil

	// --- filter ---
	case "/":
		a.keys.StartFilter()
		return a, textinput.Blink

	// --- refresh ---
	case "r":
		statusCmd := a.setStatus("Refreshing…", false)
		return a, tea.Batch(statusCmd, a.loadKeys(), a.loadServerInfo())

	// --- operations ---
	case "n":
		a.modal = NewNewKeyModal(func(r ModalResult) tea.Cmd {
			if !r.Confirmed || len(r.Values) < 3 {
				return nil
			}
			return a.createKey(r.Values[0], r.Values[1], r.Values[2])
		})
		return a, textinput.Blink

	case "d":
		// Multi-select batch delete
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
		// Dir node delete (all keys with prefix)
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
		// Single key delete
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

// ---- Redis commands ----

func (a *App) loadKeys() tea.Cmd {
	return func() tea.Msg {
		keys, err := a.redis.Scan("*")
		return msgKeysLoaded{keys: keys, err: err}
	}
}

func (a *App) loadKeysPattern(pattern string) tea.Cmd {
	return func() tea.Msg {
		keys, err := a.redis.Scan(pattern)
		return msgKeysLoaded{keys: keys, err: err}
	}
}

func (a *App) loadTypesFor(keys []string) tea.Cmd {
	// Only fetch types for keys not already in cache.
	var missing []string
	for _, k := range keys {
		if _, ok := a.typeCache[k]; !ok {
			missing = append(missing, k)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return func() tea.Msg {
		// Single pipeline: one round-trip for all missing TYPE commands.
		return typeCacheMsg(a.redis.GetKeyTypes(missing))
	}
}

type typeCacheMsg map[string]string

func (a *App) selectKey(key string) tea.Cmd {
	return func() tea.Msg {
		info, err := a.redis.GetKeyInfo(key)
		return msgValueLoaded{key: key, info: info, err: err}
	}
}

func (a *App) loadValueData(key string, info *redisclient.KeyInfo) tea.Cmd {
	if info == nil {
		return nil
	}
	switch info.Type {
	case redisclient.TypeString:
		return func() tea.Msg {
			val, err := a.redis.GetString(key)
			if err != nil {
				return msgStatus{text: err.Error(), isError: true}
			}
			return msgStringValue{val: val}
		}
	case redisclient.TypeList:
		return func() tea.Msg {
			items, err := a.redis.GetList(key, 0, 999)
			if err != nil {
				return msgStatus{text: err.Error(), isError: true}
			}
			return msgListValue{items: items}
		}
	case redisclient.TypeHash:
		return func() tea.Msg {
			fields, err := a.redis.GetHash(key)
			if err != nil {
				return msgStatus{text: err.Error(), isError: true}
			}
			keys := make([]string, 0, len(fields))
			for k := range fields {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			return msgHashValue{fields: fields, keys: keys}
		}
	case redisclient.TypeSet:
		return func() tea.Msg {
			members, err := a.redis.GetSet(key)
			if err != nil {
				return msgStatus{text: err.Error(), isError: true}
			}
			return msgSetValue{members: members}
		}
	case redisclient.TypeZSet:
		return func() tea.Msg {
			items, err := a.redis.GetZSet(key, 0, 999)
			if err != nil {
				return msgStatus{text: err.Error(), isError: true}
			}
			rows := make([]ZSetMemberRow, len(items))
			for i, z := range items {
				rows[i] = ZSetMemberRow{Member: fmt.Sprintf("%v", z.Member), Score: z.Score}
			}
			return msgZSetValue{items: rows}
		}
	case redisclient.TypeStream:
		return func() tea.Msg {
			entries, err := a.redis.GetStream(key, 100)
			if err != nil {
				return msgStatus{text: err.Error(), isError: true}
			}
			var lines []string
			for _, e := range entries {
				lines = append(lines, styleWarning.Render("["+e.ID+"]"))
				fkeys := make([]string, 0, len(e.Fields))
				for k := range e.Fields {
					fkeys = append(fkeys, k)
				}
				sort.Strings(fkeys)
				for _, k := range fkeys {
					lines = append(lines, fmt.Sprintf("  %-20s  %v", k, e.Fields[k]))
				}
			}
			stringValueCache[key] = strings.Join(lines, "\n")
			return msgStringValue{val: strings.Join(lines, "\n")}
		}
	default:
		// Redis module types (JSON, Vector Set, Time Series, etc.)
		// Use raw GET or DO to fetch the serialized value.
		return func() tea.Msg {
			val, err := a.redis.GetModuleValue(key, string(info.Type))
			if err != nil {
				return msgStringValue{val: styleMuted.Render("(use : command to inspect this key)")}
			}
			return msgStringValue{val: val}
		}
	}
}

func (a *App) loadServerInfo() tea.Cmd {
	return func() tea.Msg {
		info, err := a.redis.GetServerInfo()
		if err != nil {
			return msgServerInfo{err: err}
		}
		raw, _ := a.redis.GetRawInfo("")
		return msgServerInfo{info: info, rawInfo: raw}
	}
}

func (a *App) loadPubSubStats() tea.Cmd {
	return func() tea.Msg {
		stats, err := a.redis.GetPubSubStats()
		return msgPubSubStats{stats: stats, err: err}
	}
}

func (a *App) waitPubSubMsg() tea.Cmd {
	sub := a.pubsub.Sub
	return func() tea.Msg {
		msg, ok := <-sub.Channel()
		if !ok {
			return msgPubSubDone{}
		}
		return msgPubSubMessage{channel: msg.Channel, payload: msg.Payload}
	}
}

func (a *App) applySettings(vals []string) tea.Cmd {
	return func() tea.Msg {
		port, _ := strconv.Atoi(vals[1])
		if port <= 0 {
			port = 6379
		}
		db, _ := strconv.Atoi(vals[3])
		if db < 0 || db > 15 {
			db = 0
		}
		interval := parseRefreshInterval(vals[6])

		// If only the interval changed, skip reconnect.
		sameConn := vals[0] == a.cfg.Host &&
			port == a.cfg.Port &&
			vals[2] == a.cfg.Password &&
			db == a.cfg.DB &&
			(vals[4] == "true") == a.cfg.TLS &&
			(vals[5] == "true") == a.cfg.TLSSkipVerify
		if sameConn {
			return msgSettingsApplied{interval: interval}
		}

		newCfg := &config.Config{
			Host:          vals[0],
			Port:          port,
			Password:      vals[2],
			DB:            db,
			TLS:           vals[4] == "true",
			TLSSkipVerify: vals[5] == "true",
			TLSCert:       a.cfg.TLSCert,
			TLSKey:        a.cfg.TLSKey,
			TLSCA:         a.cfg.TLSCA,
		}
		client, err := redisclient.New(newCfg)
		if err != nil {
			return msgStatus{text: "Connection error: " + err.Error(), isError: true}
		}
		if err := client.Ping(); err != nil {
			client.Close()
			return msgStatus{text: "Connection failed: " + err.Error(), isError: true}
		}
		return msgConnected{client: client, cfg: newCfg, interval: interval, profileIdx: -1}
	}
}

func parseRefreshInterval(s string) time.Duration {
	switch s {
	case "1s":
		return 1 * time.Second
	case "2s":
		return 2 * time.Second
	case "5s":
		return 5 * time.Second
	case "10s":
		return 10 * time.Second
	case "30s":
		return 30 * time.Second
	default:
		return 0
	}
}

func refreshIntervalIdx(d time.Duration) int {
	switch d {
	case 1 * time.Second:
		return 1
	case 2 * time.Second:
		return 2
	case 5 * time.Second:
		return 3
	case 10 * time.Second:
		return 4
	case 30 * time.Second:
		return 5
	default:
		return 0
	}
}

func (a *App) createKey(name, typ, val string) tea.Cmd {
	return func() tea.Msg {
		var err error
		switch typ {
		case "string":
			err = a.redis.SetString(name, val, 0)
		case "list":
			err = a.redis.RPush(name, val)
		case "hash":
			parts := strings.SplitN(val, "=", 2)
			field, hval := parts[0], ""
			if len(parts) == 2 {
				hval = parts[1]
			}
			err = a.redis.HSet(name, field, hval)
		case "set":
			err = a.redis.SAdd(name, val)
		case "zset":
			err = a.redis.ZAdd(name, val, 0)
		}
		if err != nil {
			return msgOpDone{err: err}
		}
		return msgOpDone{status: fmt.Sprintf("Created %s '%s'", typ, name), reload: true}
	}
}

func (a *App) deleteKey(key string) tea.Cmd {
	return func() tea.Msg {
		if _, err := a.redis.Unlink(key); err != nil {
			return msgOpDone{err: err}
		}
		a.keys.RemoveKey(key)
		delete(a.typeCache, key)
		a.value.Reset()
		if next := a.keys.SelectedKey(); next != "" {
			return msgOpDoneWithNext{
				msgOpDone: msgOpDone{status: fmt.Sprintf("Deleted '%s'", key)},
				nextKey:   next,
			}
		}
		n, _ := a.redis.DBSize()
		a.dbSize = n
		return msgOpDone{status: fmt.Sprintf("Deleted '%s'", key)}
	}
}

func (a *App) renameKey(old, newName string) tea.Cmd {
	return func() tea.Msg {
		if err := a.redis.Rename(old, newName); err != nil {
			return msgOpDone{err: err}
		}
		return msgOpDone{status: fmt.Sprintf("Renamed '%s' → '%s'", old, newName), reload: true}
	}
}

func (a *App) setTTL(key, secondsStr string) tea.Cmd {
	return func() tea.Msg {
		secs, err := strconv.ParseInt(strings.TrimSpace(secondsStr), 10, 64)
		if err != nil {
			return msgOpDone{err: fmt.Errorf("invalid seconds: %s", secondsStr)}
		}
		if secs == 0 {
			if err := a.redis.Persist(key); err != nil {
				return msgOpDone{err: err}
			}
			return msgOpDone{status: fmt.Sprintf("Removed TTL from '%s'", key), reloadValue: true}
		}
		if err := a.redis.Expire(key, time.Duration(secs)*time.Second); err != nil {
			return msgOpDone{err: err}
		}
		return msgOpDone{status: fmt.Sprintf("TTL set to %ds on '%s'", secs, key), reloadValue: true}
	}
}

func (a *App) switchDB(db int) tea.Cmd {
	if db < 0 || db > 15 {
		a.setStatus(fmt.Sprintf("DB must be 0-15 (got %d)", db), true)
		return nil
	}
	return func() tea.Msg {
		if err := a.redis.SelectDB(db); err != nil {
			return msgStatus{text: "DB switch failed: " + err.Error(), isError: true}
		}
		a.currentDB = db
		a.value.Reset()
		a.typeCache = make(map[string]string)
		return msgStatus{text: fmt.Sprintf("Switched to DB %d", db), isError: false}
	}
}

func (a *App) runCommand(raw string) tea.Cmd {
	return func() tea.Msg {
		parts := strings.Fields(raw)
		if len(parts) == 0 {
			return nil
		}
		args := make([]interface{}, len(parts))
		for i, p := range parts {
			args[i] = p
		}
		result, err := a.redis.Do(args...)
		if err != nil {
			return msgCmdResult{cmd: raw, err: err}
		}
		return msgCmdResult{cmd: raw, result: fmt.Sprintf("%v", result)}
	}
}

// startEdit opens an edit modal appropriate for the current key type.
func (a *App) startEdit() tea.Cmd {
	key := a.keys.Selected()
	if key == "" || a.value.Info == nil {
		return nil
	}

	switch a.value.Info.Type {
	case redisclient.TypeString:
		cur := stringValueCache[key]
		a.modal = NewInputModal("Edit String Value", "Value:", cur, func(r ModalResult) tea.Cmd {
			if !r.Confirmed {
				return nil
			}
			return func() tea.Msg {
				if err := a.redis.SetString(key, r.Values[0], 0); err != nil {
					return msgOpDone{err: err}
				}
				stringValueCache[key] = r.Values[0]
				return msgOpDone{status: "Saved", reloadValue: true}
			}
		})
		return textinput.Blink

	case redisclient.TypeHash:
		field := a.value.SelectedHashField()
		if field == "" {
			return nil
		}
		cur := a.value.hashFields[field]
		a.modal = NewDoubleInputModal(
			fmt.Sprintf("Edit Hash Field: %s", field),
			"field name (tab to switch)",
			"value",
			func(r ModalResult) tea.Cmd {
				if !r.Confirmed || len(r.Values) < 2 {
					return nil
				}
				newField, newVal := r.Values[0], r.Values[1]
				return func() tea.Msg {
					if newField != field {
						if err := a.redis.HDel(key, field); err != nil {
							return msgOpDone{err: err}
						}
					}
					if err := a.redis.HSet(key, newField, newVal); err != nil {
						return msgOpDone{err: err}
					}
					return msgOpDone{status: "Hash field updated", reloadValue: true}
				}
			},
		)
		// pre-fill
		a.modal.inputs[0].SetValue(field)
		a.modal.inputs[1].SetValue(cur)
		a.modal.inputs[0].Focus()
		a.modal.inputs[1].Blur()
		return textinput.Blink

	case redisclient.TypeList:
		idx := int64(a.value.SelectedListIdx())
		if idx < 0 || idx >= int64(len(a.value.listItems)) {
			return nil
		}
		cur := a.value.listItems[idx]
		a.modal = NewInputModal(
			fmt.Sprintf("Edit List[%d]", idx),
			"Value:",
			cur,
			func(r ModalResult) tea.Cmd {
				if !r.Confirmed {
					return nil
				}
				return func() tea.Msg {
					if err := a.redis.LSet(key, idx, r.Values[0]); err != nil {
						return msgOpDone{err: err}
					}
					return msgOpDone{status: "List item updated", reloadValue: true}
				}
			},
		)
		a.modal.inputs[0].SetValue(cur)
		return textinput.Blink

	case redisclient.TypeZSet:
		z := a.value.SelectedZSetMember()
		if z == nil {
			return nil
		}
		a.modal = NewDoubleInputModal(
			fmt.Sprintf("Edit ZSet Member: %s", z.Member),
			"new score",
			"new member name",
			func(r ModalResult) tea.Cmd {
				if !r.Confirmed || len(r.Values) < 2 {
					return nil
				}
				score, err := strconv.ParseFloat(strings.TrimSpace(r.Values[0]), 64)
				if err != nil {
					return func() tea.Msg { return msgOpDone{err: fmt.Errorf("invalid score")} }
				}
				newMember := strings.TrimSpace(r.Values[1])
				if newMember == "" {
					newMember = z.Member
				}
				return func() tea.Msg {
					if err := a.redis.ZRem(key, z.Member); err != nil {
						return msgOpDone{err: err}
					}
					if err := a.redis.ZAdd(key, newMember, score); err != nil {
						return msgOpDone{err: err}
					}
					return msgOpDone{status: "ZSet member updated", reloadValue: true}
				}
			},
		)
		a.modal.inputs[0].SetValue(fmt.Sprintf("%g", z.Score))
		a.modal.inputs[1].SetValue(z.Member)
		a.modal.inputs[0].Focus()
		a.modal.inputs[1].Blur()
		return textinput.Blink
	}
	return nil
}

// startAdd opens an add modal for list/hash/set/zset.
func (a *App) startAdd() tea.Cmd {
	key := a.keys.Selected()
	if key == "" || a.value.Info == nil {
		return nil
	}

	switch a.value.Info.Type {
	case redisclient.TypeList:
		a.modal = NewInputModal("Add to List", "Value (added to right):", "", func(r ModalResult) tea.Cmd {
			if !r.Confirmed {
				return nil
			}
			return func() tea.Msg {
				if err := a.redis.RPush(key, r.Values[0]); err != nil {
					return msgOpDone{err: err}
				}
				return msgOpDone{status: "Added to list", reloadValue: true}
			}
		})
		return textinput.Blink

	case redisclient.TypeHash:
		a.modal = NewDoubleInputModal("Add Hash Field", "field", "value", func(r ModalResult) tea.Cmd {
			if !r.Confirmed || len(r.Values) < 2 {
				return nil
			}
			return func() tea.Msg {
				if err := a.redis.HSet(key, r.Values[0], r.Values[1]); err != nil {
					return msgOpDone{err: err}
				}
				return msgOpDone{status: "Hash field added", reloadValue: true}
			}
		})
		return textinput.Blink

	case redisclient.TypeSet:
		a.modal = NewInputModal("Add Set Member", "Member:", "", func(r ModalResult) tea.Cmd {
			if !r.Confirmed {
				return nil
			}
			return func() tea.Msg {
				if err := a.redis.SAdd(key, r.Values[0]); err != nil {
					return msgOpDone{err: err}
				}
				return msgOpDone{status: "Member added", reloadValue: true}
			}
		})
		return textinput.Blink

	case redisclient.TypeZSet:
		a.modal = NewDoubleInputModal("Add ZSet Member", "score", "member", func(r ModalResult) tea.Cmd {
			if !r.Confirmed || len(r.Values) < 2 {
				return nil
			}
			score, err := strconv.ParseFloat(strings.TrimSpace(r.Values[0]), 64)
			if err != nil {
				return func() tea.Msg { return msgOpDone{err: fmt.Errorf("invalid score")} }
			}
			return func() tea.Msg {
				if err := a.redis.ZAdd(key, r.Values[1], score); err != nil {
					return msgOpDone{err: err}
				}
				return msgOpDone{status: "ZSet member added", reloadValue: true}
			}
		})
		return textinput.Blink
	}
	return nil
}

// startDeleteSubItem deletes the currently selected sub-item.
func (a *App) startDeleteSubItem() tea.Cmd {
	key := a.keys.Selected()
	if key == "" || a.value.Info == nil {
		return nil
	}

	var target, warn string

	switch a.value.Info.Type {
	case redisclient.TypeHash:
		f := a.value.SelectedHashField()
		if f == "" {
			return nil
		}
		target, warn = f, fmt.Sprintf("Delete hash field '%s'?", f)
	case redisclient.TypeSet:
		m := a.value.SelectedSetMember()
		if m == "" {
			return nil
		}
		target, warn = m, fmt.Sprintf("Remove set member '%s'?", m)
	case redisclient.TypeZSet:
		z := a.value.SelectedZSetMember()
		if z == nil {
			return nil
		}
		target, warn = z.Member, fmt.Sprintf("Remove zset member '%s'?", z.Member)
	case redisclient.TypeList:
		idx := a.value.SelectedListIdx()
		target = a.value.listItems[idx]
		warn = fmt.Sprintf("Remove all list items equal to '%s'?", target)
	default:
		return nil
	}

	a.modal = NewConfirmModal("Delete Sub-Item", warn, func(r ModalResult) tea.Cmd {
		if !r.Confirmed {
			return nil
		}
		return func() tea.Msg {
			var err error
			switch a.value.Info.Type {
			case redisclient.TypeHash:
				err = a.redis.HDel(key, target)
			case redisclient.TypeSet:
				err = a.redis.SRem(key, target)
			case redisclient.TypeZSet:
				err = a.redis.ZRem(key, target)
			case redisclient.TypeList:
				err = a.redis.LRem(key, 0, target)
			}
			if err != nil {
				return msgOpDone{err: err}
			}
			return msgOpDone{status: "Deleted", reloadValue: true}
		}
	})
	return nil
}

func (a *App) copyKeyName() (tea.Model, tea.Cmd) {
	name := a.keys.SelectedKeyName()
	if name == "" {
		return a, nil
	}
	_ = clipboard.WriteAll(name)
	return a, a.setStatus(fmt.Sprintf("Copied key: %s", truncate(name, 50)), false)
}

func (a *App) batchDeleteKeys(keys []string) tea.Cmd {
	return func() tea.Msg {
		err := a.redis.BatchUnlink(keys)
		if err != nil {
			return msgBatchDeleted{err: err}
		}
		return msgBatchDeleted{
			keys:   keys,
			status: fmt.Sprintf("Deleted %d keys", len(keys)),
		}
	}
}

func (a *App) copyValue() (tea.Model, tea.Cmd) {
	key := a.keys.Selected()
	if key == "" {
		return a, nil
	}
	var text string
	switch a.value.Info.Type {
	case redisclient.TypeString:
		text = stringValueCache[key]
	case redisclient.TypeHash:
		f := a.value.SelectedHashField()
		if f != "" && a.value.hashFields != nil {
			text = a.value.hashFields[f]
		}
	case redisclient.TypeList:
		i := a.value.SelectedListIdx()
		if i < len(a.value.listItems) {
			text = a.value.listItems[i]
		}
	case redisclient.TypeSet:
		text = a.value.SelectedSetMember()
	case redisclient.TypeZSet:
		z := a.value.SelectedZSetMember()
		if z != nil {
			text = z.Member
		}
	}
	if text == "" {
		return a, a.setStatus("Nothing to copy", false)
	}
	_ = clipboard.WriteAll(text)
	return a, a.setStatus(fmt.Sprintf("Copied to clipboard: %s", truncate(text, 40)), false)
}

func (a *App) subItemCount() int {
	if a.value.Info == nil {
		return 0
	}
	switch a.value.Info.Type {
	case redisclient.TypeList:
		return len(a.value.listItems)
	case redisclient.TypeHash:
		return len(a.value.hashKeys)
	case redisclient.TypeSet:
		return len(a.value.setMembers)
	case redisclient.TypeZSet:
		return len(a.value.zsetItems)
	}
	return 0
}

// setStatus sets the status bar text and, for non-error messages, schedules
// an auto-clear after 3 seconds so the hint bar returns automatically.
func (a *App) setStatus(text string, isErr bool) tea.Cmd {
	a.statusText = text
	a.statusErr = isErr
	a.statusGen++
	if text == "" || isErr {
		return nil
	}
	gen := a.statusGen
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return msgClearStatus{gen: gen}
	})
}

func (a *App) addCmdLog(entry string) {
	a.cmdLog = append(a.cmdLog, entry)
	if len(a.cmdLog) > 50 {
		a.cmdLog = a.cmdLog[len(a.cmdLog)-50:]
	}
}

// ---- View ----

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

	// Modal overlay — render fg on top of background, preserving panel borders.
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
		{"Pub/Sub (tab 4)", [][]string{
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
