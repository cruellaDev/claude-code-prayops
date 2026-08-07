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

// lampSmokeRows is how many rows rise above the spout.
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

// A wish either works or it does not, and it says so at once.
//
// A hand rubbed the belly for the first two seconds at one point. It read as a
// loading bar in front of the thing the user asked for, so it is gone.
const grantChance = 0.7

// The lamp is drawn the way the cartoon reference is: an outline holding the
// shape, and flat gold inside it.
//
// The art separates the two by itself: a full block is inside the lamp, and
// every partial block - the quadrants and half blocks - is an edge of it.
//
// The outline is left uncoloured on purpose. Nearly half the lamp's cells are
// edges - the whole spout, the handle, the foot - so whatever colour they get,
// the object mostly is that colour: brown made it look like a chestnut, and
// near black made it vanish on a dark background. Unpainted, it takes the
// terminal's own foreground, which contrasts with the terminal's own
// background by definition. That is why the censer has always looked right in
// both.
//
// The fill was shaded for a while, in five steps and then three. Neither read:
// twenty-seven cells across, one face of the body is five or six cells, so the
// steps land a cell each and the gradient becomes a smudge.
const (
	ansiLampOutline = ""
	ansiLampFill    = "\x1b[38;5;220m"
)

// A wish that lands rises as hearts; one that does not goes off like a
// firework that misfired inside the lamp.
//
// The hearts are red, which is red on any background. The sparks are left
// unpainted, for the same reason the outline is: a fixed dark grey is soot on
// a pale terminal and nearly invisible on a dark one. Unpainted they take the
// terminal's own foreground - black sparks on a light background, bright ones
// on a dark background, which is what a firework looks like at night anyway.
const (
	ansiHeart = "\x1b[38;5;203m"
	ansiSoot  = ""
)

// Heart is what a granted wish rises as.
const Heart = '♥'

// Spark is one ember of the firework a failed wish throws.
const Spark = '▪'

// burstSpokes are the directions the firework throws its sparks. Up and out
// only: the rows below belong to the lamp.
var burstSpokes = [][2]int{
	{-3, 0}, {3, 0},
	{-2, -1}, {2, -1}, {0, -1},
	{-1, -1}, {1, -1}, {0, -2},
}

// lampBodyRows lists the rows that hold oil, bottom first. The lamp fills from
// the foot up, so the gold has a waterline rather than a scatter.
var lampBodyRows = []int{6, 5, 4, 3, 2, 1}

