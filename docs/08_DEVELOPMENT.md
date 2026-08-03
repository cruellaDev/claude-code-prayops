# PrayOps 개발 지침

## 1. 기술

| 영역 | 기술 |
|---|---|
| Runtime | Go 1.26 |
| TUI | Bubble Tea v2 |
| Style | Lip Gloss |
| CLI | Cobra |
| Width | go-runewidth 계열 |
| WebP | golang.org/x/image/webp |
| Test | stdlib + go-cmp |
| Release | GoReleaser |
| CI | GitHub Actions |
| Bootstrap | POSIX shell v0.1 |

## 2. 구조

```text
cmd/prayops/
internal/
├─ app/
├─ domain/
├─ spool/
├─ session/
├─ raster/
├─ effect/
├─ render/
├─ tui/
├─ setup/
├─ statusline/
├─ alias/
└─ platform/
```

## 3. Go 규칙

- gofmt
- go vet
- race test
- context
- wrapped errors
- no global RNG
- injected clock
- exported doc comments
- panic only terminal recovery boundary

## 4. Bootstrap shell

- `set -eu`
- quote all paths
- temp directory
- trap cleanup
- no sudo
- no eval
- no untrusted archive extraction
- curl/wget fallback
- sha256sum/shasum fallback

## 5. Plugin files

- JSON schema validation
- skill YAML frontmatter validation
- scripts executable bit
- no path outside plugin root
- plugin version synchronized

## 6. Hooks

- max stdin 1MB
- allowlist DTO
- raw JSON not persisted
- no transcript reads
- no stdout
- runtime missing no-op
- benchmark
- hook errors rate-limited

## 7. Testing

### Domain

- reducer
- placement
- dissolve
- smoke
- layout
- queue

### Bootstrap

- OS map
- arch map
- URL
- checksum
- rollback
- concurrent lock
- runtime mismatch

### Plugin

- marketplace fixture
- manifest fixture
- hooks fixture
- skill frontmatter
- setup flow
- alias flow
- status line backup

### Integration

- temp plugin data
- hook subprocess
- event spool
- watch render
- release archive install

### Golden

- full idle
- working
- prayer 25%
- prayer 75%
- compact
- mono
- doctor

## 8. CI

Jobs:

1. docs/plugin validation
2. Go lint/test
3. race
4. cross build
5. bootstrap tests
6. release dry-run
7. privacy fixtures

## 9. Release

- semantic version
- changelog
- tag
- GoReleaser
- checksums
- marketplace/plugin/runtime manifest bump
- installation smoke
- README command verification

## 10. R&R

### 지원

- 제품 범위
- 디자인
- 설치 UX
- public README
- release 승인

### Claude

- plugin skill UX
- status line UX
- docs
- privacy review
- golden visual review

### Codex

- Go Core
- setup
- checksum
- hooks
- TUI
- tests
- CI·release

## 11. Definition of Done

- backlog ID
- tests
- plugin validate
- Go tests
- release asset fixture
- install/update/uninstall impact
- privacy
- supported platform
- no scope creep
