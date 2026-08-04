# `/pray` 픽셀 효과 명세

## 1. 목표

기도 이미지·텍스트·preset이 향로 주변의 매번 다른 자연스러운 위치에 나타나고, 연기와 함께 부유하다가 픽셀 단위로 dissolve된다.

완전 random이 아니라 layout을 보존하는 constrained random이다.

## 2. 입력

```go
type PrayerRequest struct {
    ID        string
    SessionID string
    Source    PrayerSource
    Seed      uint64
    Duration  time.Duration
}
```

Source:

- IMAGE
- TEXT
- PRESET

## 3. 이미지

지원:

- PNG
- JPEG
- WebP

제한:

- 4MB
- decode 1024×1024
- render 24×12 terminal cells
- 사용자가 명시한 local path
- 원본 장기 보관 금지

## 4. Raster

Pipeline:

```text
decode
→ alpha crop
→ aspect correction
→ resize
→ color profile
→ TerminalRaster
```

TrueColor:

- half-block `▀`
- top foreground
- bottom background

Fallback:

- ANSI-256
- monochrome glyph ramp

## 5. Text card

- 24 grapheme cluster
- 1~3행
- runewidth 중앙 정렬
- card border
- 같은 dissolve mask 적용

## 6. Safe zones

- LEFT
- RIGHT
- UPPER_LEFT
- UPPER_RIGHT

기준 anchor:

- Burner Rect
- Incense Rect
- Content Rect

제외:

- border
- title
- status
- altar bottom
- burner body
- incense stick
- active prayer Rect

## 7. Weight

```text
RIGHT        35
LEFT         30
UPPER_RIGHT  20
UPPER_LEFT   15
```

유효 zone만 재정규화한다.

## 8. Placement

```text
seed RNG
→ valid zones
→ weighted selection
→ x ±2, y ±1 jitter
→ collision
→ 최대 16회
→ deterministic fallback
→ clamp
```

같은 seed·layout·raster는 같은 결과를 낸다.

## 9. Timeline

기본 3350ms:

- FADE_IN 250ms
- HOLD 800ms
- DISSOLVE 1800ms
- TRAIL 500ms

최소 1.5초, 최대 10초.

## 10. Fade-in

cell별 reveal threshold:

```text
Hash01(seed, x, y, "reveal")
```

## 11. Hold

- full visual
- 최대 1 row 상승
- x drift 최대 1
- prayer smoke burst
- seeded smooth noise

## 12. Dissolve

```text
threshold = Hash01(seed, x, y, "dissolve")
visible = progress < threshold
```

규칙:

- 사라진 cell 재등장 금지
- brightness 감소
- 일부 cell 0~2 row 상승
- 마지막 sparse pixel
- frame random 금지

## 13. Trail

이미지는 사라지고 마지막 위치에서 smoke particle만 남는다.

## 14. Smoke

Particle:

- position
- velocity
- age
- lifetime
- glyph
- brightness

Emitter:

- ambient incense
- prayer hold
- prayer trail

## 15. Resize

저장:

- selected zone
- normalized offset
- seed
- elapsed phase

Resize 후:

- zone 재계산
- offset 복원
- clamp
- rerandomize 금지

## 16. Queue

- active 1
- pending 5
- FIFO
- queue full이면 oldest pending drop
- duplicate ID 무시

## 17. Motion off

- drift 없음
- smoke frame 최소화
- dissolve 4~6단계
- flashing 없음

## 18. 테스트

- same seed
- 100 seed distribution
- bounds property
- overlap property
- forced fallback
- dissolve monotonic
- resize
- queue
- color fallback
- 한글 width
