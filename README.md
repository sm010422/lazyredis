<div align="center">

<img src="https://raw.githubusercontent.com/parksangmin/lazyredis/main/assets/logo.png" width="120" alt="LazyRedis logo">

# LazyRedis

**A blazing fast terminal UI for Redis**

[![Go Report Card](https://goreportcard.com/badge/github.com/parksangmin/lazyredis)](https://goreportcard.com/report/github.com/parksangmin/lazyredis)
[![GitHub release](https://img.shields.io/github/v/release/parksangmin/lazyredis?color=blue)](https://github.com/parksangmin/lazyredis/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.21-blue)](https://golang.org/)

```
LazyRedis  ● CONNECTED  db0  9 keys                        redis://127.0.0.1:6379
    Keys           Server           Help
╭─────────────────╮╭──────────────────────────────────────────────────────────────╮
│ Keys  9         ││ leaderboard  ZST                                              │
│ ZST leaderboard ││ RANK  SCORE        MEMBER                                    │
│ HSH profile:alice││    1  72           dave                                      │
│ STR session:abc ││    2  87           charlie                                   │
│ SET tags:go     ││    3  95           bob                                       │
│ LST tasks:queue ││    4  100          alice                                     │
│ STR user:1      ││                                                              │
│ STR user:2      │╰──────────────────────────────────────────────────────────────╯
│                 │╭──────────────────────────────────────────────────────────────╮
│                 ││ Info                                                          │
│                 ││ Key:    leaderboard                                           │
│                 ││ Type:   ZST                                                  │
│                 ││ TTL:    persistent                                            │
│                 ││ Size:   4 elements  Memory: 74 B                             │
╰─────────────────╯╰──────────────────────────────────────────────────────────────╯
 j/k navigate  / filter  n new key  d delete  e edit  a add  R rename  t TTL  q quit
```

</div>

---

## Elevator Pitch

Let me rant for a second. You're debugging a production issue at 2am. You fire up `redis-cli` and type `KEYS *`. 50,000 keys flood your terminal. Great. Now you need to inspect one of them — so you squint at the wall of text, copy a key name, type `TYPE user:session:a1b2c3d4`, then `TTL user:session:a1b2c3d4`, then `HGETALL user:session:a1b2c3d4`, and you're still not sure if that's the right key. Meanwhile you've typed the same key name five times and made a typo twice. *Are you kidding me?*

Want to delete a bunch of keys? Better write a one-liner with `redis-cli KEYS "pattern:*" | xargs redis-cli DEL` and pray it doesn't nuke something you need. Want to see your server stats? `INFO` dumps 100 lines into your terminal. Want to edit a hash field? That's `HSET key field value`, but first you need to remember the exact field name from the `HGETALL` output you just scrolled past.

**If you're tired of juggling `redis-cli` commands and squinting at walls of text, LazyRedis is for you.**

---

## Table of Contents

- [Features](#features)
  - [Browse and inspect keys](#browse-and-inspect-keys)
  - [Full CRUD for all Redis types](#full-crud-for-all-redis-types)
  - [JSON auto-detection](#json-auto-detection)
  - [Live filter](#live-filter)
  - [Server info dashboard](#server-info-dashboard)
  - [Multi-database switching](#multi-database-switching)
  - [Raw command mode](#raw-command-mode)
- [Installation](#installation)
  - [Go](#go)
  - [Homebrew](#homebrew)
  - [Binary releases](#binary-releases)
  - [Build from source](#build-from-source)
- [Usage](#usage)
- [Keybindings](#keybindings)
- [Configuration](#configuration)
- [Supported Redis types](#supported-redis-types)
- [Contributing](#contributing)
- [Alternatives](#alternatives)

---

## Features

### Browse and inspect keys

Navigate keys with `j`/`k` (or arrow keys). The value panel updates instantly as you move — no commands to type, no copy-pasting key names. The info panel shows type, TTL, element count, and memory usage for the selected key.

```
╭──────────────────╮╭───────────────────────────────────────────────────────────╮
│ Keys  9          ││ profile:alice  HSH                                         │
│ STR  config:debug││ FIELD                  VALUE                              │
│ ZST  leaderboard ││ age                    28                                 │
│ HSH  profile:alice││ city                   Seoul                              │
│ STR  session:abc ││ name                   Alice Kim                          │
│ SET  tags:go     │╰───────────────────────────────────────────────────────────╯
│ LST  tasks:queue │╭───────────────────────────────────────────────────────────╮
│ STR  user:1      ││ Info                                                       │
│ STR  user:2      ││ Key:    profile:alice                                      │
│                  ││ Type:   HSH   TTL: persistent   Size: 3 elements           │
╰──────────────────╯╰───────────────────────────────────────────────────────────╯
```

Every key type gets a colour-coded badge:

| Badge | Type | Colour |
|-------|------|--------|
| `STR` | String | 🟢 Green |
| `LST` | List | 🔵 Blue |
| `HSH` | Hash | 🟠 Orange |
| `SET` | Set | 🟡 Yellow |
| `ZST` | Sorted Set | 🟣 Purple |
| `STM` | Stream | 🩵 Teal |

---

### Full CRUD for all Redis types

LazyRedis isn't just a viewer. Every operation you'd normally type as a command is available through modal dialogs:

**Key operations** — `n` new key (with type picker), `d` delete with confirmation, `R` rename, `t` set/remove TTL, `c` copy value to clipboard.

**Sub-item editing** — `tab` to focus the value panel, `J`/`K` to select a row, then:
- `e` — edit the selected item (hash field value, list element, zset score/member)
- `a` — add a new item to a list/hash/set/zset
- `D` — delete the selected sub-item

After every mutation the value panel refreshes instantly — no manual `r` needed.

```
                        ╭────────────────────────────────────╮
                        │ Edit Hash Field: city               │
                        │                                     │
                        │ field name (tab to switch)          │
                        │ > city                              │
                        │                                     │
                        │ value                               │
                        │ > Busan                             │
                        │                                     │
                        │  enter  confirm    esc  cancel      │
                        ╰────────────────────────────────────╯
```

---

### JSON auto-detection

String values that contain valid JSON are automatically pretty-printed with syntax highlighting. No plugin needed.

```
╭──────────────────────────────────────────────╮
│ api:response  STR                            │
│ {                                            │
│   "status": "ok",                            │
│   "user": {                                  │
│     "id": 42,                                │
│     "name": "Alice"                          │
│   },                                         │
│   "tokens": ["read", "write"]                │
│ }                                            │
╰──────────────────────────────────────────────╯
```

---

### Live filter

Press `/` to open the filter bar. As you type, the key list narrows in real time using fuzzy substring matching. Patterns containing `*`, `?`, or `[` are sent to Redis as a `SCAN` glob pattern for server-side filtering — useful when you have millions of keys.

```
╭──────────────────╮
│ Keys  3/42       │
│ /user            │
│ STR  user:1      │
│ STR  user:2      │
│ STR  user:3      │
╰──────────────────╯
```

Press `enter` to lock the filter, `esc` to clear it.

---

### Server info dashboard

Press `2` to switch to the Server tab. See your Redis instance at a glance — version, mode, role, uptime, memory, connected clients, and cache hit ratio. Press `r` to toggle raw `INFO` output.

```
╭────────────────────────────────────────────────╮
│ Server Info  [r] raw                           │
│                                                │
│ ── Server ──                                   │
│   Version:           8.0.1                     │
│   Mode:              standalone                │
│   Role:              master                    │
│   OS:                Darwin 24.0.0 arm64       │
│   Uptime:            2d 4h 12m                 │
│                                                │
│ ── Clients & Memory ──                         │
│   Connected:         3 clients                 │
│   Used Memory:       2.31M                     │
│                                                │
│ ── Stats ──                                    │
│   Total Commands:    18,432                    │
│   Cache Hits:        17,980                    │
│   Hit Ratio:         97.5%                     │
╰────────────────────────────────────────────────╯
```

---

### Multi-database switching

Press `[` and `]` to cycle through Redis databases 0–15. The key list, type cache, and value panel all reset instantly on switch.

---

### Raw command mode

Press `:` to open a command prompt and run any Redis command. Results appear in the command log in the info panel.

```
╭──────────────────────────────────────────────╮
│ Run Command                                   │
│                                               │
│ Redis command:                                │
│ > DEBUG SLEEP 0                               │
│                                               │
│  enter  confirm    esc  cancel                │
╰──────────────────────────────────────────────╯
```

---

## Installation

### Go

```sh
go install github.com/parksangmin/lazyredis@latest
```

### Homebrew

```sh
brew install parksangmin/tap/lazyredis
```

### Binary releases

Pre-built binaries for macOS (arm64, amd64), Linux (amd64, arm64), and Windows are available on the [releases page](https://github.com/parksangmin/lazyredis/releases).

```sh
# macOS arm64 example
curl -Lo lazyredis.tar.gz \
  "https://github.com/parksangmin/lazyredis/releases/latest/download/lazyredis_Darwin_arm64.tar.gz"
tar xf lazyredis.tar.gz
sudo mv lazyredis /usr/local/bin/
```

### Build from source

```sh
git clone https://github.com/parksangmin/lazyredis.git
cd lazyredis
go build -o lazyredis .
sudo mv lazyredis /usr/local/bin/
```

Requires **Go 1.21+**.

---

## Usage

```sh
lazyredis
```

Connect to a specific host, port, password, or database:

```sh
lazyredis --host 192.168.1.10 --port 6380 --pass mysecret --db 3
```

| Flag | Default | Description |
|------|---------|-------------|
| `--host` | `127.0.0.1` | Redis host |
| `--port` | `6379` | Redis port |
| `--pass` | _(empty)_ | Redis password / AUTH string |
| `--db` | `0` | Database number (0–15) |

**Tip:** Add an alias so you can launch it in one keystroke:

```sh
echo "alias lr='lazyredis'" >> ~/.zshrc
```

---

## Keybindings

### Navigation

| Key | Action |
|-----|--------|
| `j` / `k` | Move cursor down / up in key list |
| `↓` / `↑` | Same as j / k |
| `g` | Jump to top of key list |
| `G` | Jump to bottom of key list |
| `ctrl+d` | Page down (10 keys) |
| `ctrl+u` | Page up (10 keys) |
| `tab` / `l` | Switch focus to value panel |
| `h` / `shift+tab` | Switch focus back to key list |
| `J` / `K` | Move sub-item cursor inside value panel (list rows, hash fields, set members, zset members) |
| `j` / `k` (value focused) | Scroll value content |

### Key Operations

| Key | Action |
|-----|--------|
| `n` | Create new key — opens a wizard to choose name, type, and initial value |
| `d` | Delete selected key (confirmation required) |
| `R` | Rename selected key |
| `t` | Set or remove TTL (enter `0` to make persistent) |
| `c` | Copy selected value / member / field-value to clipboard |

### Value Editing

| Key | Action |
|-----|--------|
| `e` | Edit selected item — adapts to the key type: string value, list element by index, hash field+value, zset score+member |
| `a` | Add new item to list (RPush), hash field, set member, or zset member+score |
| `D` | Delete selected sub-item (hash field, set member, zset member, or list element) |

### Filter

| Key | Action |
|-----|--------|
| `/` | Open filter bar — live fuzzy match as you type |
| `enter` | Confirm filter (glob patterns: `*`, `?`, `[` trigger server-side SCAN) |
| `esc` | Clear filter and restore full key list |

### Global

| Key | Action |
|-----|--------|
| `r` | Refresh — reload all keys and server info |
| `:` | Run a raw Redis command |
| `[` | Switch to previous database (db-1) |
| `]` | Switch to next database (db+1) |
| `1` | Keys tab |
| `2` | Server info tab |
| `3` | Help tab |
| `?` | Open help screen |
| `q` / `ctrl+c` | Quit |

---

## Configuration

LazyRedis reads a config file from `~/.config/lazyredis/config.yml` (coming soon). Until then, all options are available as CLI flags.

```yaml
# ~/.config/lazyredis/config.yml  (upcoming)
host: 127.0.0.1
port: 6379
password: ""
db: 0
```

---

## Supported Redis types

| Type | View | Add item | Edit item | Delete item |
|------|------|----------|-----------|-------------|
| String | ✅ JSON pretty-print | — | ✅ `e` | ✅ `d` (whole key) |
| List | ✅ indexed rows | ✅ RPush | ✅ LSet | ✅ LRem |
| Hash | ✅ field/value table | ✅ HSet | ✅ HSet | ✅ HDel |
| Set | ✅ sorted members | ✅ SAdd | — | ✅ SRem |
| Sorted Set | ✅ rank/score/member | ✅ ZAdd | ✅ ZRem + ZAdd | ✅ ZRem |
| Stream | ✅ entry viewer | ✅ XAdd | — | — |

---

## Contributing

Contributions are welcome! Please open an issue first to discuss what you'd like to change.

```sh
git clone https://github.com/parksangmin/lazyredis.git
cd lazyredis
go run main.go          # run from source
go test ./...           # run tests
```

### Project structure

```
lazyredis/
├── main.go                  # entry point, CLI flags
└── pkg/
    ├── config/config.go     # flag parsing
    ├── redis/client.go      # all Redis operations (SCAN, CRUD per type, INFO)
    └── ui/
        ├── app.go           # bubbletea root model, event loop, key routing
        ├── panel_keys.go    # left panel — key list, filter, cursor
        ├── panel_value.go   # right-top panel — type-aware value viewer
        ├── panel_info.go    # right-bottom panel — metadata + command log
        ├── panel_server.go  # server tab — Redis INFO formatted/raw
        ├── modal.go         # reusable overlay dialogs (confirm, input, wizard)
        └── styles.go        # Catppuccin Mocha colour palette
```

### Architecture

LazyRedis uses the [Bubble Tea](https://github.com/charmbracelet/bubbletea) framework (Elm architecture) for the TUI. All Redis I/O runs off the main goroutine as `tea.Cmd` functions and communicates back through typed messages (`tea.Msg`). Styles are built with [Lip Gloss](https://github.com/charmbracelet/lipgloss) using the Catppuccin Mocha palette.

---

## FAQ

**Does LazyRedis support Redis Cluster?**  
Not yet. Single-node and Sentinel setups work. Cluster support is planned.

**Is it safe to use in production?**  
Yes — LazyRedis uses non-blocking `SCAN` for key iteration (never `KEYS *`) and always asks for confirmation before destructive operations. That said, use caution with `:` (raw command mode) as it executes commands directly.

**The type badges aren't showing colours in my terminal.**  
Make sure your terminal supports 256 colours or true colour. Set `TERM=xterm-256color` or `COLORTERM=truecolor` if needed.

**Can I connect over TLS / Redis Sentinel?**  
TLS and Sentinel support are on the roadmap. For now, use an SSH tunnel if needed.

---

## Alternatives

If LazyRedis doesn't fit your needs, these tools might:

- [redis-tui](https://github.com/mylxsw/redis-tui) — another Redis TUI in Go
- [RedisInsight](https://redis.com/redis-enterprise/redis-insight/) — official GUI client (desktop app)
- [redis-cli](https://redis.io/docs/manual/cli/) — the original CLI (no shame in using it)

---

## License

[MIT](LICENSE)

---

<div align="center">

Built with ❤️ and [Bubble Tea](https://github.com/charmbracelet/bubbletea)

If LazyRedis saves you time, consider giving it a ⭐

</div>
