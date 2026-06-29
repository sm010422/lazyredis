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
	"github.com/sm010422/lazyredis/pkg/config"
	redisclient "github.com/sm010422/lazyredis/pkg/redis"
)

// ---- Key loading ----

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

// ---- Value loading ----

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
		return func() tea.Msg {
			val, err := a.redis.GetModuleValue(key, string(info.Type))
			if err != nil {
				return msgStringValue{val: styleMuted.Render("(use : command to inspect this key)")}
			}
			return msgStringValue{val: val}
		}
	}
}

// ---- Server / Pub-Sub ----

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

// ---- Connection / settings ----

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

// ---- CRUD operations ----

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

// ---- Edit / Add / Delete sub-items ----

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

// ---- Clipboard ----

func (a *App) copyKeyName() (tea.Model, tea.Cmd) {
	name := a.keys.SelectedKeyName()
	if name == "" {
		return a, nil
	}
	_ = clipboard.WriteAll(name)
	return a, a.setStatus(fmt.Sprintf("Copied key: %s", truncate(name, 50)), false)
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
