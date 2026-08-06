package statusline

import (
	"strings"
	"time"

	"github.com/cruellaDev/claude-code-prayops/contracts"
	"github.com/cruellaDev/claude-code-prayops/internal/effect"
)

// The censer is drawn in block characters rather than box-drawing ones. Box
// drawing reads as a thin diagram; this is meant to look like pixels.
//
// It is a three-legged censer: a mouth with ash showing, a belly wider than
// the mouth, ears on both sides, three feet. Every row is centred on the same
// column - an earlier version had each row on its own axis, which read as a
// wobble rather than a shape.
// Glyphs are chosen for where they sit *inside* their cell, not just which
// cell they occupy. ▏ hugs the left edge, so sticks drawn with it stood a
// little under half a cell left of the embers above them; ▮ is centred. ▘ is
// the upper-left quadrant, so a row of them put the left foot outside the
// pedestal and the middle foot a quarter cell off axis - ▝ ▀ ▘ mirrors.
var censer = []string{
	"     ▮ ▮ ▮     ",
	"     ▮ ▮ ▮     ",
	"    ▗▄▄▄▄▄▖    ",
	" ▗▄▟███████▙▄▖ ",
	" ▝▀▜███████▛▀▘ ",
	"    ▝▀▀▀▀▀▘    ",
	"    ▝  ▀  ▘    ",
}

// CenserWidth is the width of every row of the censer, in cells.
const CenserWidth = 15

// emberRow sits on the tips of the incense, above the censer. It is the one
// warm thing in the scene, so it is the one thing drawn in red.
const emberRow = 1

// Ember is the lit tip of a stick of incense.
const Ember = '▪'

// smokeRows is how many rows of smoke rise above the embers.
//
// Two, so a puff can visibly climb from one to the other. One row could only
// change density, which reads as still. Both rows always draw something: the
// first version put puffs on random rows, and whenever the top one stayed
// empty it collapsed, taking a line of height with it and moving the prompt
// underneath - 23 frames out of 300.
const smokeRows = 2

// smokeColumns are where the sticks stand, so the smoke, the embers and the
// incense all share one set of columns.
var smokeColumns = []int{5, 7, 9}

// Prayer is the emoji a prayer appears as.
const Prayer = "🙏"

// PrayerShows is how long after a prayer the emoji keep appearing. The status
// line refreshes about once a second, so this is also roughly the frame count.
const PrayerShows = 5 * time.Second

// MaxPrayers is how many emoji can be on screen at once. A prayer should feel
// like a lot of them, not a polite few.
const MaxPrayers = 44

// SceneOptions control one rendered scene.
type SceneOptions struct {
	Now     time.Time
	Columns int
	Color   bool
	Motion  bool
}

// Scene renders the censer, its smoke, any prayers, and the status text.
//
// It is a pure function of the state and the clock: the same second renders
// the same picture, which keeps the status line from flickering between two
// refreshes that happen to land close together.
func Scene(state contracts.SessionState, opts SceneOptions) []string {
	columns := opts.Columns
	if columns <= 0 {
		columns = DefaultColumns
	}
	// Below this there is no room for a censer; the caller falls back to the
	// single line.
	if columns < CenserWidth+2 {
		return nil
	}

	height := smokeRows + emberRow + len(censer)
	g := newGrid(columns, height)

	// One frame per second. Anything finer would be invisible: the status line
	// cannot refresh faster than that.
	frame := uint64(opts.Now.Unix())
	if !opts.Motion {
		frame = 0
	}

	drawSmoke(g, frame, burning(phaseOf(state)))
	for _, column := range smokeColumns {
		g.set(column, smokeRows, Ember)
	}
	for i, row := range censer {
		g.text(0, smokeRows+emberRow+i, row)
	}
	drawPrayers(g, state, opts.Now, columns)

	lines := g.lines(opts.Color)
	return append(lines, Render(state, Options{
		Now: opts.Now, Columns: columns, Color: opts.Color,
	}))
}

// drawSmoke lifts puffs through the two rows above the censer.
//
// The puffs never move sideways. They used to drift by a hashed offset, which
// pulled the smoke off the censer's axis every second - movement in the corner
// of the eye, on a line the user is trying to type under. They climb instead:
// which column is high and which is low changes with the frame, so the smoke
// moves without the shape wandering.
func drawSmoke(g *grid, frame uint64, active bool) {
	// Both rows always hold exactly three puffs, on the columns the sticks
	// stand in. Only the density changes.
	//
	// Two earlier versions moved the puffs instead - first onto random rows,
	// then one column left or right. Both made the plume's footprint change
	// every second directly above the line the user types on, which is the
	// complaint that started all of this. Density alone is visible without
	// anything shifting: nothing to track, nothing to wobble.
	dense := 0.25
	if active {
		dense = 0.55
	}

	for i, column := range smokeColumns {
		g.set(column, smokeRows-1, '▒')

		glyph := '░'
		if effect.Hash01(frame, i, 0, "puff") < dense {
			glyph = '▒'
		}
		g.set(column, 0, glyph)
	}
}

