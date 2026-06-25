Output demo.gif

Set Shell "zsh"
Set FontSize 15
Set Width 1100
Set Height 620
Set Theme "Catppuccin Mocha"
Set Padding 12
Set PlaybackSpeed 1.0

# ── Launch ────────────────────────────────────────────────────────────────────
Type "/tmp/lazyredis_demo"
Enter
Sleep 3s

# ── Tree: navigate down to user/ folder ───────────────────────────────────────
Down
Sleep 150ms
Down
Sleep 150ms
Down
Sleep 150ms
Down
Sleep 150ms
Down
Sleep 150ms
Down
Sleep 200ms

# Enter user/ folder
Enter
Sleep 800ms

# Enter user/1/ subfolder
Enter
Sleep 800ms

# Move to profile key
Down
Sleep 400ms

# ── Hash viewer: linger so viewer can read fields ─────────────────────────────
Sleep 2s

# Exit subfolder → user/
Backspace
Sleep 500ms

# Move to user 2
Down
Sleep 200ms
Enter
Sleep 500ms
Down
Sleep 200ms
Sleep 1.5s

# ── Back to root ──────────────────────────────────────────────────────────────
Escape
Sleep 500ms

# Move to leaderboard (3rd item at root)
Down
Sleep 150ms
Down
Sleep 150ms
Down
Sleep 300ms

# ── Sorted Set: leaderboard ───────────────────────────────────────────────────
Sleep 2s

# ── Move to config/ → config:app (JSON string) ───────────────────────────────
Up
Sleep 150ms
Up
Sleep 150ms
Enter
Sleep 600ms
Down
Sleep 300ms
Sleep 2s

# ── Back to root, search ─────────────────────────────────────────────────────
Escape
Sleep 400ms
Type "/"
Sleep 300ms
Type "user"
Sleep 1s
Escape
Sleep 600ms

# ── Multi-select cache keys and batch delete ──────────────────────────────────
# Jump to top
Type "g"
Sleep 300ms

# Toggle select on first item (cache/)
Ctrl+@
Sleep 300ms

# Extend selection down one
Type "J"
Sleep 300ms
Sleep 800ms

# Batch delete
Type "d"
Sleep 700ms
Enter
Sleep 1.2s

# ── Final view ────────────────────────────────────────────────────────────────
Sleep 1s

# Quit
Type "q"
Sleep 500ms
