# Codex 보조 통합

## 1. 우선순위

Claude Code plugin 공개 출시가 P0다.

Codex 통합은 동일 Go Core를 재사용하지만 Claude marketplace·plugin release를 막지 않는다.

## 2. 위치

```text
integrations/codex/
├─ .codex-plugin/plugin.json
├─ skills/pray/SKILL.md
└─ hooks/hooks.json
```

Claude plugin directory 안에 넣지 않는다.

## 3. 호출

```text
$pray
```

Codex에 custom `/pray` slash command가 있다고 가정하지 않는다.

## 4. Full TUI

```bash
prayops watch --host codex
```

별도 terminal/pane.

## 5. Runtime

Codex 사용자는:

- GitHub Release 직접 설치
- 후속 package manager
- Claude plugin data runtime을 우연히 공유하지 않음

Platform-neutral user state root를 후속 ADR로 검토한다.

## 6. Hooks

- SessionStart
- UserPromptSubmit
- PreToolUse
- PostToolUse
- PermissionRequest
- Stop
- SessionEnd

Codex Stop stdout 계약을 별도로 준수한다.

## 7. Privacy

Claude Adapter와 동일 allowlist.

메시지 내용을 읽어 실패를 추론하지 않는다.

## 8. 출시

Codex integration은 v0.2 이후 별도 release note로 공개한다.
