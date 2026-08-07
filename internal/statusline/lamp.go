package statusline

import (
	"time"

	"github.com/cruellaDev/claude-code-prayops/contracts"
	"github.com/cruellaDev/claude-code-prayops/internal/effect"
)

// The lamp is the second theme. Where the censer answers a prayer with an
// offering, the lamp answers it with a wish - and a wish can fail.
//
// The spout points right and lifts at the tip, the handle curls on the left,
// a finial sits on the lid and the whole thing stands on a foot. That is the
// silhouette every reference the user picked shares.
var lamp = []string{
	"          ▄                ",
	"         ███               ",
	" ▗▄▖   ▗███████▖▄▄▄▄▄▄▄▄▄▄▖",
	" ▐ ▌  ▟████████████████▛▀▘ ",
	"  ▀   ▜██████████▛▀▀▀▘     ",
	"        ▝▀█████▀▘          ",
	"        ▄▄▄███▄▄▄          ",
}

// LampWidth is the width of every row of the lamp, in cells.
const LampWidth = 27

// lampSmokeRows is how many rows rise above the spout. Three, because the
// wish's smoke has to zigzag and one row cannot show a zigzag.
const lampSmokeRows = 3

// spoutTip is the column the smoke leaves from - the lifted end of the spout.
const spoutTip = 25

// lampPlume is where idle smoke sits: fixed columns fanning out from the
// spout, never moving. Only the density changes.
//
// Moving smoke has been written three times in this project and taken out
// three times: a plume whose footprint changes every second, directly above
// the line the user types on, reads as flicker.
var lampPlume = [][]int{
	{spoutTip - 2, spoutTip, spoutTip + 2},
	{spoutTip - 1, spoutTip + 1},
	{spoutTip},
}

// A wish takes a moment to work, and then either works or does not.
const (
	rubSeconds  = 2
	grantChance = 0.7
)

// The lamp's own colours.
//
// The lamp is brass, which is what every reference the user picked is made
// of, and a lamp is not anyone's intellectual property: the story is a few
// centuries older than any film of it. What would be someone's is a named
// character or a studio's particular artwork, and neither is here.
//
// The body is the duller gold so the granted smoke, a brighter one, still
// reads as something leaving the lamp rather than more of it.
const (
	ansiGold = "\x1b[38;5;220m"
	ansiSoot = "\x1b[38;5;240m"
)

// brass is a ramp rather than one colour, so the lamp reads as a round object
// instead of a flat cut-out. Light comes from the upper left, which is where
// it comes from in every one of the references.
var brass = []string{
	"\x1b[38;5;229m", // 0 lit edge
	"\x1b[38;5;221m", // 1
	"\x1b[38;5;178m", // 2 the body's own colour
	"\x1b[38;5;136m", // 3
	"\x1b[38;5;94m",  // 4 deep shadow
}

// shadeFor picks a step of the ramp for one cell of the lamp.
//
// Two gradients at once: down the rows, because the top faces the light, and
// across the columns, because the spout recedes to the right. A single
// vertical ramp made it look like a stack of bars rather than a lamp.
func shadeFor(row, column int) string {
	shade := []int{0, 1, 1, 2, 3, 3, 2}[row]

	switch {
	case column < 12:
		shade--
	case column > 18:
		shade++
	}

	if shade < 0 {
		shade = 0
	}
	if shade >= len(brass) {
		shade = len(brass) - 1
	}
	return brass[shade]
}

// Rub is the hand on the lamp's belly.
const Rub = "✋"

// LampScene renders the lamp, its smoke, whatever a prayer is doing, and the
// status text.
func LampScene(state contracts.SessionState, opts SceneOptions) []string {
	columns := opts.Columns
	if columns <= 0 {
		columns = DefaultColumns
	}
	if columns < LampWidth+2 {
		return nil
	}

	g := newGrid(columns, lampSmokeRows+len(lamp))

	frame := uint64(opts.Now.Unix())
	if !opts.Motion {
		frame = 0
	}

	elapsed, wishing := wishAge(state, opts.Now)

	// The lamp keeps smoking while it is being rubbed - and the rows must
	// hold something in every frame anyway, or the scene loses height and the
	// prompt underneath moves.
	if !wishing || elapsed < rubSeconds {
		drawPlume(g, frame, burning(phaseOf(state)))
	}
	for i, row := range lamp {
		x := 0
		for _, r := range row {
			if r != ' ' {
				g.paint(x, lampSmokeRows+i, r, shadeFor(i, x))
			}
			x++
		}
	}

	label := ""
	if wishing {
		label = drawWish(g, state, elapsed, frame)
	}

	lines := g.lines(opts.Color)
	if label != "" {
		return append(lines, colorize(label, phaseOf(state), opts.Color))
	}
	return append(lines, Render(state, Options{
		Now: opts.Now, Columns: columns, Color: opts.Color,
	}))
}

