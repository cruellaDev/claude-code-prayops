# PrayOps 요구사항

## 1. Marketplace

### FR-001 Marketplace manifest

저장소 루트에 `.claude-plugin/marketplace.json`을 제공한다.

필수:

- marketplace name: `prayops`
- owner: `cruellaDev`
- plugin source: `./plugins/prayops`
- description
- version
- plugin version

### FR-002 설치 명령

```text
/plugin marketplace add cruellaDev/claude-code-prayops
/plugin install prayops@prayops
/reload-plugins
```

README 첫 화면에 노출한다.

### FR-003 Validation

CI에서:

```bash
claude plugin validate .
claude plugin validate ./plugins/prayops --strict
```

가능한 환경에서 실행한다. Claude CLI가 없는 CI job에서는 JSON·frontmatter·path fixture를 자체 검증한다.

## 2. Plugin 구조

### FR-010 기본 구조

```text
plugins/prayops/
├─ .claude-plugin/plugin.json
├─ skills/
│  ├─ setup/
│  ├─ pray/
│  ├─ watch/
│  └─ doctor/
├─ hooks/hooks.json
├─ bin/prayops
├─ scripts/
└─ runtime-manifest.json
```

### FR-011 Plugin manifest

- name: `prayops`
- displayName: `PrayOps Terminal`
- version
- description
- author
- repository
- license
- keywords

### FR-012 Plugin cache

plugin은 marketplace install 시 cache로 복사되는 것을 전제로 한다.

plugin은 자신의 directory 밖 파일을 참조하지 않는다.

Go source root를 plugin이 직접 상대경로로 실행하지 않는다.

## 3. Runtime bootstrap

### FR-020 명시적 설치 — v0.3.0에서 개정

**개정 (2026-08-06, v0.3.0).** 원문은 runtime을 인터넷에서 받아오는 것을 전제로
했고, 그래서 SessionStart hook에서의 설치를 금지했다. v0.3.0부터 runtime은
plugin 안에 함께 배포되므로 "설치"는 복사다. 새로 신뢰하는 것이 없으므로 별도
동의를 물을 대상도 아니다. 개정된 규칙:

- SessionStart hook은 plugin 안의 binary를 `${CLAUDE_PLUGIN_DATA}/bin`으로
  복사할 수 있다. 네트워크를 쓰지 않는다.
- 어떤 hook도 네트워크에서 코드를 받아오지 않는다. 이 금지는 그대로다.
- status line은 여전히 설치하지 않는다.

원문 (v0.2.x까지 유효):

> 다음에서만 runtime 설치를 시작할 수 있다: `/prayops:setup`,
> `/prayops:pray` first-use ensure, 사용자가 직접 `scripts/setup.sh` 실행.
> 다음에서는 설치 금지: SessionStart hook, PreToolUse/PostToolUse hook,
> status line.
- plugin load

### FR-021 설치 사전 정보

설치 전에 표시:

- runtime version
- OS
- architecture
- GitHub repository
- release asset
- 설치 위치
- checksum 검증 여부

### FR-022 설치 위치

```text
${CLAUDE_PLUGIN_DATA}/bin/prayops
```

상태:

```text
${CLAUDE_PLUGIN_DATA}/state/
```

설치 metadata:

```text
${CLAUDE_PLUGIN_DATA}/runtime.json
```

### FR-023 Release asset

지원:

```text
prayops_<version>_darwin_arm64.tar.gz
prayops_<version>_darwin_amd64.tar.gz
prayops_<version>_linux_arm64.tar.gz
prayops_<version>_linux_amd64.tar.gz
checksums.txt
```

### FR-024 Checksum

- `checksums.txt` 다운로드
- 대상 asset SHA-256 계산
- 일치한 경우에만 압축 해제·설치
- 불일치 시 temp 삭제·실패
- 기존 정상 runtime 유지

### FR-025 Atomic install

```text
temp directory
→ download
→ checksum
→ extract
→ smoke test
→ new binary rename
→ runtime metadata write
```

설치 도중 실패해도 기존 runtime이 깨지면 안 된다.

### FR-026 Update

plugin bundled runtime manifest version과 설치 metadata를 비교한다.

- 같으면 유지
- 다르면 setup·pray에서 update 안내
- hook에서는 update하지 않음
- 사용자가 거절하면 기존 runtime 사용 가능 여부를 표시

### FR-027 Runtime 없음

hook launcher는 runtime이 없으면 stdout·stderr 없이 exit 0 한다.

status line은 runtime이 없으면 빈 출력 또는 최소 setup 안내를 출력할 수 있으나 반복 노이즈를 만들지 않는다.

## 4. Skills

### FR-030 Setup

`/prayops:setup`

- runtime ensure
- version check
- doctor
- status line 설치 선택
- `/pray` alias 설치 선택
- watch 안내

### FR-031 Pray

`/prayops:pray`

입력:

- 없음 → deploy preset
- text
- `--image <explicit path>`
- preset

규칙:

- 사용자가 이미지 path를 명시한 경우만 읽음
- 자동 이미지 검색 금지
- runtime ensure
- prayer event 전송
- 짧은 완료 문구

### FR-032 Watch

`/prayops:watch`

- long-running TUI를 Claude Bash tool에서 실행하지 않음
- 사용자의 shell에 맞는 명령 안내
- tmux 예시는 opt-in
- watcher 상태 확인

### FR-033 Doctor

`/prayops:doctor`

- plugin root
- plugin data
- runtime
- version
- checksum metadata
- state directory
- hook
- status line
- alias
- terminal color
- watcher

