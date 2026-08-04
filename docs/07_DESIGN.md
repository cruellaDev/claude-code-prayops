# PrayOps 디자인 명세

## 1. 시각 원칙

- 조용함
- 의식적
- 작은 2D 픽셀 장면
- 블록 문자(`█ ▄ ▀ ▐▌ ▒ ░`)로 그린다. 박스 드로잉(`│ ─ ┌ ┐`)은 도면처럼 보여
  픽셀 느낌을 주지 못한다
- 건조한 개발자 유머
- 연기와 dissolve 중심
- 펫 없음
- 과도한 게임 UI 없음

## 2. Full TUI

```text
╭──────────────────── PrayOps ────────────────────╮
│                                                  │
│                  .       ~                       │
│               ~      .       ~                  │
│                    │ │ │                         │
│                  ┌───────┐       ▄▓▓▄            │
│                  │       │      ▓████▓           │
│                  └───────┘                       │
│                                                  │
│             (____)   (____)   (____)             │
│          ──────────────────────────────          │
│                                                  │
│  WORKING · 정성을 들이고 있습니다                │
│  Claude · payment-api                            │
╰──────────────────────────────────────────────────╯
```

Prayer visual은 LEFT·RIGHT·UPPER zone에서 등장한다.

## 3. Status line

상태줄이 제품의 주 무대다. Claude Code 상태줄은 `echo` 한 번에 한 행씩 여러 줄을
출력할 수 있고 ANSI 색상과 `COLUMNS`·`LINES`를 지원한다. 향로를 여기에 그린다.

```text
     ░ ░    ░
       ▒
      ░      ░
     ▕▏ ▕▏ ▕▏
   ▄▄▄▄▄▄▄▄▄▄▄▄▄
  ▐▒▒▒▒▒▒▒▒▒▒▒▒▒▌
▐▌▟███████████████▙▐▌
  ▜█████████████▛
   ▝▀▀▀▀▀▀▀▀▀▀▀▘
    █▘   █    ▝█
祈 PrayOps · WORKING · 香 64%
```

향로 폭(21칸)이 들어가지 않는 좁은 창에서는 마지막 한 줄로 물러난다.

```text
祈 WORKING 64%
```

갱신은 `refreshInterval: 1`, 즉 초당 한 프레임이다. 그래서 장면은 **초 단위로
결정적**이어야 한다 — 상태줄은 타이머 외에 이벤트로도 다시 실행되므로, 같은 1초
안에 두 번 실행되면 같은 그림이 나와야 깜빡이지 않는다.

## 4. 상태 문구

| 상태 | 문구 |
|---|---|
| IDLE | 기술적 조치를 기다리고 있습니다 |
| THINKING | 기도를 정리하고 있습니다 |
| WORKING | 정성을 들이고 있습니다 |
| APPROVAL_REQUIRED | 인간의 결단이 필요합니다 |
| TURN_COMPLETED | 한 차례 기도가 완료되었습니다 |
| TURN_FAILED | 기도는 충분했습니다. 상태를 확인하십시오 |
| SESSION_ENDED | 의식을 마쳤습니다 |

## 5. 향

- 3개
- 붉은·갈색 stick
- 회색 ash
- 주황 ember
- slow smoke
- status별 density

## 6. 향로

- Unicode/block vector
- prayer placement anchor
- 실사 이미지 없음

## 7. 접시

- 3개
- 단순 shape
- 상판 위
- 정보보다 장면 구성 역할

## 8. Prayer

`/pray`는 상태줄에 기도 이모지(🙏)를 향로 주변에 잔뜩 뿌린다. 5초에 걸쳐 개수가
줄어들며 사라진다. 향로가 차지한 블록에는 놓지 않는다 — 다리 사이에 끼면 봉헌이
아니라 잡음으로 보인다.

`prayops watch`의 이미지 dissolve는 큰 화면용 부가 기능으로 남는다.

- 최대 24×12 cell
- truecolor pixel
- fallback 유지
- 향로 주변
- dissolve

## 9. Setup UX

Skill 응답:

```text
PrayOps runtime v0.1.0 설치 준비

플랫폼  darwin/arm64
출처    GitHub Releases
검증    SHA-256
위치    Claude plugin data

설치를 진행합니다.
```

광고성·과장 문구보다 신뢰 정보를 우선한다.

## 10. Doctor UX

```text
Plugin       OK  0.1.0
Runtime      OK  0.1.0
Hooks        OK
Status line  Not configured
Alias /pray  Not installed
Watcher      Not running
Terminal     truecolor
```

## 11. Watch 안내

`/prayops:watch`는 다음을 보여준다.

```bash
# 새 터미널
prayops watch

# tmux
tmux split-window -h 'prayops watch'
```

자동 실행하지 않는다.

## 12. Motion

- active 12fps
- idle 4fps
- 강한 flicker 금지
- status text 고정
- motion off 지원

## 13. Color

- terminal background 유지
- ritual red
- ember
- smoke gray
- muted gold
- success/failure

`NO_COLOR`에서 shape과 text만으로 상태를 구분한다.

## 14. Plugin README UX

첫 화면 우선순위:

1. GIF/asciinema
2. 설치 3줄
3. 첫 setup
4. prayer 예시
5. privacy
6. 지원 플랫폼

Architecture 설명은 접어서 아래로 내린다.
