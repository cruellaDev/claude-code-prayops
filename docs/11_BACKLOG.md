# PrayOps 구현 백로그

## Milestone 0 — Plugin/Release Spike

### SP-01 Marketplace scaffold

- marketplace.json
- plugin.json
- skills folders
- hooks skeleton
- plugin validate
- local marketplace install smoke

### SP-02 Runtime release spike

- minimal Go binary
- GoReleaser
- darwin/linux amd64/arm64
- archives
- checksums
- GitHub Release dry-run

### SP-03 Bootstrap spike

- manifest
- OS/arch
- curl/wget
- checksum
- plugin data install
- no-op hook before install
- rollback

## Milestone 1 — Contracts/Core

### FND-01 Contracts

- event
- session
- layout
- raster
- effect
- setup metadata
- runtime manifest

### FND-02 Spool

- paths
- atomic writer
- reader
- rejected
- dedupe
- cleanup

### FND-03 Reducer

- Claude event mapping
- session selection
- prayer queue

## Milestone 2 — Claude Plugin UX

### CLD-01 Setup skill

- installation summary
- consent
- setup command
- doctor
- optional config

### CLD-02 Pray skill

- first-use ensure
- text
- image
- preset
- short response

### CLD-03 Watch skill

- separate terminal
- tmux example
- watcher detection

### CLD-04 Doctor skill

- structured diagnostics
- repair suggestions

### CLD-05 Alias

- detect conflict
- install
- uninstall
- restore

### CLD-06 Status line

- backup
- merge settings
- compact render
- restore

## Milestone 3 — Hooks

### HOK-01 Launcher

- runtime absent no-op
- runtime present exec
- stdout empty
- error rate limit

### HOK-02 Adapter

- lifecycle mapping
- privacy allowlist
- cwd hash
- benchmark

### HOK-03 Fixtures

- prompt
- tool_input
- transcript
- StopFailure
- no persisted sensitive fields

## Milestone 4 — TUI

### TUI-01 Altar

- border
- title
- altar
- plates
- burner
- incense
- status

### TUI-02 Smoke

- ambient
- state density
- motion off
- FPS

### TUI-03 Layout

- full
- medium
- compact
- resize
- Unicode width

## Milestone 5 — Prayer effect

### PRY-01 Raster

- PNG
- JPEG
- WebP
- text card
- preset
- truecolor
- fallback

### PRY-02 Placement

- safe zone
- weight
- jitter
- collision
- fallback
- property tests

### PRY-03 Dissolve

- reveal
- hold
- dissolve
- trail
- monotonic

### PRY-04 Queue

- active 1
- pending 5
- duplicate ID

## Milestone 6 — Release quality

### REL-01 Installation E2E

- local marketplace
- plugin install
- setup
- pray
- watch
- status line
- uninstall

### REL-02 Update E2E

- plugin version bump
- runtime mismatch
- accepted update
- rejected update
- rollback

### REL-03 Platform

- macOS arm64
- macOS amd64
- Linux amd64
- Linux arm64
- WSL

### REL-04 Public README

- recording
- install 3 lines
- privacy
- platform
- troubleshooting

### REL-05 v0.1.0

- changelog
- tag
- release
- checksum
- marketplace update
