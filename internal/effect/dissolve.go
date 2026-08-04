package effect

import "time"

// Phase is where an effect is in its timeline.
type Phase string

const (
	PhaseFadeIn   Phase = "FADE_IN"
	PhaseHold     Phase = "HOLD"
	PhaseDissolve Phase = "DISSOLVE"
	PhaseTrail    Phase = "TRAIL"
	PhaseDone     Phase = "DONE"
)

// The default timeline, 3350ms in total.
const (
	FadeIn   = 250 * time.Millisecond
	Hold     = 800 * time.Millisecond
	Dissolve = 1800 * time.Millisecond
	Trail    = 500 * time.Millisecond

	DefaultDuration = FadeIn + Hold + Dissolve + Trail

	MinDuration = 1500 * time.Millisecond
	MaxDuration = 10 * time.Second
)

// Timeline scales the four phases to a total duration.
type Timeline struct {
	FadeIn   time.Duration
	Hold     time.Duration
	Dissolve time.Duration
	Trail    time.Duration
}

// NewTimeline scales the default proportions to total, clamped to the
// supported range. A zero total means the default.
func NewTimeline(total time.Duration) Timeline {
	if total <= 0 {
		total = DefaultDuration
	}
	total = min(max(total, MinDuration), MaxDuration)

	// Scaled in float: part * total overflows int64 nanoseconds well inside
	// the supported range - 1800ms times 10s is already past the limit.
	scale := func(part time.Duration) time.Duration {
		return time.Duration(float64(part) * float64(total) / float64(DefaultDuration))
	}
	t := Timeline{FadeIn: scale(FadeIn), Hold: scale(Hold), Dissolve: scale(Dissolve)}
	// The trail absorbs the rounding so the phases always sum to the total.
	t.Trail = total - t.FadeIn - t.Hold - t.Dissolve
	return t
}

// Total is the whole timeline.
func (t Timeline) Total() time.Duration {
	return t.FadeIn + t.Hold + t.Dissolve + t.Trail
}

// At returns the phase and that phase's progress in [0,1] at an elapsed time.
func (t Timeline) At(elapsed time.Duration) (Phase, float64) {
	switch {
	case elapsed < 0:
		return PhaseFadeIn, 0
	case elapsed < t.FadeIn:
		return PhaseFadeIn, ratio(elapsed, t.FadeIn)
	case elapsed < t.FadeIn+t.Hold:
		return PhaseHold, ratio(elapsed-t.FadeIn, t.Hold)
	case elapsed < t.FadeIn+t.Hold+t.Dissolve:
		return PhaseDissolve, ratio(elapsed-t.FadeIn-t.Hold, t.Dissolve)
	case elapsed < t.Total():
		return PhaseTrail, ratio(elapsed-t.FadeIn-t.Hold-t.Dissolve, t.Trail)
	default:
		return PhaseDone, 1
	}
}

func ratio(part, whole time.Duration) float64 {
	if whole <= 0 {
		return 1
	}
	return float64(part) / float64(whole)
}

// Mask decides which cells of an effect are visible at a moment.
//
// Both thresholds are hashes of the cell position, not draws from a stream, so
// a cell's fate is fixed for the whole effect. Re-rolling per frame would make
// cells flicker back into existence.
type Mask struct {
	seed     uint64
	timeline Timeline
	reduced  bool
}

// NewMask returns the mask for one effect.
func NewMask(seed uint64, timeline Timeline) Mask {
	return Mask{seed: seed, timeline: timeline}
}

// Reduced returns a mask that steps rather than fades, for viewers who have
// asked for less motion. The cells that go and the order they go in are
// unchanged; only the number of distinct frames is.
func (m Mask) Reduced() Mask {
	m.reduced = true
	return m
}

// progress applies the reduced-motion quantisation, if any.
func (m Mask) progress(raw float64) float64 {
	if m.reduced {
		return Quantise(raw)
	}
	return raw
}

// Visible reports whether the cell at (x,y) is drawn at this elapsed time.
//
// Cells appear in a hash-ordered sequence during fade-in and disappear in a
// different hash-ordered sequence during dissolve. Once a cell has dissolved
// it never comes back.
func (m Mask) Visible(x, y int, elapsed time.Duration) bool {
	phase, progress := m.timeline.At(elapsed)

	switch phase {
	case PhaseFadeIn:
		return Hash01(m.seed, x, y, "reveal") < m.progress(progress)
	case PhaseHold:
		return true
	case PhaseDissolve:
		return m.progress(progress) < Hash01(m.seed, x, y, "dissolve")
	default:
		return false
	}
}

// Rise is how many rows a cell has drifted upward, which gives the dissolve
// its lift. It never decreases within an effect.
func (m Mask) Rise(x, y int, elapsed time.Duration) int {
	phase, progress := m.timeline.At(elapsed)

	switch phase {
	case PhaseDissolve:
		// Only some cells lift, chosen by hash so the same cells lift every
		// time this effect is replayed.
		lift := Hash01(m.seed, x, y, "lift")
		if lift < 0.55 {
			return 0
		}
		if lift < 0.85 {
			return int(m.progress(progress) + 0.5)
		}
		return int(m.progress(progress)*2 + 0.5)
	case PhaseTrail, PhaseDone:
		return 2
	default:
		return 0
	}
}

// Steps quantises progress for reduced motion, so a dissolve reads as a few
// deliberate steps rather than a smooth fade.
const Steps = 5

// Quantise rounds progress to the reduced-motion step count.
func Quantise(progress float64) float64 {
	if progress <= 0 {
		return 0
	}
	if progress >= 1 {
		return 1
	}
	return float64(int(progress*Steps)) / Steps
}
