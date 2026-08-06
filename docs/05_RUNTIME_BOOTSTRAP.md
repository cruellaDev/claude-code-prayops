> **이 문서는 v0.3.0에서 대체되었다 (2026-08-06).**
>
> 여기 서술된 GitHub Releases 다운로드 bootstrap은 더 이상 설치 경로가 아니다.
> v0.3.0부터 runtime binary 4종이 plugin에 함께 배포되고, SessionStart hook이
> 알맞은 것을 `${CLAUDE_PLUGIN_DATA}/bin`으로 복사한다. 네트워크를 쓰지 않는다.
>
> 현재 규칙은 `docs/10_DECISIONS.md`의 D-060..D-063과
> `docs/02_REQUIREMENTS.md`의 FR-020 개정본을 보라. 구현은
> `plugins/prayops/scripts/ensure-runtime.sh`와 `scripts/build-plugin-runtime.sh`다.
>
> `plugins/prayops/scripts/setup.sh`는 이 문서대로 여전히 동작하지만, 어떤 skill도
> 그것을 호출하지 않는다. 지원되지 않는 플랫폼을 위한 수동 대체 경로로만 남아 있다.
>
> 아래는 v0.2.x까지의 원문이다.

---

# Runtime Bootstrap 명세

## 1. 목표

Plugin 설치는 가볍게 유지하고, 현재 플랫폼용 Go runtime 하나만 first-use에 설치한다.

## 2. 비목표

- plugin install 직후 silent download
- SessionStart automatic download
- 모든 OS binary를 plugin에 번들
- system-wide install
- sudo
- Homebrew 필수
- Go toolchain 필수

## 3. Runtime manifest

Plugin root:

```json
{
  "schemaVersion": 1,
  "runtimeVersion": "0.1.0",
  "repository": "cruellaDev/claude-code-prayops",
  "assetTemplate": "prayops_{version}_{os}_{arch}.tar.gz",
  "checksumAsset": "checksums.txt",
  "supported": [
    {"os": "darwin", "arch": "arm64"},
    {"os": "darwin", "arch": "amd64"},
    {"os": "linux", "arch": "arm64"},
    {"os": "linux", "arch": "amd64"}
  ]
}
```

## 4. 설치 trigger

### `/prayops:setup`

명시적 설치.

### `/prayops:pray`

runtime ensure:

1. runtime 있음·version 일치 → 바로 pray
2. runtime 없음 → 설치 정보·동의
3. runtime 구버전 → update 정보·동의
4. 설치 거절 → 기도 미실행·수동 명령 안내

### Hooks

설치 trigger 아님.

## 5. 설치 안내

사용자에게:

```text
PrayOps runtime v0.1.0을 설치합니다.

플랫폼: darwin/arm64
출처: github.com/cruellaDev/claude-code-prayops/releases
파일: prayops_0.1.0_darwin_arm64.tar.gz
설치 위치: ~/.claude/plugins/data/prayops-prayops/bin/prayops
검증: SHA-256
```

## 6. Platform mapping

```text
uname -s:
Darwin → darwin
Linux → linux

uname -m:
arm64|aarch64 → arm64
x86_64|amd64 → amd64
```

WSL은 linux로 처리한다.

native Windows는 v0.2에서 PowerShell bootstrap을 별도 구현한다.

## 7. Download

우선:

- `curl -fL`

fallback:

- `wget`

둘 다 없으면 수동 설치 안내.

GitHub Release URL:

```text
https://github.com/<repo>/releases/download/v<version>/<asset>
```

## 8. Checksum

지원 도구:

- `sha256sum`
- macOS `shasum -a 256`

절차:

1. checksums 다운로드
2. asset entry 추출
3. local hash 계산
4. 정확한 일치 확인
5. mismatch → temp 삭제
6. 기존 binary 유지

## 9. Archive security

압축 해제 전:

- file list 검사
- absolute path 금지
- `..` path 금지
- symlink 금지
- 예상 binary 외 파일 최소화

## 10. Smoke test

새 binary:

```bash
prayops version --json
prayops doctor --bootstrap-smoke
```

version이 manifest와 일치해야 한다.

## 11. Atomic replacement

```text
plugin-data/tmp/install-<id>/
→ verify
→ plugin-data/bin/prayops.new
→ chmod
→ smoke
→ rename prayops
→ runtime.json
```

기존 runtime은 성공 직전까지 보존한다.

## 12. Metadata

```json
{
  "runtimeVersion": "0.1.0",
  "installedAt": "...",
  "asset": "...",
  "sha256": "...",
  "pluginVersion": "0.1.0"
}
```

## 13. Update

runtime mismatch는 다음에서 확인:

- setup
- pray
- doctor

hook·status line은 자동 update하지 않는다.

Status line은 mismatch가 오래 지속될 때 한 번만 작은 marker를 표시할 수 있다.

## 14. Offline

이미 runtime이 있으면 offline 사용 가능.

runtime이 없고 network가 없으면:

- 실패 이유
- 수동 release asset 경로
- `prayops setup --from <archive>` 후속 기능 검토

## 15. Uninstall

Plugin uninstall은 plugin data 삭제 여부를 Claude Code가 사용자에게 확인한다.

PrayOps alias와 status line은 plugin data 자동 삭제와 별개이므로:

- setup이 변경 기록 저장
- `/prayops:setup --uninstall-user-config` 또는 runtime uninstall 명령 제공
- plugin 제거 전에 doctor에서 복원 안내

## 16. 테스트

- supported map
- unsupported platform
- curl failure
- checksum mismatch
- malicious archive path
- smoke failure
- atomic rollback
- version mismatch
- offline existing runtime
- concurrent setup lock
