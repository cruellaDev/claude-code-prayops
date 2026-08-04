# Claude Code Plugin 배포 명세

## 1. 배포 목표

사용자가 별도 Go 설치·clone·hook 복사를 하지 않고 Claude Code plugin만 설치하도록 한다.

## 2. Marketplace 구조

루트:

```text
.claude-plugin/marketplace.json
```

예:

```json
{
  "$schema": "https://json.schemastore.org/claude-code-marketplace.json",
  "name": "prayops",
  "owner": {
    "name": "cruellaDev"
  },
  "description": "Pixel ritual tools for Claude Code",
  "version": "0.1.0",
  "plugins": [
    {
      "name": "prayops",
      "source": "./plugins/prayops",
      "description": "A pixel ritual altar and prayer effects for Claude Code",
      "version": "0.1.0",
      "category": "developer-tools",
      "tags": ["tui", "statusline", "hooks", "pixel-art"]
    }
  ]
}
```

`source`는 저장소 루트 기준으로 `./`로 시작하는 상대경로여야 한다. `metadata.pluginRoot`와 합성되지 않으므로 `"prayops"`나 `"plugins/prayops"`는 `claude plugin validate`에서 거부된다.

## 3. Plugin manifest

```json
{
  "$schema": "https://json.schemastore.org/claude-code-plugin-manifest.json",
  "name": "prayops",
  "displayName": "PrayOps Terminal",
  "version": "0.1.0",
  "description": "A pixel ritual altar and prayer effects for Claude Code",
  "author": {
    "name": "cruellaDev"
  },
  "repository": "https://github.com/cruellaDev/claude-code-prayops",
  "license": "MIT",
  "keywords": ["tui", "hooks", "statusline", "pixel-art"]
}
```

Default locations를 사용하므로 skills·hooks path를 불필요하게 중복 선언하지 않는다.

## 4. 사용자 명령

```text
/plugin marketplace add cruellaDev/claude-code-prayops
/plugin install prayops@prayops
/reload-plugins
```

## 5. Version 정책

Marketplace entry와 plugin manifest version을 동일하게 유지한다.

runtime manifest도 같은 app version을 기본으로 한다.

릴리즈:

```text
v0.1.0
v0.1.1
v0.2.0
```

plugin version을 올리지 않으면 marketplace 사용자가 update를 받지 못할 수 있으므로 모든 배포에서 bump한다.

## 6. 설치 scope

기본 권장:

- user scope

project scope:

- 팀이 동일 plugin을 사용하도록 선언할 때

PrayOps는 개인 aesthetic tool이므로 README는 user scope를 기본으로 안내한다.

## 7. Plugin cache

Marketplace plugin은 local cache로 복사된다.

금지:

- plugin 밖 상대 경로
- repository root Go binary 직접 실행
- plugin root에 runtime state 저장
- plugin root에 사용자 config 쓰기

## 8. `bin/`

`plugins/prayops/bin/`은 Bash tool PATH에 추가된다.

포함:

- `prayops`: installed runtime launcher
- `prayops-setup`: setup script launcher

실제 Go binary는 plugin data에 설치한다.

## 9. Skills

### setup

`/prayops:setup`

### pray

`/prayops:pray`

### watch

`/prayops:watch`

### doctor

`/prayops:doctor`

Skill은 모두 `disable-model-invocation: true`를 기본으로 해 사용자가 명시적으로 호출할 때만 동작하도록 한다.

## 10. Hooks

plugin `hooks/hooks.json`에 포함한다.

Hook은 runtime이 없을 때 no-op이므로 plugin 설치 직후 오류를 발생시키지 않는다.

## 11. Status line

plugin default settings로 user statusLine을 강제할 수 없으므로 setup runtime이 user settings를 opt-in 수정한다.

## 12. Monitor 미사용

Plugin monitor는 stdout line을 Claude notification으로 전달하는 용도다.

PrayOps 표면으로 사용하지 않는다. 향로는 상태줄에 그린다.

## 13. 공식 marketplace

초기:

- 자체 public marketplace

후속:

- 안정화
- 보안·privacy 검토
- 공식 marketplace 제출 검토

공식 marketplace 등록은 MVP 완료 조건이 아니다.

## 14. Validation

릴리즈 전:

```bash
claude plugin validate .
claude plugin validate ./plugins/prayops --strict
```

추가 검사:

- marketplace source path
- plugin manifest version
- skill frontmatter
- hooks JSON
- executable bit
- runtime manifest
