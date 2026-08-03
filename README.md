# PrayOps Terminal for Claude Code

> DevOps 이후의 마지막 단계.

PrayOps Terminal은 Claude Code의 작업 상태를 향과 흔들리는 연기가 있는 작은 2D 픽셀 고사상으로 보여주는 플러그인이다.

사용자가 기도를 실행하면 로컬 이미지·텍스트·프리셋이 향로 주변의 안전한 랜덤 위치에서 나타나고, 잠시 떠오른 뒤 픽셀 단위로 흩어져 사라진다.

- 펫·캐릭터 없음
- 프롬프트·코드·tool input/output 저장 없음
- 원격 서버·계정·텔레메트리 없음
- macOS·Linux·WSL 우선 지원
- Go 단일 바이너리
- Claude Code plugin marketplace 배포
- Codex Adapter는 후속·보조 통합

## 사용자 설치

Claude Code에서:

```text
/plugin marketplace add cruellaDev/claude-code-prayops
/plugin install prayops@prayops
/reload-plugins
```

첫 실행:

```text
/prayops:setup
```

또는 바로:

```text
/prayops:pray
```

첫 기도 실행 시 런타임이 없으면 설치할 버전·플랫폼·GitHub Release 출처를 먼저 보여주고 설치한다.

## 사용

### 기도

```text
/prayops:pray
/prayops:pray 무사배포
/prayops:pray --image ./wish.png
```

선택적으로 `/pray` 별칭을 설치할 수 있다.

### 전체 TUI

별도 터미널이나 tmux pane에서:

```bash
prayops watch
```

Claude Code 안의 `/prayops:watch`는 장시간 프로세스를 직접 실행하지 않고 위 명령과 실행 방법을 안내한다.

### 상태줄

설정 후 Claude Code 하단에 compact 상태를 표시한다.

```text
祈 PrayOps · WORKING · 香 64%
```

전체 향 연기·기도 dissolve 애니메이션은 별도 `prayops watch`에서만 보여준다.

### 진단

```text
/prayops:doctor
```

## 저장소 구조

```text
claude-code-prayops/
├─ .claude-plugin/
│  └─ marketplace.json
├─ plugins/
│  └─ prayops/
│     ├─ .claude-plugin/plugin.json
│     ├─ skills/
│     ├─ hooks/
│     ├─ bin/
│     ├─ scripts/
│     └─ runtime-manifest.json
├─ cmd/prayops/
├─ internal/
├─ contracts/
├─ docs/
├─ integrations/codex/
├─ .goreleaser.yaml
└─ AGENTS.md
```

## 문서

| 문서 | 내용 |
|---|---|
| `AGENTS.md` | 구현 에이전트 비협상 규칙 |
| `docs/01_PRODUCT.md` | 제품 범위와 사용자 경험 |
| `docs/02_REQUIREMENTS.md` | 기능·비기능 요구사항 |
| `docs/03_ARCHITECTURE.md` | Core·plugin·spool·TUI 구조 |
| `docs/04_DISTRIBUTION.md` | Marketplace·plugin 배포 |
| `docs/05_RUNTIME_BOOTSTRAP.md` | 최초 설치·업데이트·검증 |
| `docs/06_PRAY_EFFECT.md` | 랜덤 위치·raster·dissolve |
| `docs/07_DESIGN.md` | 픽셀 TUI·상태줄·문구 |
| `docs/08_DEVELOPMENT.md` | Go·테스트·CI·R&R |
| `docs/09_CODEX_INTEGRATION.md` | 동일 Core의 Codex 보조 통합 |
| `docs/10_DECISIONS.md` | 확정 결정 |
| `docs/11_BACKLOG.md` | 구현 순서 |

## 문서 우선순위

1. `AGENTS.md`
2. `docs/10_DECISIONS.md`
3. `docs/02_REQUIREMENTS.md`
4. `docs/05_RUNTIME_BOOTSTRAP.md`
5. `docs/06_PRAY_EFFECT.md`
6. `contracts/`
7. 나머지 문서

## 개발 시작

> `AGENTS.md`, `docs/10_DECISIONS.md`, `docs/11_BACKLOG.md`, `docs/04_DISTRIBUTION.md`, `docs/05_RUNTIME_BOOTSTRAP.md`를 읽고 `SP-01`, `SP-02`, `FND-01`만 구현하라. 테스트를 먼저 만들고, 서버·MCP·monitor·펫·transcript 저장을 추가하지 마라.

## 공식 참고 자료

- Claude Code plugins: https://code.claude.com/docs/en/plugins
- Plugin reference: https://code.claude.com/docs/en/plugins-reference
- Plugin marketplaces: https://code.claude.com/docs/en/plugin-marketplaces
- Claude Code hooks: https://code.claude.com/docs/en/hooks
- Status line: https://code.claude.com/docs/en/statusline
- Go releases: https://go.dev/doc/devel/release
- Bubble Tea: https://github.com/charmbracelet/bubbletea
