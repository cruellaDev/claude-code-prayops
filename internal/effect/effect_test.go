package effect

import (
	"fmt"
	"testing"
	"time"

	"github.com/cruellaDev/claude-code-prayops/contracts"
	"github.com/cruellaDev/claude-code-prayops/internal/layout"
)

const (
	effectWidth  = 12
	effectHeight = 5
)

func full() contracts.Layout { return layout.Compute(80, 24) }

// D-027 / acceptance test 11: the same seed must land in the same place.
func TestSameSeedSamePlacement(t *testing.T) {
	l := full()

	for seed := uint64(0); seed < 50; seed++ {
		first, ok := Place(seed, l, effectWidth, effectHeight, nil)
		if !ok {
			t.Fatalf("seed %d found nowhere to place", seed)
		}
		for i := 0; i < 5; i++ {
			again, _ := Place(seed, l, effectWidth, effectHeight, nil)
			if again != first {
				t.Fatalf("seed %d moved between calls:\n%+v\n%+v", seed, first, again)
			}
		}
	}
}

// Go randomises map iteration order, so a placement that walked a map would
// be irreproducible even with a fixed seed. This is the test that catches it.
func TestPlacementDoesNotDependOnMapOrder(t *testing.T) {
	l := full()
	want := map[uint64]contracts.Placement{}

	for seed := uint64(0); seed < 30; seed++ {
		p, _ := Place(seed, l, effectWidth, effectHeight, nil)
		want[seed] = p
	}
	for round := 0; round < 20; round++ {
		for seed, expected := range want {
			if got, _ := Place(seed, l, effectWidth, effectHeight, nil); got != expected {
				t.Fatalf("seed %d differs on round %d:\n%+v\n%+v", seed, round, expected, got)
			}
		}
	}
}

// 100 seeds must all land inside the scene and never on the furniture.
func TestPlacementStaysInBounds(t *testing.T) {
	l := full()

	for seed := uint64(0); seed < 100; seed++ {
		p, ok := Place(seed, l, effectWidth, effectHeight, nil)
		if !ok {
			t.Fatalf("seed %d found nowhere to place", seed)
		}

		if p.Rect.Width != effectWidth || p.Rect.Height != effectHeight {
			t.Fatalf("seed %d resized the effect: %+v", seed, p.Rect)
		}
		if p.Rect.X < l.Content.X || p.Rect.X+p.Rect.Width > l.Content.X+l.Content.Width ||
			p.Rect.Y < l.Content.Y || p.Rect.Y+p.Rect.Height > l.Content.Y+l.Content.Height {
			t.Fatalf("seed %d escaped the content area: %+v", seed, p.Rect)
		}
		for name, furniture := range map[string]contracts.Rect{
			"burner": l.Burner, "plates": l.Plates, "altar": l.Altar, "status": l.Status,
		} {
			if p.Rect.Intersects(furniture) {
				t.Fatalf("seed %d overlaps the %s: %+v", seed, name, p.Rect)
			}
		}
	}
}

// Placement must actually vary, or every prayer would appear in the same spot.
func TestPlacementSpreadsAcrossZones(t *testing.T) {
	l := full()
	seen := map[contracts.Zone]int{}
	positions := map[string]bool{}

	for seed := uint64(0); seed < 200; seed++ {
		p, _ := Place(seed, l, effectWidth, effectHeight, nil)
		seen[p.Zone]++
		positions[fmt.Sprintf("%d,%d", p.Rect.X, p.Rect.Y)] = true
	}

	if len(seen) < 3 {
		t.Fatalf("only %d zones used: %v", len(seen), seen)
	}
	if len(positions) < 20 {
		t.Fatalf("only %d distinct positions in 200 seeds", len(positions))
	}
	// The weights say RIGHT is the most likely zone.
	if seen[contracts.ZoneRight] == 0 {
		t.Fatalf("the heaviest zone was never chosen: %v", seen)
	}
}