// drawPrayers throws the emoji out of the censer and lets them fly.
//
// The trick is that the seed is the prayer, not the frame. Each emoji keeps
// its own direction and speed for the whole five seconds, so it travels
// outward frame by frame instead of teleporting to a fresh random spot every
// refresh - a cloud of dots that merely rearranges reads as noise, while the
// same dots moving away from a point read as a burst.
//
// They land anywhere, the censer included. Reserving its block kept the
// drawing tidy and made the burst look fenced off; offerings piling onto the
// censer is the point.
func drawPrayers(g *grid, state contracts.SessionState, now time.Time, columns int) {
	if state.LastPrayerAt.IsZero() {
		return
	}
	// Whole seconds, not the exact age: Claude Code re-runs the status line on
	// events as well as on its timer, and a position derived from a fractional
	// age would redraw a different picture twice within the same second.
	elapsed := now.Unix() - state.LastPrayerAt.Unix()
	shows := int64(PrayerShows / time.Second)
	if elapsed < 0 || elapsed >= shows {
		return
	}

	// One burst, one seed. Two prayers a second apart throw different sprays.
	seed := uint64(state.LastPrayerAt.Unix())
	age := float64(elapsed)
	progress := (age + 1) / float64(shows)

	rows := len(g.cells)
	taken := make(map[int]bool, MaxPrayers*2)

	for i := 0; i < MaxPrayers; i++ {
		// Each one burns out at its own moment, so the crowd thins unevenly
		// instead of all of them stepping back together.
		if age > 1+effect.Hash01(seed, i, 0, "life")*float64(shows-1) {
			continue
		}

		// The scene is six rows and most of a terminal across, so the travel
		// that reads as a burst is sideways. Vertically they only drift up a
		// row or so; anything more flies out of the top and is simply lost.
		reach := effect.Hash01(seed, i, 1, "reach") * float64(columns-CenserWidth)
		if effect.Hash01(seed, i, 2, "side") < 0.35 {
			reach = -reach / 3 // a few go the other way, past the censer
		}

		x := int(float64(CenserWidth)/2 + reach*progress)
		y := int(effect.Hash01(seed, i, 3, "row")*float64(rows) - age/2)
		if y < 0 {
			y = 0
		}

		if x < 0 || y >= rows || x+prayerCells > g.width {
			continue
		}
		// Two emoji in the same place would leave half a glyph behind, so
		// prayers give way to each other - and to nothing else.
		if taken[y*g.width+x] || taken[y*g.width+x+1] {
			continue
		}

		g.text(x, y, Prayer)
		taken[y*g.width+x] = true
		taken[y*g.width+x+1] = true
	}
}

// prayerCells is how many terminal cells the prayer emoji occupies.
const prayerCells = 2

// grid is a fixed block of cells the scene is composed into.
type grid struct {
	cells [][]rune
	width int
}

func newGrid(width, height int) *grid {
	g := &grid{width: width, cells: make([][]rune, height)}
	for y := range g.cells {
		g.cells[y] = make([]rune, width)
		for x := range g.cells[y] {
			g.cells[y][x] = ' '
		}
	}
	return g
}

func (g *grid) set(x, y int, r rune) {
	if y < 0 || y >= len(g.cells) || x < 0 || x >= g.width {
		return
	}
	g.cells[y][x] = r
}

// text writes a string, advancing by each rune's display width. The second
// cell of a wide rune is marked so nothing else is drawn into it.
func (g *grid) text(x, y int, s string) {
	for _, r := range s {
		g.set(x, y, r)
		if narrow.RuneWidth(r) == 2 {
			g.set(x+1, y, covered)
		}
		x += max(narrow.RuneWidth(r), 1)
	}
}

// covered marks the second cell of a wide rune.
const covered rune = 0

// free reports whether a run of cells is untouched, which is what keeps a
// prayer from landing on the censer.
func (g *grid) free(x, y, width int) bool {
	if y < 0 || y >= len(g.cells) {
		return false
	}
	for i := 0; i < width; i++ {
		if x+i < 0 || x+i >= g.width || g.cells[y][x+i] != ' ' {
			return false
		}
	}
	return true
}

// lines renders the grid. The ember is the one cell that carries its own
// colour, so the escape is written around that rune alone rather than around
// the row - anything wider would tint the censer too.
func (g *grid) lines(color bool) []string {
	out := make([]string, 0, len(g.cells))
	for _, row := range g.cells {
		var b strings.Builder
		for _, r := range row {
			switch {
			case r == covered:
			case r == Ember && color:
				b.WriteString(ansiEmberLit)
				b.WriteRune(r)
				b.WriteString(ansiReset)
			default:
				b.WriteRune(r)
			}
		}
		// Trimming has to ignore the escapes, which never end a row anyway:
		// only spaces do, and TrimRight sees them plainly.
		out = append(out, indent(strings.TrimRight(b.String(), " ")))
	}
	return out
}

// blank is a braille cell with no dots raised. It occupies one column and
// draws nothing, which is what a space does - except that it is not a space.
//
// Claude Code strips the leading whitespace from every row of a status line.
// With ordinary spaces the censer arrived with each row flush against the
// left edge, so the shape collapsed into a stack of bars no matter how
// carefully the art was centred.
const blank = '\u2800'

// indent replaces a row's leading spaces with blanks that survive the strip.
// Only the leading run: spaces inside a row are kept as they are, and those
// are not touched.
func indent(row string) string {
	runes := []rune(row)

	for i, r := range runes {
		if r != ' ' {
			for j := 0; j < i; j++ {
				runes[j] = blank
			}
			return string(runes)
		}
	}
	return row
}

// phaseOf resolves an unset phase to idle, the same way Render does.
func phaseOf(state contracts.SessionState) contracts.SessionPhase {
	if _, known := phaseLabels[state.Phase]; !known {
		return contracts.PhaseIdle
	}
	return state.Phase
}
