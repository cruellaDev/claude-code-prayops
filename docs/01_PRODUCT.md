# PrayOps 제품 정의

## 1. 제품 한 줄

**Claude Code의 작업 상태를 향과 연기가 있는 2D 픽셀 고사상으로 보여주고, 기도 명령을 실행하면 로컬 기도 이미지가 향로 옆에 나타났다가 흩어져 사라지는 plugin-first 터미널 제품.**

## 2. 제품 표면

사용자가 인지하는 제품은 네 가지다.

1. Claude Code plugin
2. `/prayops:*` skills
3. compact status line
4. 별도 full TUI `prayops watch`

Go CLI만 단독으로 설치하도록 요구하지 않는다. CLI는 plugin이 사용하는 runtime이자 고급 사용자 표면이다.

## 3. 해결하는 문제

Claude Code가 긴 작업을 수행하는 동안 사용자는 기본 spinner와 로그를 반복해서 확인한다.

PrayOps는 작업 상태를 다음처럼 표현한다.

- prompt 제출: 향 점화
- tool 실행: 연기 활성
- 권한 요청: 인간의 결단이 필요합니다
- turn 완료: 한 차례 기도가 완료되었습니다
- API 실패: 향이 흐려지고 실패 상태
- `/prayops:pray`: 기도 이미지 등장·dissolve

## 4. 핵심 사용자

- Claude Code를 매일 사용하는 개발자
- WSL·macOS·Linux 터미널 사용자
- tmux pane을 사용하는 개발자
- 장시간 agent workflow 사용자
- status line과 terminal aesthetic을 중요하게 보는 사용자

## 5. 가치 제안

### 설치 가능성

사용자는 marketplace와 plugin만 설치한다.

### 신뢰

runtime 다운로드는 명시적 first-use에서만 발생하고 checksum을 검증한다.

### 자동 상태 반영

사용자가 매번 상태 명령을 입력하지 않아도 hooks가 로컬 상태를 갱신한다.

### 시각적 만족

full TUI에서 향 연기와 prayer dissolve를 본다.

### Privacy

prompt·코드·tool payload를 저장하지 않는다.

## 6. 사용자 흐름

### 설치

```text
marketplace 추가
→ prayops plugin 설치
→ plugin reload
→ /prayops:setup
→ runtime 설치 동의
→ checksum 검증
→ doctor
→ 선택적 status line·/pray alias
```

### 첫 기도에서 설치

```text
/prayops:pray
→ runtime 없음 감지
→ 설치 정보 표시
→ 사용자 동의
→ runtime 설치
→ 기도 요청
```

### Full TUI

```text
별도 terminal
→ prayops watch
→ Claude hooks event 소비
→ 향·연기·상태 표시
```

### 기도

```text
/prayops:pray 무사배포
→ text card raster
→ safe random placement
→ fade-in
→ hold
→ dissolve
→ trail smoke
```

## 7. Skill

### `/prayops:setup`

- runtime 설치·업데이트
- doctor
- status line opt-in
- `/pray` alias opt-in
- watch 실행 안내

### `/prayops:pray`

- text
- explicit image path
- preset
- first-use ensure

### `/prayops:watch`

- 별도 terminal 명령 안내
- tmux 사용 예시
- 장시간 Bash tool 실행 금지

### `/prayops:doctor`

- plugin
- runtime
- version
- permissions
- state dir
- status line
- watcher
- terminal capability

## 8. Runtime CLI

```bash
prayops watch
prayops pray
prayops hook claude
prayops statusline claude
prayops setup
prayops doctor
prayops alias install
prayops alias uninstall
```

## 9. 비주얼

포함:

- 향 3개
- 향 연기
- 향로
- 빈 접시 3개
- 상판
- prayer image/card
- 상태 문구

제외:

- 펫
- 캐릭터
- 게임 HUD
- 화려한 배경
- 실제 종교 신상
- 사실적 제물

## 10. 지원 플랫폼

### v0.1

- macOS arm64·amd64
- Linux amd64·arm64
- WSL2

### v0.2

- native Windows PowerShell·Windows Terminal

v0.1 문서에서 native Windows를 지원한다고 주장하지 않는다.

## 11. MVP 비목표

- MCP
- plugin monitor
- 서버
- 클라우드
- 계정
- 텔레메트리
- 공식 Claude marketplace 등록
- 자동 tmux 설치
- kitty/sixel
- Codex 동시 출시
- 여러 theme
- 소리
- agent message 의미 분석

## 12. 성공 지표

- marketplace 추가부터 첫 prayer까지 5분 이내
- setup 성공률 90% 이상
- runtime checksum 검증 100%
- hook p95 100ms 이내
- Claude 작업 중단 유발 0건
- 100 seed bounds 위반 0건
- 동일 seed 재현 100%
- prompt privacy fixture 100%
- uninstall 후 설정 복원 가능