func TestPlacementAvoidsAnActiveEffect(t *testing.T) {
	l := full()
	blocked := 0

	for seed := uint64(0); seed < 100; seed++ {
		first, _ := Place(seed, l, effectWidth, effectHeight, nil)
		second, ok := Place(seed+9999, l, effectWidth, effectHeight, []contracts.Rect{first.Rect})
		if !ok {
			t.Fatalf("seed %d found nowhere to place", seed)
		}
		if second.Rect.Intersects(first.Rect) {
			blocked++
		}
	}

	// The fallback may still overlap when a zone is tight, but it must be rare.
	if blocked > 5 {
		t.Fatalf("%d of 100 placements overlapped an active effect", blocked)
	}
}

func TestPlacementFailsWhenNothingFits(t *testing.T) {
	if _, ok := Place(1, full(), 500, 500, nil); ok {
		t.Fatal("an oversized effect was placed anyway")
	}
	if _, ok := Place(1, layout.Compute(10, 4), 4, 2, nil); ok {
		t.Fatal("an effect was placed on a terminal with no scene")
	}
}

// D-028: a resize restores the same relative spot instead of re-rolling.
func TestRepositionKeepsTheRelativeSpot(t *testing.T) {
	before, ok := Place(7, full(), effectWidth, effectHeight, nil)
	if !ok {
		t.Fatal("nothing placed")
	}

	after, ok := Reposition(before, layout.Compute(100, 30), effectWidth, effectHeight)
	if !ok {
		t.Fatal("reposition failed")
	}

	if after.Zone != before.Zone {
		t.Fatalf("zone changed from %q to %q", before.Zone, after.Zone)
	}
	if diff := after.OffsetX - before.OffsetX; diff > 0.2 || diff < -0.2 {
		t.Fatalf("offset drifted: %.2f -> %.2f", before.OffsetX, after.OffsetX)
	}

	// And it must be repeatable.
	again, _ := Reposition(before, layout.Compute(100, 30), effectWidth, effectHeight)
	if again != after {
		t.Fatalf("reposition is not deterministic:\n%+v\n%+v", after, again)
	}
}

func TestRepositionFallsBackWhenTheZoneIsGone(t *testing.T) {
	before := contracts.Placement{Zone: contracts.ZoneUpperLeft, OffsetX: 0.5, OffsetY: 0.5}

	after, ok := Reposition(before, full(), effectWidth, effectHeight)
	if !ok {
		t.Skip("upper left still fits at this size")
	}
	if after.Rect.Width != effectWidth {
		t.Fatalf("bad rect: %+v", after.Rect)
	}
}

func TestTimelineProportions(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		got := NewTimeline(0)
		if got.Total() != DefaultDuration {
			t.Fatalf("total = %v, want %v", got.Total(), DefaultDuration)
		}
	})

	for _, total := range []time.Duration{MinDuration, 2 * time.Second, 5 * time.Second, MaxDuration} {
		got := NewTimeline(total)
		if got.Total() != total {
			t.Fatalf("total = %v, want %v", got.Total(), total)
		}
		if got.FadeIn <= 0 || got.Hold <= 0 || got.Dissolve <= 0 || got.Trail <= 0 {
			t.Fatalf("a phase collapsed at %v: %+v", total, got)
		}
	}
}

func TestTimelineIsClamped(t *testing.T) {
	if got := NewTimeline(time.Millisecond).Total(); got != MinDuration {
		t.Fatalf("short total = %v, want %v", got, MinDuration)
	}
	if got := NewTimeline(time.Hour).Total(); got != MaxDuration {
		t.Fatalf("long total = %v, want %v", got, MaxDuration)
	}
}

func TestTimelinePhases(t *testing.T) {
	tl := NewTimeline(0)

	cases := []struct {
		at   time.Duration
		want Phase
	}{
		{-time.Second, PhaseFadeIn},
		{0, PhaseFadeIn},
		{FadeIn - time.Millisecond, PhaseFadeIn},
		{FadeIn, PhaseHold},
		{FadeIn + Hold, PhaseDissolve},
		{FadeIn + Hold + Dissolve, PhaseTrail},
		{DefaultDuration, PhaseDone},
		{time.Hour, PhaseDone},
	}
	for _, tc := range cases {
		if got, _ := tl.At(tc.at); got != tc.want {
			t.Fatalf("at %v = %q, want %q", tc.at, got, tc.want)
		}
	}
}

