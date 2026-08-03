# PrayOps 확정 결정

| ID | 결정 |
|---|---|
| D-001 | 제품은 Claude Code plugin-first다. |
| D-002 | public repository는 marketplace와 plugin과 Go Core를 함께 가진다. |
| D-003 | marketplace 이름은 `prayops`다. |
| D-004 | plugin 이름은 `prayops`다. |
| D-005 | plugin source는 `./plugins/prayops`다. |
| D-006 | portable prayer skill은 `/prayops:pray`다. |
| D-007 | `/pray`는 opt-in alias다. |
| D-008 | setup·watch·doctor skills를 제공한다. |
| D-009 | runtime은 Go 단일 바이너리다. |
| D-010 | runtime은 GitHub Releases에서 first-use 설치한다. |
| D-011 | plugin install 직후 자동 다운로드하지 않는다. |
| D-012 | hooks는 runtime을 설치하지 않는다. |
| D-013 | runtime은 `${CLAUDE_PLUGIN_DATA}/bin`에 설치한다. |
| D-014 | state도 `${CLAUDE_PLUGIN_DATA}`에 둔다. |
| D-015 | checksum을 검증한다. |
| D-016 | install은 atomic하다. |
| D-017 | runtime 없는 hook은 no-op이다. |
| D-018 | status line은 opt-in이다. |
| D-019 | 기존 statusLine을 backup·restore한다. |
| D-020 | full TUI는 별도 `prayops watch`다. |
| D-021 | `/prayops:watch`는 long-running Bash를 시작하지 않는다. |
| D-022 | plugin monitor를 사용하지 않는다. |
| D-023 | MCP server를 사용하지 않는다. |
| D-024 | hooks는 side-effect only다. |
| D-025 | prompt·code·tool payload·transcript를 저장하지 않는다. |
| D-026 | 펫·캐릭터를 넣지 않는다. |
| D-027 | safe random prayer placement를 사용한다. |
| D-028 | dissolve는 deterministic하다. |
| D-029 | active effect 1, queue 5다. |
| D-030 | macOS·Linux·WSL이 v0.1 P0다. |
| D-031 | native Windows는 v0.2 P1이다. |
| D-032 | kitty/sixel은 MVP 제외다. |
| D-033 | Codex는 secondary integration이다. |
| D-034 | marketplace·plugin·runtime version을 release에서 동기화한다. |
| D-035 | official Anthropic marketplace 등록은 MVP 제외다. |
| D-036 | 자동 tmux 설치는 하지 않는다. |
| D-037 | 텔레메트리는 없다. |
| D-038 | 원격 서버·daemon·socket·DB는 없다. |
