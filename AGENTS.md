# AGENTS.md

## 제품 정체성

PrayOps는 Claude Code 사용자를 위한 **plugin-first 제품**이다.

사용자가 설치하는 표면은 Claude Code plugin이며, 실제 픽셀 TUI와 상태 처리는 GitHub Releases에서 배포하는 Go runtime이 담당한다.

펫·캐릭터가 아니라 향·연기·향로·접시·부적·기도 이미지가 핵심이다.

## 비협상 규칙

### 배포

1. 저장소 루트는 Claude Code marketplace다.
2. `.claude-plugin/marketplace.json`을 제공한다.
3. 실제 plugin은 `plugins/prayops/`에 둔다.
4. plugin은 skills, hooks, bin wrapper, setup scripts를 포함한다.
5. Go source와 release 설정은 같은 public repository에 둔다.
6. 사용자 설치 경로는 marketplace 추가 → plugin 설치 → setup이다.
7. plugin 설치 시 자동 네트워크 다운로드를 실행하지 않는다.
8. SessionStart hook에서 runtime을 다운로드하지 않는다.
9. runtime 설치는 `/prayops:setup` 또는 첫 `/prayops:pray`에서 명시적으로 수행한다.
10. 설치 전에 버전, OS/arch, GitHub Release 출처를 사용자에게 보여준다.
11. runtime은 `${CLAUDE_PLUGIN_DATA}/bin/`에 설치한다.
12. plugin 상태는 `${CLAUDE_PLUGIN_ROOT}`에 쓰지 않는다.
13. plugin update 후에도 runtime과 상태는 `${CLAUDE_PLUGIN_DATA}`에서 유지한다.
14. plugin uninstall 시 Claude Code가 data 삭제 여부를 관리한다.
15. GitHub Release checksum을 반드시 검증한다.
16. checksum 검증 실패 시 실행 파일을 설치하지 않는다.
17. hook은 runtime이 없으면 조용히 exit 0 한다.
18. hook은 설치·업데이트를 시도하지 않는다.
19. runtime version mismatch는 skill 또는 doctor에서 사용자에게 안내한다.
20. marketplace와 plugin manifest version을 릴리즈마다 함께 올린다.

### Claude Code 표면

21. portable 기본 기도 호출은 `/prayops:pray`다.
22. `/pray`는 명시적 opt-in user alias다.
23. `/prayops:setup`, `/prayops:watch`, `/prayops:doctor`를 제공한다.
24. `/prayops:watch`는 Claude Bash 도구 안에서 장시간 TUI를 시작하지 않는다.
25. full animation은 별도 `prayops watch` terminal/pane에서 실행한다.
26. status line은 compact snapshot이다.
27. status line 설정은 opt-in이며 기존 설정을 백업한다.
28. plugin `settings.json`으로 user `statusLine`을 강제하지 않는다.
29. plugin monitor를 사용하지 않는다.
30. MCP server를 만들지 않는다.
31. hooks는 상태 이벤트만 기록한다.
32. hooks는 Claude의 permission·stop 결정을 변경하지 않는다.
33. hooks는 exec form을 우선 사용한다.
34. runtime 부재 전용 no-op launcher는 shell form을 사용할 수 있다.
35. hook stdout은 비운다.
36. StopFailure 출력과 exit code가 무시된다는 점을 전제로 side effect만 수행한다.

### Runtime

37. Go 1.26 계열을 사용한다.
38. Bubble Tea v2와 Lip Gloss를 사용한다.
39. 단일 `prayops` 바이너리가 Core다.
40. daemon, socket, database, remote server를 MVP에 추가하지 않는다.
41. lifecycle event 전달은 atomic JSON file spool을 사용한다.
42. hook p95 100ms 이내를 목표로 한다.
43. TUI가 없어도 hook은 성공해야 한다.
44. runtime이 없어도 Claude Code는 정상 동작해야 한다.

### Privacy

45. prompt를 저장하거나 파싱하지 않는다.
46. code를 저장하거나 파싱하지 않는다.
47. tool input/output을 저장하지 않는다.
48. transcript 파일을 읽지 않는다.
49. `last_assistant_message`를 저장하거나 분석하지 않는다.
50. `error_details`를 저장하지 않는다.
51. cwd는 hash와 basename만 event에 기록한다.
52. Prayer image는 사용자가 명시한 로컬 파일만 읽는다.
53. 원본 prayer image를 장기 복사하지 않는다.
54. 텔레메트리는 MVP에서 없다.
55. 네트워크는 runtime 설치·업데이트 시 GitHub Releases에만 사용한다.

### Visual

56. 펫·캐릭터를 추가하지 않는다.
57. full TUI는 향·연기·향로·접시·기도 이미지 중심이다.
58. 기도 위치는 향로 주변 safe zone 안에서만 랜덤이다.
59. 같은 seed·layout에서는 같은 위치가 나와야 한다.
60. dissolve는 cell별 deterministic threshold를 사용한다.
61. 매 프레임 새 random dissolve를 하지 않는다.
62. resize 시 active effect를 다시 추첨하지 않는다.
63. active prayer effect는 1개다.
64. pending queue는 최대 5개다.
65. reduced motion과 `NO_COLOR`를 지원한다.
66. Unicode cell width를 정확히 계산한다.
67. status line에서는 지속 애니메이션을 구현하지 않는다.

### 범위

68. macOS, Linux, WSL을 v0.1 P0로 지원한다.
69. native Windows는 v0.2 P1로 분리한다.
70. Codex는 동일 Core의 secondary integration이며 Claude plugin 출시를 막지 않는다.
71. kitty/sixel/iTerm image protocol은 MVP에서 제외한다.
72. 여러 테마는 MVP에서 제외한다.
73. 자동 tmux 설치는 하지 않는다.
74. 기능과 무관한 대규모 리팩터링을 하지 않는다.
75. 한 작업은 backlog ID 1개, 강결합된 경우 최대 2개다.

## 작업 절차

1. `docs/11_BACKLOG.md`에서 ID를 선택한다.
2. `docs/10_DECISIONS.md`를 확인한다.
3. 관련 요구사항·배포·runtime·effect 문서만 읽는다.
4. 계약을 확인한다.
5. 실패하는 테스트 또는 fixture를 먼저 만든다.
6. 순수 domain을 구현한다.
7. Adapter를 구현한다.
8. plugin skeleton과 runtime을 연결한다.
9. `claude plugin validate` 대상 구조를 검사한다.
10. Go test·build·cross compile을 수행한다.
11. 개인정보와 설치 흐름을 검토한다.
12. 완료 형식으로 보고한다.

## 필수 테스트

- marketplace JSON schema와 상대 source
- plugin manifest와 skill frontmatter
- hooks JSON
- runtime 미설치 hook no-op
- setup source·version 표시
- OS/arch mapping
- checksum success·failure
- atomic install
- plugin update version mismatch
- status line backup·restore
- alias opt-in·restore
- privacy allowlist
- same seed same placement
- dissolve monotonic
- resize no rerandomization
- TUI terminal restore
- macOS·Linux·WSL
- GitHub Release asset naming

## 완료 보고

```markdown
## 완료
- 작업 ID:
- 연결 요구사항:

## 변경
- ...

## 검증
- plugin validate:
- go test:
- build:
- manual:

## 설치·업데이트 영향
- ...

## 개인정보
- 읽은 필드:
- 저장한 필드:
- 네트워크:

## 남은 위험
- ...
```
