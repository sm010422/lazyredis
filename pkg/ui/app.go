package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sm010422/lazyredis/pkg/config"
	redisclient "github.com/sm010422/lazyredis/pkg/redis"
)

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

type msgBatchDeleted struct {
	keys   []string
	status string
	err    error
}
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
	reload      bool
	reloadValue bool
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
	typeCache       map[string]string
	refreshInterval time.Duration

	// profiles
	profiles         []config.Profile
	activeProfileIdx int
	profileColor     lipgloss.Color

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
	statusGen  int

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
		d = 10 * time.Second
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
				a.setStatus("", false)
			}
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

// ---- Status / log helpers ----

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
