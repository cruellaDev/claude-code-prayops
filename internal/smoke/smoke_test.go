package smoke

import (
	"testing"

	"github.com/cruellaDev/claude-code-prayops/contracts"
	"github.com/cruellaDev/claude-code-prayops/internal/layout"
	"github.com/cruellaDev/claude-code-prayops/internal/render"
)

func run(seed uint64, steps int, phase contracts.SessionPhase, motion bool) *Field {
	f := New(seed)
	l := layout.Compute(56, 18)
	for i := 0; i < steps; i++ {
		f.Step(Options{Layout: l, Phase: phase, Motion: motion})
	}
	return f
}

// A field must be reproducible from its seed, or the same session would look
// different on every restart.
func TestSameSeedSameField(t *testing.T) {
	a := run(42, 60, contracts.PhaseWorking, true)
	b := run(42, 60, contracts.PhaseWorking, true)

	l := layout.Compute(56, 18)
	left, right := a.Render(l), b.Render(l)

	if len(left) != len(right) {
		t.Fatalf("%d vs %d particles", len(left), len(right))
	}
	for i := range left {
		if left[i] != right[i] {
			t.Fatalf("particle %d differs: %+v vs %+v", i, left[i], right[i])
		}
	}
}

func TestDifferentSeedsDiffer(t *testing.T) {
	l := layout.Compute(56, 18)
	a := run(1, 60, contracts.PhaseWorking, true).Render(l)
	b := run(2, 60, contracts.PhaseWorking, true).Render(l)

	same := len(a) == len(b)
	if same {
		for i := range a {
			if a[i] != b[i] {
				same = false
				break
			}
		}
	}
	if same {
		t.Fatal("two seeds produced an identical field")
	}
}

// The scene should read as activity without any text, so a working session
// has to smoke more than an idle one.
func TestDensityFollowsThePhase(t *testing.T) {
	const steps = 300
	counts := map[contracts.SessionPhase]int{}

	for _, phase := range []contracts.SessionPhase{
		contracts.PhaseWorking,
		contracts.PhaseThinking,
		contracts.PhaseIdle,
		contracts.PhaseSessionEnded,
	} {
		counts[phase] = run(7, steps, phase, true).Len()
	}

	if !(counts[contracts.PhaseWorking] > counts[contracts.PhaseThinking]) {
		t.Fatalf("working %d is not busier than thinking %d",
			counts[contracts.PhaseWorking], counts[contracts.PhaseThinking])
	}
	if !(counts[contracts.PhaseThinking] > counts[contracts.PhaseIdle]) {
		t.Fatalf("thinking %d is not busier than idle %d",
			counts[contracts.PhaseThinking], counts[contracts.PhaseIdle])
	}
	if counts[contracts.PhaseSessionEnded] != 0 {
		t.Fatalf("an ended session still smokes: %d", counts[contracts.PhaseSessionEnded])
	}
}

// NFR-004: reduced motion keeps the scene legible.
func TestMotionOffStillsTheField(t *testing.T) {
	l := layout.Compute(56, 18)

	f := New(3)
	for i := 0; i < 40; i++ {
		f.Step(Options{Layout: l, Phase: contracts.PhaseWorking, Motion: false})
	}

	for _, p := range f.particles {
		if p.VX != 0 || p.VY != 0 {
			t.Fatalf("a particle drifts with motion off: %+v", p)
		}
	}

	moving := run(3, 40, contracts.PhaseWorking, true).Len()
	if f.Len() >= moving {
		t.Fatalf("motion off produced %d particles, not fewer than %d", f.Len(), moving)
	}
	if f.Len() == 0 {
		t.Fatal("motion off produced no smoke at all")
	}
}

// A watcher runs for hours; the field must not grow without bound.
func TestFieldIsBounded(t *testing.T) {
	f := run(9, 5000, contracts.PhaseWorking, true)

	if f.Len() > maxParticles {
		t.Fatalf("%d particles, want at most %d", f.Len(), maxParticles)
	}
}

// Particles must never be drawn over the border, the title, or the altar.
func TestRenderedParticlesStayInTheScene(t *testing.T) {
	for _, size := range [][2]int{{56, 18}, {40, 13}, {30, 9}, {24, 8}} {
		l := layout.Compute(size[0], size[1])

		f := New(11)
		for i := 0; i < 200; i++ {
			f.Step(Options{Layout: l, Phase: contracts.PhaseWorking, Motion: true})

			for _, p := range f.Render(l) {
				if p.Y < l.Content.Y || p.Y >= l.Altar.Y {
					t.Fatalf("%v: particle at y=%d escapes the scene (content %d, altar %d)",
						size, p.Y, l.Content.Y, l.Altar.Y)
				}
				if p.X < l.Content.X || p.X >= l.Content.X+l.Content.Width {
					t.Fatalf("%v: particle at x=%d escapes the content area", size, p.X)
				}
			}
		}
	}
}

func TestSmokeRisesAndFades(t *testing.T) {
	l := layout.Compute(56, 18)
	f := New(5)
	f.Step(Options{Layout: l, Phase: contracts.PhaseWorking, Motion: true, Burst: 1})

	if f.Len() == 0 {
		t.Fatal("a burst produced nothing")
	}
	start := f.particles[0]

	for i := 0; i < 4; i++ {
		f.Step(Options{Layout: l, Phase: contracts.PhaseWorking, Motion: true})
	}
	if len(f.particles) == 0 {
		t.Fatal("the field emptied too fast")
	}

	// The first particle either rose or expired; it must not have sunk.
	for _, p := range f.particles {
		if p.Life == start.Life && p.Y > start.Y {
			t.Fatalf("smoke sank: %+v started at %+v", p, start)
		}
	}
}

func TestBurstUsesItsOwnOrigin(t *testing.T) {
	l := layout.Compute(56, 18)
	origin := contracts.Rect{X: l.Content.X + 2, Y: l.Content.Y + 3, Width: 4, Height: 1}

	f := New(13)
	f.Step(Options{Layout: l, Phase: contracts.PhaseIdle, Motion: true, Burst: 6, BurstAt: origin})

	found := 0
	for _, p := range f.particles {
		if int(p.Y) == origin.Y && int(p.X) >= origin.X && int(p.X) < origin.X+origin.Width {
			found++
		}
	}
	if found < 6 {
		t.Fatalf("only %d of 6 burst particles came from the origin", found)
	}
}

// A layout with no incense - a terminal too short for it - must not emit.
func TestNoIncenseNoSmoke(t *testing.T) {
	l := layout.Compute(30, 9)
	if l.Incense.Height != 0 {
		t.Skip("this size does have incense")
	}

	f := New(17)
	for i := 0; i < 50; i++ {
		f.Step(Options{Layout: l, Phase: contracts.PhaseWorking, Motion: true})
	}
	if f.Len() != 0 {
		t.Fatalf("%d particles with no incense to burn", f.Len())
	}
}

// The renderer takes what this produces, so the shapes have to line up.
func TestRenderProducesCanvasParticles(t *testing.T) {
	l := layout.Compute(56, 18)
	f := run(19, 30, contracts.PhaseWorking, true)

	var _ []render.Particle = f.Render(l)

	for _, p := range f.Render(l) {
		if p.Glyph == 0 {
			t.Fatalf("particle with no glyph: %+v", p)
		}
	}
}
