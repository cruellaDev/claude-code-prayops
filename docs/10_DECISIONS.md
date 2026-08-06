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
| D-010 | ~~runtime은 GitHub Releases에서 first-use 설치한다.~~ **v0.3.0에서 뒤집힘 → D-060** |
| D-011 | plugin install 직후 자동 다운로드하지 않는다. (v0.3.0: 다운로드 자체가 없다) |
| D-012 | ~~hooks는 runtime을 설치하지 않는다.~~ **v0.3.0에서 좁혀짐 → D-061** |
| D-013 | runtime은 `${CLAUDE_PLUGIN_DATA}/bin`에 설치한다. |
| D-014 | state도 `${CLAUDE_PLUGIN_DATA}`에 둔다. |
| D-015 | checksum을 검증한다. |
| D-016 | install은 atomic하다. |
| D-017 | runtime 없는 hook은 no-op이다. |
| D-018 | status line은 opt-in이지만 제품의 주 표면이다. |
| D-019 | 기존 statusLine을 backup·restore한다. |
| D-020 | 향로는 Claude Code 상태줄에 그린다. 상태줄은 여러 줄을 지원한다. |
| D-020a | `prayops watch`는 큰 화면용 부가 기능이며 주 표면이 아니다. |
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

## v0.3.0 개정

| ID | 결정 |
|---|---|
| D-060 | runtime binary 4종(darwin/linux × arm64/amd64)을 plugin에 함께 배포한다. Claude Code가 marketplace를 shallow clone하므로 history 누적 비용이 사용자에게 전가되지 않는다. 커밋된 binary가 source와 어긋나지 않도록 build를 재현 가능하게 하고(`-trimpath -buildvcs=false`, `GOTOOLCHAIN` 고정) CI가 재빌드해 byte 단위로 비교한다. |
| D-061 | SessionStart hook은 plugin 안의 binary를 복사할 수 있다. 금지의 대상은 "네트워크에서 코드를 받아오는 것"이었고, 이미 설치한 plugin 안의 파일을 복사하는 것은 새로 신뢰하는 것이 없다. |
| D-062 | `CLAUDE_PLUGIN_DATA`는 hook 안에서만 신뢰한다. 그 밖에서는 `scripts/plugin-data.sh`의 `resolve_plugin_data`를 거치고, 해석된 값만 하위 프로세스에 넘긴다. 물려받은 값을 그대로 전달하면 다른 plugin의 디렉터리에 상태를 쓰게 된다. |
| D-063 | status line의 project 범위는 설정이 아니라 runtime이 판단한다. `statusLine`은 사용자 settings에서만 읽히므로 project 설정으로 범위를 정하면 어디에서도 보이지 않는다. |