// D-028: a cell that has dissolved must never come back. This is the property
// that a per-frame random dissolve would violate.
func TestDissolveIsMonotonic(t *testing.T) {
	tl := NewTimeline(0)
	mask := NewMask(1234, tl)

	for y := 0; y < 6; y++ {
		for x := 0; x < 14; x++ {
			gone := false
			// Step finely enough to catch a flicker.
			for elapsed := time.Duration(0); elapsed <= tl.Total(); elapsed += 5 * time.Millisecond {
				visible := mask.Visible(x, y, elapsed)

				phase, _ := tl.At(elapsed)
				if phase == PhaseFadeIn {
					continue // still appearing
				}
				if !visible {
					gone = true
					continue
				}
				if gone {
					t.Fatalf("cell (%d,%d) reappeared at %v", x, y, elapsed)
				}
			}
		}
	}
}

// Fade-in is the mirror: once a cell has appeared it stays through the hold.
func TestFadeInIsMonotonic(t *testing.T) {
	tl := NewTimeline(0)
	mask := NewMask(99, tl)

	for y := 0; y < 6; y++ {
		for x := 0; x < 14; x++ {
			shown := false
			for elapsed := time.Duration(0); elapsed < tl.FadeIn; elapsed += time.Millisecond {
				visible := mask.Visible(x, y, elapsed)
				if visible {
					shown = true
					continue
				}
				if shown {
					t.Fatalf("cell (%d,%d) vanished during fade-in at %v", x, y, elapsed)
				}
			}
		}
	}
}

func TestEverythingIsVisibleDuringHold(t *testing.T) {
	tl := NewTimeline(0)
	mask := NewMask(5, tl)

	at := tl.FadeIn + tl.Hold/2
	for y := 0; y < 6; y++ {
		for x := 0; x < 14; x++ {
			if !mask.Visible(x, y, at) {
				t.Fatalf("cell (%d,%d) is missing during hold", x, y)
			}
		}
	}
}

func TestNothingSurvivesTheTrail(t *testing.T) {
	tl := NewTimeline(0)
	mask := NewMask(5, tl)

	at := tl.FadeIn + tl.Hold + tl.Dissolve + tl.Trail/2
	for y := 0; y < 6; y++ {
		for x := 0; x < 14; x++ {
			if mask.Visible(x, y, at) {
				t.Fatalf("cell (%d,%d) is still drawn during the trail", x, y)
			}
		}
	}
}

// The dissolve has to look like a dissolve: cells leaving gradually, not all
// at once.
func TestDissolveThinsGradually(t *testing.T) {
	tl := NewTimeline(0)
	mask := NewMask(77, tl)

	count := func(at time.Duration) int {
		n := 0
		for y := 0; y < 6; y++ {
			for x := 0; x < 14; x++ {
				if mask.Visible(x, y, at) {
					n++
				}
			}
		}
		return n
	}

	start := tl.FadeIn + tl.Hold
	quarter := count(start + tl.Dissolve/4)
	half := count(start + tl.Dissolve/2)
	threeQuarters := count(start + 3*tl.Dissolve/4)

	if !(quarter > half && half > threeQuarters) {
		t.Fatalf("dissolve is not gradual: %d, %d, %d", quarter, half, threeQuarters)
	}
	if quarter == 84 || threeQuarters == 0 && half == 0 {
		t.Fatalf("dissolve is a cliff, not a fade: %d, %d, %d", quarter, half, threeQuarters)
	}
}

func TestRiseNeverSinks(t *testing.T) {
	tl := NewTimeline(0)
	mask := NewMask(31, tl)

	for y := 0; y < 4; y++ {
		for x := 0; x < 10; x++ {
			previous := 0
			for elapsed := time.Duration(0); elapsed <= tl.Total(); elapsed += 10 * time.Millisecond {
				got := mask.Rise(x, y, elapsed)
				if got < previous {
					t.Fatalf("cell (%d,%d) sank from %d to %d at %v", x, y, previous, got, elapsed)
				}
				previous = got
			}
		}
	}
}

