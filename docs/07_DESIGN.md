# PrayOps 디자인 명세

## 1. 시각 원칙

- 조용함
- 의식적
- 작은 2D 픽셀 장면
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

## 3. Compact status line

기본:

```text
祈 PrayOps · WORKING · 香 64%
```

좁은 화면:

```text
祈 WORKING 64%
```

두 줄은 선택적이며 기본은 한 줄이다.

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

## 8. Prayer image

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