// LampHeight is how many rows the lamp scene draws above the status text.
func LampHeight() int { return lampSmokeRows + len(lamp) }

// LampRows returns the lamp's rows, for callers that check the art against
// what is published rather than redrawing it by hand.
func LampRows() []string { return append([]string(nil), lamp...) }

// wishAge reports how many whole seconds ago the prayer was, and whether the
// effect is still running.
//
// Whole seconds, not the exact age: the status line re-runs on events as well
// as on its timer, and a frame derived from a fractional age would redraw a
// different picture twice within the same second.
func wishAge(state contracts.SessionState, now time.Time) (int64, bool) {
	if state.LastPrayerAt.IsZero() {
		return 0, false
	}
	elapsed := now.Unix() - state.LastPrayerAt.Unix()
	shows := int64(PrayerShows / time.Second)
	return elapsed, elapsed >= 0 && elapsed < shows
}

// granted decides whether a wish works. The seed is the prayer, so the answer
// holds for the whole effect instead of changing on every refresh.
func granted(state contracts.SessionState) bool {
	return effect.Hash01(uint64(state.LastPrayerAt.Unix()), 0, 0, "grant") < grantChance
}

// drawPlume lifts smoke off the spout. Fixed columns, varying density.
func drawPlume(g *grid, frame uint64, active bool) {
	dense := 0.25
	if active {
		dense = 0.55
	}

	for row, columns := range lampPlume {
		for i, column := range columns {
			glyph := '░'
			if effect.Hash01(frame, row*8+i, 0, "puff") < dense {
				glyph = '▒'
			}
			g.set(column, row, glyph)
		}
	}
}

// drawWish plays the prayer: a hand on the belly first, then the answer. It
// returns the status text to show while it runs.
func drawWish(g *grid, state contracts.SessionState, elapsed int64, frame uint64) string {
	if elapsed < rubSeconds {
		// The hand works back and forth across the belly.
		x := 9 + int(elapsed%2)*3
		g.text(x, lampSmokeRows+3, Rub)
		return "PrayOps · RUBBING…"
	}

	if granted(state) {
		drawGrantedSmoke(g, elapsed, frame)
		return "PrayOps · GRANTED"
	}
	drawSootSmoke(g, elapsed, frame)
	return "PrayOps · NOTHING"
}

// drawGrantedSmoke sends a gold ribbon up from the spout, leaning one way then
// the other so it reads as a zigzag rather than a column.
func drawGrantedSmoke(g *grid, elapsed int64, frame uint64) {
	step := elapsed - rubSeconds

	for row := 0; row < lampSmokeRows; row++ {
		// The higher the row, the further the ribbon has travelled, and the
		// phase shifts with the frame so it appears to flow.
		lean := int((step+int64(row))%2)*2 - 1
		x := spoutTip + lean*(row+1)

		g.paint(x, lampSmokeRows-1-row, '▒', ansiGold)
		if row > 0 {
			g.paint(x-lean, lampSmokeRows-1-row, '░', ansiGold)
		}
	}

	// A spark at the mouth, so the ribbon is clearly leaving the lamp.
	g.paint(spoutTip, lampSmokeRows-1, '▓', ansiGold)
	_ = frame
}

// drawSootSmoke coughs a dark cloud out of the spout: dense at the mouth,
// scattering outward and thinning as it goes.
func drawSootSmoke(g *grid, elapsed int64, frame uint64) {
	step := int(elapsed-rubSeconds) + 1

	for row := 0; row < lampSmokeRows; row++ {
		spread := step + row
		for i := -spread; i <= spread; i++ {
			// Not every cell: a solid block would read as a wall, not a cough.
			if effect.Hash01(frame, row*32+i+16, 0, "soot") > 0.55 {
				continue
			}
			glyph := '▓'
			if row > 0 || i < -1 || i > 1 {
				glyph = '▒'
			}
			g.paint(spoutTip+i, lampSmokeRows-1-row, glyph, ansiSoot)
		}
	}
}
