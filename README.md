<div align="center">

# LazyRedis

**A lazygit-inspired terminal UI for Redis**

[![GitHub release](https://img.shields.io/github/v/release/sm010422/lazyredis?color=blue)](https://github.com/sm010422/lazyredis/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.21-blue)](https://golang.org/)

```
LazyRedis  ● CONNECTED  db0  63 keys                   redis://127.0.0.1:6379
    Keys        Server        Help
╭────────────────────╮╭────────────────────────────────────────────────────────╮
│Keys  63  /user/    ││user:1:profile  HSH                                     │
│← backspace  esc    ││FIELD                   VALUE                           │
│root                ││age                     28                              │
│  ▶ 1/  (3)         ││city                    Seoul                           │
│  ▶ 2/  (3)         ││name                    Alice Kim                       │
│  ▶ 3/  (2)         ││                                                        │
│                    │╰────────────────────────────────────────────────────────╯
│                    │╭────────────────────────────────────────────────────────╮
│                    ││Info                                                    │
│                    ││Key:     user:1:profile                                 │
│                    ││Type:     HSH                                           │
│                    ││TTL:     persistent                                     │
╰────────────────────╯│Size:    3 elements  Memory: 96 B                      │
                      ╰────────────────────────────────────────────────────────╯
 j/k navigate  /search  n new  d delete  enter enter dir  ← up  p profile  q quit
```

</div>

---

## Elevator Pitch

You're debugging a production issue at 2am. You fire up `redis-cli` and type `KEYS *`. 50,000 keys flood your terminal. Now you need to inspect one — so you copy a key name, type `TYPE user:session:a1b2c3d4`, then `TTL user:session:a1b2c3d4`, then `HGETALL user:session:a1b2c3d4`. You've typed the same key name five times and made a typo twice.

**LazyRedis gives you a lazygit-style TUI that shows everything at once — no commands to remember, no copy-pasting key names.**

---

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Usage](#usage)
- [Keybindings](#keybindings)
- [Multi-Profile Config](#multi-profile-config)
- [Supported Redis Types](#supported-redis-types)
- [Project Structure](#project-structure)
- [Contributing](#contributing)

---

## Features

### Hierarchical Key Tree

Keys are grouped by the `:` delimiter into a navigable folder structure. `user:1:name`, `user:1:email`, `user:2:name` becomes:

```
▶ user/  (3)
  └─ ▶ 1/  (2)
       ├─ STR  name
       └─ STR  email
  └─ ▶ 2/  (1)
       └─ STR  name
```

- `enter` — enter a folder
- `backspace` — go up one level
- `esc` — jump to root
- `d` on a folder — delete all keys under that prefix (with count + confirm)

### Multi-Select & Batch Delete

- `ctrl+space` — toggle selection on any item (leaf or folder)
- `J` / `K` — extend range selection down / up
- `d` with items selected — batch delete everything selected at once

```
│● 3 selected  (d=delete  ctrl+space=toggle)  │
│● ▶ cache/  (12)                             │
│● STR  config:debug                          │
│   STR  session:abc                          │
```

### Multi-Profile Support

Store multiple Redis connections in `~/.config/lazyredis/config.json`. Press `p` to switch profiles without restarting.

```json
{
  "profiles": [
    { "name": "local",      "host": "127.0.0.1", "port": 6379, "color": "green" },
    { "name": "staging",    "host": "staging.internal", "port": 6379, "color": "yellow" },
    { "name": "production", "host": "prod.internal",    "port": 6379, "color": "red", "tls": true }
  ]
}
```

Each profile gets a colour that's reflected in the active panel border.

### Full CRUD for All Redis Types

Every mutation is available through modal dialogs — no commands to type:

- `n` — create a new key (type picker: string / list / hash / set / zset)
- `d` — delete key (or folder, or batch selection) with confirmation
- `R` — rename key
- `t` — set / remove TTL
- `e` — edit selected item (hash field, list element, zset score/member)
- `a` — add item to list / hash / set / zset
- `D` — delete selected sub-item

### JSON Auto-Detection & Hex Display

String values are rendered contextually:

- **Valid JSON** → pretty-printed with syntax highlighting
- **Binary / non-printable bytes** → hex dump with offset + ASCII sidebar
- **Plain text** → shown as-is, paginated

### Copy Without Typing

- `y` — copy the current **key name** to clipboard
- `Y` / `c` — copy the current **value** (or selected sub-item) to clipboard

### TLS Support

```sh
lazyredis --tls --tls-skip-verify
lazyredis --tls --tls-cert ./client.crt --tls-key ./client.key --tls-ca ./ca.crt
```

TLS can also be toggled in the in-TUI connection settings modal (`S` key).

### Auto-Refresh

Configurable auto-refresh (off / 1s / 2s / 5s / 10s / 30s). Default is **2 seconds**. Set it in the connection modal (`S` → Tab to Auto-refresh).

### Server Info Dashboard

Press `2` for the Server tab — version, mode, role, uptime, memory, clients, cache hit ratio. Press `r` to toggle raw `INFO` output.

### Disconnection Recovery

When Redis is unreachable, a clear warning overlay appears with the failed address and instructions. LazyRedis keeps retrying in the background — no restart needed when Redis comes back up.

---

## Installation

### Homebrew (recommended)

```sh
brew tap sm010422/lazyredis
brew install lazyredis
```

### Go install

```sh
go install github.com/sm010422/lazyredis@latest
```

### Binary releases

Pre-built binaries for macOS (arm64 / amd64) and Linux (arm64 / amd64) are available on the [releases page](https://github.com/sm010422/lazyredis/releases).

```sh
# macOS Apple Silicon example
curl -Lo lazyredis.tar.gz \
  "https://github.com/sm010422/lazyredis/releases/latest/download/lazyredis_darwin_arm64.tar.gz"
tar xf lazyredis.tar.gz
sudo mv lazyredis /usr/local/bin/
```

### Build from source

```sh
git clone https://github.com/sm010422/lazyredis.git
cd lazyredis
go build -o lazyredis .
sudo mv lazyredis /usr/local/bin/
```

Requires **Go 1.21+**.

---

## Usage

```sh
lazyredis                                        # localhost:6379
lazyredis --host 192.168.1.10 --port 6380
lazyredis --pass mysecret --db 3
lazyredis --tls
```

| Flag | Default | Description |
|------|---------|-------------|
| `--host` | `127.0.0.1` | Redis host |
| `--port` | `6379` | Redis port |
| `--pass` | _(empty)_ | Redis password |
| `--db` | `0` | Database number (0–15) |
| `--tls` | `false` | Enable TLS |
| `--tls-skip-verify` | `false` | Skip TLS certificate verification |
| `--tls-cert` | _(empty)_ | Path to client certificate |
| `--tls-key` | _(empty)_ | Path to client key |
| `--tls-ca` | _(empty)_ | Path to CA certificate |

---

## Keybindings

### Tree Navigation

| Key | Action |
|-----|--------|
| `j` / `k` | Move cursor down / up |
| `enter` | Enter folder |
| `backspace` | Go up one level |
| `esc` | Go to tree root |
| `g` / `G` | Jump to top / bottom |
| `ctrl+d` / `ctrl+u` | Page down / up (10 items) |

### Multi-Select

| Key | Action |
|-----|--------|
| `ctrl+space` | Toggle selection on current item |
| `J` / `K` | Extend selection range down / up |
| `d` | Batch delete all selected items |

### Key Operations

| Key | Action |
|-----|--------|
| `n` | New key (type picker) |
| `d` | Delete key / folder / selection |
| `R` | Rename key |
| `t` | Set / remove TTL |
| `y` | Copy key name to clipboard |
| `Y` / `c` | Copy value to clipboard |

### Value Editing

| Key | Action |
|-----|--------|
| `tab` / `l` | Focus value panel |
| `h` / `shift+tab` | Focus key list |
| `J` / `K` (value focused) | Move sub-item cursor |
| `e` | Edit selected sub-item |
| `a` | Add item |
| `D` | Delete selected sub-item |
| `j` / `k` (value focused) | Scroll value |

### Search

| Key | Action |
|-----|--------|
| `/` | Open search — fuzzy match across all keys |
| `enter` | Confirm (glob patterns `*` `?` `[` trigger server-side SCAN) |
| `esc` | Close search, return to tree |

### Global

| Key | Action |
|-----|--------|
| `p` | Switch connection profile |
| `S` | Connection settings (host / port / pass / db / TLS / refresh) |
| `[` / `]` | Switch database (db0–db15) |
| `r` | Refresh keys + server info |
| `:` | Run raw Redis command |
| `1` / `2` / `3` | Tab: Keys / Server / Help |
| `?` | Help screen |
| `q` / `ctrl+c` | Quit |

---

## Multi-Profile Config

On first run, LazyRedis creates `~/.config/lazyredis/config.json` with a default `local` profile. Edit it to add more connections:

```json
{
  "profiles": [
    {
      "name": "local",
      "host": "127.0.0.1",
      "port": 6379,
      "db": 0,
      "color": "green"
    },
    {
      "name": "staging",
      "host": "staging.redis.internal",
      "port": 6379,
      "password": "stagingpass",
      "db": 0,
      "color": "yellow"
    },
    {
      "name": "production",
      "host": "prod.redis.internal",
      "port": 6380,
      "password": "prodpass",
      "db": 0,
      "tls": true,
      "color": "red"
    }
  ]
}
```

Available colors: `green`, `blue`, `red`, `yellow`, `purple`, `peach`, `teal`, `pink`, or any hex code (`#a6e3a1`).

Press `p` in the TUI to open the profile selector and switch connections instantly.

---

## Supported Redis Types

| Badge | Type | View | Add | Edit | Delete sub-item |
|-------|------|------|-----|------|-----------------|
| `STR` | String | ✅ JSON / hex / text | — | ✅ | — |
| `LST` | List | ✅ indexed rows | ✅ RPush | ✅ LSet | ✅ LRem |
| `HSH` | Hash | ✅ field/value table | ✅ HSet | ✅ HSet | ✅ HDel |
| `SET` | Set | ✅ sorted members | ✅ SAdd | — | ✅ SRem |
| `ZST` | Sorted Set | ✅ rank/score/member | ✅ ZAdd | ✅ ZRem+ZAdd | ✅ ZRem |
| `STM` | Stream | ✅ entry viewer | — | — | — |
| `JSON` | RedisJSON | ✅ JSON.GET | — | — | — |
| `VEC` | Vector Set | ✅ card count | — | — | — |
| `TS` | Time Series | ✅ TS.INFO | — | — | — |

---

## Project Structure

```
lazyredis/
├── main.go
├── .goreleaser.yaml
└── pkg/
    ├── config/
    │   ├── config.go       # CLI flag parsing
    │   └── profiles.go     # ~/.config/lazyredis/config.json
    ├── redis/
    │   └── client.go       # all Redis operations
    └── ui/
        ├── app.go          # bubbletea root model + event loop
        ├── key_tree.go     # hierarchical key tree builder
        ├── panel_keys.go   # left panel — tree nav, multi-select, search
        ├── panel_value.go  # right-top — type-aware value viewer + hex dump
        ├── panel_info.go   # right-bottom — metadata + command log
        ├── panel_server.go # server tab — Redis INFO
        ├── modal.go        # overlay dialogs (confirm, input, profile, connect)
        ├── overlay.go      # ANSI-aware modal compositing
        └── styles.go       # Catppuccin Mocha palette + profile colors
```

---

## Contributing

Contributions are welcome. Please open an issue first to discuss what you'd like to change.

```sh
git clone https://github.com/sm010422/lazyredis.git
cd lazyredis
go run main.go
```

---

## Alternatives

- [redis-tui](https://github.com/mylxsw/redis-tui) — another Redis TUI in Go
- [RedisInsight](https://redis.com/redis-enterprise/redis-insight/) — official GUI client
- [redis-cli](https://redis.io/docs/manual/cli/) — the original CLI

---

## License

[MIT](LICENSE)

---

<div align="center">

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) · If LazyRedis saves you time, consider giving it a ⭐

</div>
