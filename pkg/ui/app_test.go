package ui

import (
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sm010422/lazyredis/pkg/config"
	redisclient "github.com/sm010422/lazyredis/pkg/redis"
)

// newTestApp returns an App connected to a fresh miniredis, with
// auto-refresh off so only explicit reloads change the key list.
func newTestApp(t *testing.T) (*App, *miniredis.Miniredis) {
	t.Helper()
	m := miniredis.RunT(t)
	host, portStr, _ := net.SplitHostPort(m.Addr())
	port, _ := strconv.Atoi(portStr)
	cfg := &config.Config{Host: host, Port: port}

	r, err := redisclient.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)

	a := New(cfg, r, nil, -1)
	a.refreshInterval = 0
	a.width, a.height = 120, 40
	return a, m
}

// drive runs cmd and feeds every message it produces back into Update,
// following batches, until nothing is left. Commands that do not return
// within a short time (timers such as tea.Tick) are dropped.
func drive(t *testing.T, a *App, cmd tea.Cmd) {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		done := make(chan tea.Msg, 1)
		go func() { done <- c() }()

		var msg tea.Msg
		select {
		case msg = <-done:
		case <-time.After(200 * time.Millisecond):
			continue
		}
		switch msg := msg.(type) {
		case nil:
		case tea.BatchMsg:
			queue = append(queue, msg...)
		case msgRefreshTick, msgClearStatus:
			// timers; following them would loop forever
		default:
			_, next := a.Update(msg)
			queue = append(queue, next)
		}
	}
}

func TestSwitchDBReloadsKeysWithoutAutoRefresh(t *testing.T) {
	a, m := newTestApp(t)
	m.Select(0)
	_ = m.Set("db0:key", "v")
	m.Select(3)
	_ = m.Set("db3:only", "v")

	drive(t, a, a.loadKeys())
	if got := a.keys.allKeys; len(got) != 1 || got[0] != "db0:key" {
		t.Fatalf("DB 0 keys = %v, want [db0:key]", got)
	}

	// Go into db0/ so the tree has a path that does not exist in DB 3.
	a.keys.EnterDir()

	drive(t, a, a.switchDB(3))

	if a.currentDB != 3 || a.redis.DB() != 3 {
		t.Errorf("currentDB = %d, client DB = %d, want 3", a.currentDB, a.redis.DB())
	}
	if got := a.keys.allKeys; len(got) != 1 || got[0] != "db3:only" {
		t.Errorf("after switching to DB 3, keys = %v, want [db3:only]", got)
	}
	if len(a.keys.pathSegs) != 0 {
		t.Errorf("tree path = %s after switching DB, want /", breadcrumbString(a.keys.pathSegs))
	}
	if a.dbSize != 1 {
		t.Errorf("header dbSize = %d, want 1", a.dbSize)
	}
}

func TestSwitchDBOutOfRange(t *testing.T) {
	a, _ := newTestApp(t)
	if cmd := a.switchDB(16); cmd != nil {
		drive(t, a, cmd)
	}
	if a.currentDB != 0 || !a.statusErr {
		t.Errorf("currentDB = %d statusErr = %v, want 0 and an error status", a.currentDB, a.statusErr)
	}
}
