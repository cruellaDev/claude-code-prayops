// Package smoke is the incense particle field.
//
// It owns its own random source rather than using the global one, so a field
// is reproducible from its seed and can be stepped in a test without a clock
// or a terminal.
package smoke

import (
	"math/rand/v2"

	"github.com/cruellaDev/claude-code-prayops/contracts"
	"github.com/cruellaDev/claude-code-prayops/internal/render"
)

// glyphs are the shapes a particle takes as it ages and thins out.
var glyphs = []rune{'.', '˙', '~', '·'}

// Particle is one puff of smoke. Position is fractional so a particle can
// drift slower than one cell per frame.
type Particle struct {
	X, Y   float64
	VX, VY float64
	Age    int
	Life   int
	Glyph  rune
}

// maxParticles bounds the field. Long idle runs would otherwise accumulate.
const maxParticles = 96

// Field is the live particle set.
type Field struct {
	particles []Particle
	rng       *rand.Rand
}

// New returns an empty field driven by the given seed.
func New(seed uint64) *Field {
	return &Field{rng: rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))}
}

// density is how many particles a phase emits per step, scaled by 100.
//
// The incense burns harder while Claude is working and settles once the turn
// is over, so the scene reads as activity without any text.
func density(phase contracts.SessionPhase) int {
	switch phase {
	case contracts.PhaseWorking:
		return 90
	case contracts.PhaseThinking:
		return 55
	case contracts.PhaseApprovalRequired:
		return 35
	case contracts.PhaseTurnCompleted, contracts.PhaseTurnFailed:
		return 20
	case contracts.PhaseSessionEnded:
		return 0
	default:
		return 25
	}
}

// Options control one step.
type Options struct {
	Layout contracts.Layout
	Phase  contracts.SessionPhase

	// Motion off keeps the scene legible for reduced-motion users: particles
	// still appear and expire, but they do not drift.
	Motion bool

	// Burst adds particles for one step, used when a prayer appears.
	Burst int
	// BurstAt is where a burst originates. Zero means the incense tips.
	BurstAt contracts.Rect
}

// Step advances the field by one frame.
func (f *Field) Step(opts Options) {
	f.age(opts.Motion)
	f.emit(opts)

	if len(f.particles) > maxParticles {
		f.particles = f.particles[len(f.particles)-maxParticles:]
	}
}

func (f *Field) age(motion bool) {
	alive := f.particles[:0]

	for _, p := range f.particles {
		p.Age++
		if p.Age >= p.Life {
			continue
		}
		if motion {
			p.X += p.VX
			p.Y += p.VY
		}
		p.Glyph = glyphs[min(p.Age*len(glyphs)/max(p.Life, 1), len(glyphs)-1)]
		alive = append(alive, p)
	}
	f.particles = alive
}

func (f *Field) emit(opts Options) {
	l := opts.Layout
	if l.Incense.Width <= 0 || l.Incense.Height <= 0 {
		return
	}

	rate := density(opts.Phase)
	if !opts.Motion {
		// Reduced motion still shows smoke, just far less of it.
		rate /= 4
	}
	if rate > 0 && f.rng.IntN(100) < rate {
		f.spawn(float64(f.tipX(l)), float64(l.Incense.Y), opts.Motion)
	}

	origin := opts.BurstAt
	if origin.Width == 0 {
		origin = l.Incense
	}
	for i := 0; i < opts.Burst; i++ {
		x := origin.X + f.rng.IntN(max(origin.Width, 1))
		f.spawn(float64(x), float64(origin.Y), opts.Motion)
	}
}

// tipX picks one of the three incense tips.
func (f *Field) tipX(l contracts.Layout) int {
	tips := (l.Incense.Width + 1) / 2
	return l.Incense.X + f.rng.IntN(max(tips, 1))*2
}

func (f *Field) spawn(x, y float64, motion bool) {
	life := 6 + f.rng.IntN(10)
	p := Particle{X: x, Y: y, Life: life, Glyph: glyphs[0]}

	if motion {
		p.VY = -0.35 - f.rng.Float64()*0.25
		p.VX = (f.rng.Float64() - 0.5) * 0.5
	}
	f.particles = append(f.particles, p)
}

// Render returns the particles as canvas cells, clipped to the scene.
//
// Particles that drift out of the scene are still kept in the field until they
// expire; clipping here rather than deleting keeps the drift honest instead of
// making particles vanish at the edge and reappear.
func (f *Field) Render(l contracts.Layout) []render.Particle {
	out := make([]render.Particle, 0, len(f.particles))

	top := l.Content.Y
	bottom := l.Altar.Y
	for _, p := range f.particles {
		x, y := int(p.X+0.5), int(p.Y+0.5)
		if y < top || y >= bottom {
			continue
		}
		if x < l.Content.X || x >= l.Content.X+l.Content.Width {
			continue
		}
		out = append(out, render.Particle{X: x, Y: y, Glyph: p.Glyph})
	}
	return out
}

// Len is how many particles are alive, including any drifted out of view.
func (f *Field) Len() int { return len(f.particles) }
