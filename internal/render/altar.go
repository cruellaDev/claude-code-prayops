package render

import (
	"github.com/cruellaDev/claude-code-prayops/contracts"
)

// Title is the name drawn into the top border.
const Title = "PrayOps"

// phrases are the ceremonial status lines. The status line uses short labels;
// the full altar has room for these.
var phrases = map[contracts.SessionPhase]string{
	contracts.PhaseIdle:             "기술적 조치를 기다리고 있습니다",
	contracts.PhaseThinking:         "기도를 정리하고 있습니다",
	contracts.PhaseWorking:          "정성을 들이고 있습니다",
	contracts.PhaseApprovalRequired: "인간의 결단이 필요합니다",
	contracts.PhaseTurnCompleted:    "한 차례 기도가 완료되었습니다",
	contracts.PhaseTurnFailed:       "기도는 충분했습니다. 상태를 확인하십시오",
	contracts.PhaseSessionEnded:     "의식을 마쳤습니다",
}

// Phrase returns the ceremonial line for a phase.
func Phrase(phase contracts.SessionPhase) string {
	if phrase, ok := phrases[phase]; ok {
		return phrase
	}
	return phrases[contracts.PhaseIdle]
}

// Options control what a frame includes.
type Options struct {
	// Smoke is the particle field to draw, if any.
	Smoke []Particle
}

// Particle is one smoke cell.
type Particle struct {
	X     int
	Y     int
	Glyph rune
}

// Frame draws one complete altar.
func Frame(l contracts.Layout, state contracts.SessionState, opts Options) string {
	c := NewCanvas(l.Width, l.Height)
	if l.Width < 2 || l.Height < 2 {
		return c.String()
	}

	c.Border()
	c.Title(Title)

	// Smoke goes down first so the furniture always draws over it, and never
	// inside the burner: the burner's interior is mostly blank cells, so a
	// particle there would show through the vessel rather than behind it.
	burner := l.Burner
	for _, p := range opts.Smoke {
		if p.Y <= l.Content.Y-1 || p.Y >= l.Altar.Y {
			continue
		}
		if (contracts.Rect{X: p.X, Y: p.Y, Width: 1, Height: 1}).Intersects(burner) {
			continue
		}
		c.Set(p.X, p.Y, p.Glyph)
	}

	drawIncense(c, l)
	drawBurner(c, l)
	drawPlates(c, l)
	drawAltar(c, l)
	drawStatus(c, l, state)

	return c.String()
}

func drawIncense(c *Canvas, l contracts.Layout) {
	if l.Incense.Width <= 0 || l.Incense.Height <= 0 {
		return
	}

	for i := 0; i < l.Incense.Width; i += 2 {
		x := l.Incense.X + i
		for y := l.Incense.Y; y < l.Incense.Y+l.Incense.Height; y++ {
			c.Set(x, y, '│')
		}
		// The ember sits at the tip.
		c.Set(x, l.Incense.Y, '╹')
	}
}

func drawBurner(c *Canvas, l contracts.Layout) {
	if l.Burner.Width < 2 || l.Burner.Height < 2 {
		return
	}

	right := l.Burner.X + l.Burner.Width - 1
	bottom := l.Burner.Y + l.Burner.Height - 1

	c.HLine(l.Burner.X+1, l.Burner.Y, l.Burner.Width-2, '─')
	c.HLine(l.Burner.X+1, bottom, l.Burner.Width-2, '─')
	for y := l.Burner.Y + 1; y < bottom; y++ {
		c.Set(l.Burner.X, y, '│')
		c.Set(right, y, '│')
	}
	c.Set(l.Burner.X, l.Burner.Y, '┌')
	c.Set(right, l.Burner.Y, '┐')
	c.Set(l.Burner.X, bottom, '└')
	c.Set(right, bottom, '┘')

	// Ash inside the burner.
	if l.Burner.Height > 2 {
		c.HLine(l.Burner.X+2, bottom-1, max(l.Burner.Width-4, 0), '░')
	}
}

func drawPlates(c *Canvas, l contracts.Layout) {
	const plate = "(____)"
	if l.Plates.Width < narrow.StringWidth(plate) {
		return
	}

	gap := 2
	unit := narrow.StringWidth(plate) + gap
	for i := 0; i*unit+narrow.StringWidth(plate) <= l.Plates.Width; i++ {
		c.Text(l.Plates.X+i*unit, l.Plates.Y, plate)
	}
}

func drawAltar(c *Canvas, l contracts.Layout) {
	if l.Altar.Width <= 0 {
		return
	}
	// The surface is inset so it reads as a table rather than a rule across
	// the whole frame.
	inset := min(l.Altar.Width/8, 6)
	c.HLine(l.Altar.X+inset, l.Altar.Y, l.Altar.Width-2*inset, '─')
}

func drawStatus(c *Canvas, l contracts.Layout, state contracts.SessionState) {
	if l.Status.Width <= 0 || l.Status.Height <= 0 {
		return
	}

	phase := state.Phase
	if _, known := phrases[phase]; !known {
		phase = contracts.PhaseIdle
	}

	label := string(phase)
	if state.ToolErrored {
		label += "!"
	}

	line := label
	if l.Mode != contracts.LayoutCompact {
		line += " · " + Phrase(phase)
	}
	c.Text(l.Status.X+1, l.Status.Y, truncate(line, l.Status.Width-2))

	if l.Status.Height > 1 {
		who := "Claude"
		if state.Host == contracts.HostCodex {
			who = "Codex"
		}
		if state.ProjectName != "" {
			who += " · " + state.ProjectName
		}
		c.Text(l.Status.X+1, l.Status.Y+1, truncate(who, l.Status.Width-2))
	}
}

// truncate cuts a string to a cell width, counting wide runes as two.
func truncate(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if narrow.StringWidth(text) <= width {
		return text
	}

	cells := 0
	for i, r := range text {
		w := max(narrow.RuneWidth(r), 1)
		if cells+w > width {
			return text[:i]
		}
		cells += w
	}
	return text
}