// shadeFor picks the outline for an edge and the fill for the inside.
//
// The inside is only gold as far up as the oil goes. Above the waterline the
// cell is left unpainted, which is plain metal in whatever colour the terminal
// draws its text - the lamp keeps its shape and loses its shine, which is what
// a lamp running out of oil looks like.
func shadeFor(r rune, row, remaining int) string {
	if r != '█' {
		return ansiLampOutline
	}

	filled := remaining * len(lampBodyRows) / 100
	for _, lit := range lampBodyRows[:filled] {
		if lit == row {
			return ansiLampFill
		}
	}
	return ansiLampOutline
}

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

	elapsed, wishing := effectAge(state, opts.Now)

	// The plume is the lamp at rest. While a wish plays, the answer fills those
	// rows instead - and it always fills them, because a row left empty costs
	// the scene a line of height and moves the prompt underneath.
	if !wishing {
		drawPlume(g, frame, burning(phaseOf(state)))
	}
	oil := fuel(state, opts)
	for i, row := range lamp {
		x := 0
		for _, r := range row {
			if r != ' ' {
				g.paint(x, lampSmokeRows+i, r, shadeFor(r, i, oil))
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

// effectAge reports how many whole seconds ago the effect started, and whether
// it is still running.
//
// Whole seconds, not the exact age: the status line re-runs on events as well
// as on its timer, and a frame derived from a fractional age would redraw a
// different picture twice within the same second.
func effectAge(state contracts.SessionState, now time.Time) (int64, bool) {
	at, _, ok := effectStart(state)
	if !ok {
		return 0, false
	}

	elapsed := now.Unix() - at.Unix()
	shows := int64(PrayerShows / time.Second)
	return elapsed, elapsed >= 0 && elapsed < shows
}

// effectStart returns the moment the current effect began, whether it was
// asked for, and whether there is one at all.
//
// Two things can set it off. A prayer is asked for, and its answer is chance.
// A turn ending is not asked for, and its answer is the truth: the work either
// finished or it failed. The more recent of the two wins, so a prayer sent
// during a turn is not overruled the instant the turn ends.
//
// Reacting to the turn is what makes this feel immediate. A slash command
// costs a whole model turn before anything can be drawn; a hook records the
// end of a turn straight away, and the status line has it within the second.
func effectStart(state contracts.SessionState) (time.Time, bool, bool) {
	prayer := state.LastPrayerAt

	var turn time.Time
	switch state.Phase {
	case contracts.PhaseTurnCompleted, contracts.PhaseTurnFailed:
		turn = state.UpdatedAt
	}

	switch {
	case !prayer.IsZero() && prayer.After(turn):
		return prayer, true, true
	case !turn.IsZero():
		return turn, false, true
	}
	return time.Time{}, false, false
}

// granted decides whether the lamp answers.
//
// A turn that finished is granted and one that failed is not - that is not a
// wish, it is a report. Only a prayer is left to chance, and its seed is the
// prayer itself, so the answer holds for the whole effect instead of changing
// on every refresh.
func granted(state contracts.SessionState) bool {
	at, asked, ok := effectStart(state)
	if !ok {
		return false
	}
	if !asked {
		return state.Phase == contracts.PhaseTurnCompleted
	}
	return effect.Hash01(uint64(at.Unix()), 0, 0, "grant") < grantChance
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

// drawWish plays the answer and returns the status text to show while it runs.
func drawWish(g *grid, state contracts.SessionState, elapsed int64, frame uint64) string {
	if granted(state) {
		drawHearts(g, elapsed, frame)
		return "PrayOps · GRANTED"
	}
	drawBurst(g, elapsed, frame)
	return "PrayOps · NOTHING"
}

// drawHearts lifts hearts out of the spout, spreading as they rise and
// thinning as the wish settles.
func drawHearts(g *grid, elapsed int64, frame uint64) {
	shows := int64(PrayerShows / time.Second)
	left := float64(shows-elapsed) / float64(shows)
	count := int(10*left) + 1

	placed := 0
	for attempt := 0; attempt < 200 && placed < count; attempt++ {
		row := int(effect.Hash01(frame, attempt, placed, "heartRow") * float64(lampSmokeRows))

		// They drift up and out: the higher the row, the wider the scatter.
		reach := 2 + row*3
		x := spoutTip - reach + int(effect.Hash01(frame, attempt, placed, "heartX")*float64(reach*2+1))

		if x < 0 || x >= g.width || g.cells[lampSmokeRows-1-row][x] != ' ' {
			continue
		}
		g.paint(x, lampSmokeRows-1-row, Heart, ansiHeart)
		placed++
	}

	fillEmptyRows(g, lampSmokeRows, Heart, ansiHeart)
}

// drawBurst throws a firework out of the spout: a core that flies apart into
// sparks, wider and sparser every second.
func drawBurst(g *grid, elapsed int64, frame uint64) {
	radius := int(elapsed) + 1

	// The core, so the first frame reads as a bang rather than a drizzle.
	if elapsed == 0 {
		for _, dx := range []int{-1, 0, 1} {
			g.paint(spoutTip+dx, lampSmokeRows-1, '▓', ansiSoot)
		}
	}

	for i, spoke := range burstSpokes {
		for r := 1; r <= radius; r++ {
			// The tail thins out behind the head, so the burst looks like it is
			// travelling rather than growing a solid shape.
			if r < radius && effect.Hash01(frame, i*8+r, 0, "spark") > 0.4 {
				continue
			}

			x := spoutTip + spoke[0]*r
			y := lampSmokeRows - 1 + spoke[1]*r
			if y < 0 || y >= lampSmokeRows {
				continue
			}

			glyph := Spark
			if r < radius {
				glyph = '·'
			}
			g.paint(x, y, glyph, ansiSoot)
		}
	}

	fillEmptyRows(g, lampSmokeRows, Spark, ansiSoot)
}

// fillEmptyRows puts one glyph on any of the top rows a scene left blank.
//
// A blank row is not a cosmetic problem: the host drops it, the scene loses a
// line of height, and the prompt underneath jumps. Every effect that scatters
// rather than drawing a fixed shape can leave one empty by chance, so they all
// come through here.
func fillEmptyRows(g *grid, rows int, glyph rune, ansi string) {
	for y := 0; y < rows && y < len(g.cells); y++ {
		empty := true
		for _, r := range g.cells[y] {
			if r != ' ' {
				empty = false
				break
			}
		}
		if empty {
			g.paint(spoutTip, y, glyph, ansi)
		}
	}
}