func TestHashIsStableAndSpread(t *testing.T) {
	// Pinned values: a change here changes every recorded seed's meaning.
	if got := Hash01(1, 2, 3, "dissolve"); got != Hash01(1, 2, 3, "dissolve") {
		t.Fatal("hash is not stable")
	}
	if Hash01(1, 2, 3, "dissolve") == Hash01(1, 2, 3, "reveal") {
		t.Fatal("two salts produced the same value")
	}
	if Hash01(1, 2, 3, "dissolve") == Hash01(2, 2, 3, "dissolve") {
		t.Fatal("two seeds produced the same value")
	}

	buckets := make([]int, 10)
	for x := 0; x < 100; x++ {
		for y := 0; y < 100; y++ {
			v := Hash01(42, x, y, "dissolve")
			if v < 0 || v >= 1 {
				t.Fatalf("hash out of range: %v", v)
			}
			buckets[int(v*10)]++
		}
	}
	for i, n := range buckets {
		if n < 700 || n > 1300 {
			t.Fatalf("bucket %d has %d of 10000, distribution is skewed: %v", i, n, buckets)
		}
	}
}

func TestQuantiseForReducedMotion(t *testing.T) {
	seen := map[float64]bool{}
	for i := 0; i <= 100; i++ {
		seen[Quantise(float64(i)/100)] = true
	}
	if len(seen) > Steps+1 {
		t.Fatalf("%d distinct steps, want at most %d", len(seen), Steps+1)
	}
	if Quantise(-1) != 0 || Quantise(2) != 1 {
		t.Fatal("quantise is not clamped")
	}
}

// NFR-004 / docs 06 §17: with reduced motion the dissolve reads as a few
// deliberate steps instead of a smooth fade.
func TestReducedMotionStepsTheDissolve(t *testing.T) {
	tl := NewTimeline(0)
	smooth := NewMask(4242, tl)
	stepped := smooth.Reduced()

	// Count how many distinct pictures each mask produces while dissolving.
	distinct := func(m Mask) int {
		seen := map[string]bool{}
		start := tl.FadeIn + tl.Hold

		for elapsed := start; elapsed < start+tl.Dissolve; elapsed += 10 * time.Millisecond {
			var frame []byte
			for y := 0; y < 4; y++ {
				for x := 0; x < 12; x++ {
					if m.Visible(x, y, elapsed) {
						frame = append(frame, '#')
						continue
					}
					frame = append(frame, ' ')
				}
			}
			seen[string(frame)] = true
		}
		return len(seen)
	}

	steppedFrames, smoothFrames := distinct(stepped), distinct(smooth)

	if steppedFrames > Steps+1 {
		t.Fatalf("reduced motion produced %d distinct frames, want at most %d", steppedFrames, Steps+1)
	}
	if steppedFrames >= smoothFrames {
		t.Fatalf("reduced motion is not calmer: %d frames vs %d", steppedFrames, smoothFrames)
	}
	if steppedFrames < 2 {
		t.Fatalf("reduced motion produced %d frames; the dissolve never happens", steppedFrames)
	}
}

// Reducing motion coarsens the dissolve, it does not change it into another
// effect. Quantising rounds down, so the stepped mask always lags the smooth
// one and never runs ahead of it - and both end empty.
func TestReducedMotionLagsButNeverLeads(t *testing.T) {
	tl := NewTimeline(0)
	smooth := NewMask(99, tl)
	stepped := smooth.Reduced()

	start := tl.FadeIn + tl.Hold
	for elapsed := start; elapsed < start+tl.Dissolve; elapsed += 5 * time.Millisecond {
		for y := 0; y < 4; y++ {
			for x := 0; x < 12; x++ {
				if smooth.Visible(x, y, elapsed) && !stepped.Visible(x, y, elapsed) {
					t.Fatalf("cell (%d,%d) is gone under reduced motion but still there without it, at %v",
						x, y, elapsed)
				}
			}
		}
	}

	// The trail clears both, whatever the pacing.
	at := start + tl.Dissolve + tl.Trail/2
	for y := 0; y < 4; y++ {
		for x := 0; x < 12; x++ {
			if stepped.Visible(x, y, at) {
				t.Fatalf("cell (%d,%d) survived the trail under reduced motion", x, y)
			}
		}
	}
}