## 5. Alias

### FR-040 `/pray` opt-in

installer는 user scope의 standalone skill 또는 command를 생성해 `/pray`를 제공할 수 있다.

- 기존 `pray` 존재 여부 확인
- 충돌 시 설치 중단
- 기존 파일 덮어쓰기 금지
- 설치 기록
- uninstall 가능

portable plugin 기능은 alias에 의존하지 않는다.

## 6. Status line

### FR-050 Opt-in

setup에서 사용자의 `~/.claude/settings.json` statusLine 변경 전에 확인한다.

### FR-051 Backup

- 기존 statusLine 존재 시 backup
- 전체 settings file을 파괴적으로 덮지 않음
- JSON merge
- uninstall에서 원래 statusLine 복원

### FR-052 Command

```json
{
  "type": "command",
  "command": "\"<plugin-data>/bin/prayops\" statusline claude",
  "padding": 1,
  "refreshInterval": 1
}
```

실제 command path는 설치된 runtime absolute path를 사용한다.

### FR-053 Output — v0.2.0에서 개정

**개정 (2026-08-05, v0.2.0).** 원문의 "1줄 기본"은 status line이 한 줄짜리
표면이라는 잘못된 전제에서 나왔다. Claude Code status line은 명령이 출력한
줄마다 한 행씩 그린다. 향로를 별도 터미널로 밀어냈던 것이 이 오독의 결과다.

- 향로 scene을 여러 줄로 그린다. 좁은 창에서는 한 줄로 물러난다.
- COLUMNS에 따라 compact
- ANSI color optional
- 초당 한 프레임을 넘지 않는다 (`refreshInterval` 최소 1초). 부드러운
  animation은 `prayops watch`에만 있다.
- prompt·transcript를 읽지 않음

### FR-054 범위 — v0.3.0에서 추가

`statusLine`은 사용자 `settings.json`에서만 읽힌다. project의
`.claude/settings.json`이나 `settings.local.json`에 넣은 것은 조용히 무시되므로,
"이 프로젝트에서만"을 설정으로 구현하면 어디에서도 보이지 않게 된다. 범위는
runtime이 판단한다: status line payload의 `workspace.project_dir`를 읽어
`${CLAUDE_PLUGIN_DATA}/statusline-scope.json`의 allowlist와 대조한다. 목록이
비어 있으면 모든 project에서 그린다. 저장하는 것은 경로가 아니라 project key
hash다.

## 7. Hooks

### FR-060 Events

- SessionStart
- UserPromptSubmit
- PreToolUse
- PostToolUse
- PostToolUseFailure
- PermissionRequest
- Stop
- StopFailure
- SessionEnd

### FR-061 Runtime no-op

hook command는 runtime 존재 여부를 먼저 확인한다.

runtime이 없으면 조용히 성공한다.

### FR-062 Side effect only

- permission 결정 반환 금지
- Stop block 금지
- stdout 비움
- stderr 기본 비움
- 오류 rate limit

### FR-063 Privacy

허용:

- session_id
- hook_event_name
- cwd hash
- project basename
- tool_name category
- StopFailure.error enum
- timestamp

금지:

- prompt
- transcript_path 저장
- tool_input
- tool_response
- error_details
- last_assistant_message
- command

## 8. Core runtime

### FR-070 Commands

```bash
prayops watch
prayops pray
prayops hook claude
prayops statusline claude
prayops setup
prayops doctor
prayops alias
```

### FR-071 Event spool

- local atomic JSON
- no daemon
- no socket
- watcher 없이도 write 성공
- corrupt event 격리
- 최근 event ID dedupe

### FR-072 Full TUI

- alternate screen
- resize
- Ctrl+C restore
- panic restore
- latest active Claude session
- full·medium·compact

## 9. Prayer effect

세부는 `06_PRAY_EFFECT.md`를 따른다.

필수:

- PNG/JPEG/WebP
- text card
- preset
- safe random placement
- deterministic seed
- fade-in
- hold
- dissolve
- trail smoke
- queue 5

## 10. Platform

### P0

- macOS
- Linux
- WSL

### P1

- native Windows

README, release assets, CI matrix가 실제 지원 상태와 일치해야 한다.

## 11. 비기능

### NFR-001 Hook

- p95 100ms 목표
- agent blocking 없음
- runtime 없음 no-op

### NFR-002 Runtime

- idle CPU 1% 내외
- active CPU 5% 내외
- memory 50MB 이하 목표
- active 12fps
- idle 4fps

### NFR-003 Security

- release HTTPS
- checksum
- temp file permissions
- path traversal 방지
- archive entry validation
- symlink extraction 방지

### NFR-004 Accessibility

- `NO_COLOR`
- `--motion off`
- status text
- no rapid flashing
- Unicode width

## 12. 인수 테스트

1. marketplace add/install 경로가 유효하다.
2. plugin install 직후 네트워크 다운로드가 없다.
3. runtime 없는 hook이 조용히 exit 0 한다.
4. 첫 pray에서 설치 정보가 표시된다.
5. checksum 실패 시 기존 runtime이 유지된다.
6. plugin update 후 runtime mismatch가 감지된다.
7. statusLine 기존 설정이 복원 가능하다.
8. alias 충돌 시 기존 파일을 건드리지 않는다.
9. prompt와 tool_input이 event file에 없다.
10. full TUI는 Claude Code Bash tool에서 직접 시작되지 않는다.
11. same seed prayer placement가 재현된다.
12. macOS·Linux·WSL release asset이 설치된다.
