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

// PrayerShows is how long after a prayer the emoji keep appearing. The status
// line refreshes about once a second, so this is also roughly the frame count.
const PrayerShows = 5 * time.Second

// MaxAura is how many cells of light or shade can be on screen at once.
const MaxAura = 16

// Theme names the picture the status line draws.
type Theme string

const (
	// ThemeCenser is the incense burner: prayers pile onto it as offerings.
	ThemeCenser Theme = "censer"
	// ThemeLamp is the genie lamp: a prayer rubs it, and the wish either
	// works or coughs soot.
	ThemeLamp Theme = "lamp"
	// ThemeHolder is the incense stick holder: the stick burns down as the
	// turn runs, so the picture reports rather than decorates.
	ThemeHolder Theme = "holder"
)

// ThemeFor resolves a stored name, falling back to the censer so an unknown
// or empty value draws something rather than nothing.
func ThemeFor(name string) Theme {
	switch Theme(name) {
	case ThemeLamp:
		return ThemeLamp
	case ThemeHolder:
		return ThemeHolder
	}
	return ThemeCenser
}

// SceneOptions control one rendered scene.
type SceneOptions struct {
	Now     time.Time
	Columns int
	Color   bool
	Motion  bool
	Theme   Theme

	// Fuel is how much of the context window is left, 0 to 100. Nil means the
	// host did not say, and nil is the zero value on purpose: an int would
	// make "unset" and "empty" the same thing, and every caller that forgot
	// the field would draw a stick that had already burned away.
	Fuel *int
}

// fuel resolves the gauge, falling back to the clock when the host reported
// nothing. Without the fallback a host that sends no context figure would
// leave every stick full for ever.
func fuel(state contracts.SessionState, opts SceneOptions) int {
	if opts.Fuel != nil {
		return *opts.Fuel
	}
	return Remaining(state, opts.Now)
}

// Scene renders the censer, its smoke, any prayers, and the status text.
//
// It is a pure function of the state and the clock: the same second renders
// the same picture, which keeps the status line from flickering between two
// refreshes that happen to land close together.
func Scene(state contracts.SessionState, opts SceneOptions) []string {
	switch opts.Theme {
	case ThemeLamp:
		return LampScene(state, opts)
	case ThemeHolder:
		return HolderScene(state, opts)
	}

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
		g.paint(column, smokeRows, Ember, ansiEmberLit)
	}
	for i, row := range censer {
		g.text(0, smokeRows+emberRow+i, row)
	}
	drawAura(g, state, opts.Now, frame)

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

// Light and shade, rather than a crowd of emoji.
//
// The burst used to be prayer emoji flying outward. It read as confetti: busy,
// and it said the same thing whatever had happened. Light and shade say which
// it was without a word - and they carry it in weight as well as in colour, so
// they still read on a terminal whose palette is nothing like this one.
const (
	ansiHalo  = "\x1b[38;5;222m"
	ansiShade = "\x1b[38;5;240m"
)

// Halo is the light a finished turn throws around the censer; Shade is what a
// failed one leaves.
//
// Neither set may borrow a glyph from the smoke. The first version used ░ for
// light and ▒ for shade, which are exactly what the incense is already making
// - so the two were told apart by colour alone, and on a terminal with a
// different palette they were not told apart at all.
var (
	haloGlyphs  = []rune{'·', '˚'}
	shadeGlyphs = []rune{'▓', '▚'}
)

// drawAura surrounds the censer with light or with shadow.
//
// Light spreads: it starts close and travels outward, thinning as it goes.
// Shade does the opposite - it presses in around the censer and stays there,
// which is what makes the two obvious at a glance even in one frame.
func drawAura(g *grid, state contracts.SessionState, now time.Time, frame uint64) {
	elapsed, active := effectAge(state, now)
	if !active {
		return
	}

	shows := int64(PrayerShows / time.Second)
	progress := float64(elapsed) / float64(shows)
	good := granted(state)

	count := int(float64(MaxAura)*(1-progress)) + 1
	glyphs := shadeGlyphs
	ansi := ansiShade
	if good {
		glyphs = haloGlyphs
		ansi = ansiHalo
	}

	placed := 0
	for attempt := 0; attempt < 400 && placed < count; attempt++ {
		y := int(effect.Hash01(frame, attempt, placed, "auraY") * float64(len(g.cells)))

		// Light travels outward as it goes; shade never leaves the censer.
		// Both start against the censer. Light then reaches further out every
		// second; shade never does.
		far := CenserWidth + 8
		if good {
			far = CenserWidth + 8 + int(progress*float64(g.width-CenserWidth))
		}
		x := int(effect.Hash01(frame, attempt, placed, "auraX") * float64(far))

		if x < 0 || x >= g.width || y < 0 || y >= len(g.cells) {
			continue
		}
		// Never inside the censer. Filling the gaps between its own glyphs
		// reads as static on the object rather than an aura around it.
		if x < CenserWidth && y >= smokeRows+emberRow {
			continue
		}
		if g.cells[y][x] != ' ' {
			continue
		}

		glyph := glyphs[int(effect.Hash01(frame, attempt, placed, "auraG")*float64(len(glyphs)))]
		g.paint(x, y, glyph, ansi)
		placed++
	}
}

// grid is a fixed block of cells the scene is composed into.
type grid struct {
	cells [][]rune
	width int

	// tint colours individual cells. A whole row cannot be wrapped: the ember
	// is red and the censer beneath it is not, and the lamp's smoke changes
	// colour while the lamp does not.
	tint map[int]string
}

func newGrid(width, height int) *grid {
	g := &grid{width: width, cells: make([][]rune, height), tint: map[int]string{}}
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

// paint sets a cell and the colour it is drawn in.
func (g *grid) paint(x, y int, r rune, ansi string) {
	if y < 0 || y >= len(g.cells) || x < 0 || x >= g.width {
		return
	}
	g.cells[y][x] = r
	g.tint[y*g.width+x] = ansi
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

// paintText writes a string in one colour, cell by cell, so the escapes never
// wrap a space the row's indent depends on.
func (g *grid) paintText(x, y int, s string, ansi string) {
	for _, r := range s {
		if r != ' ' {
			g.paint(x, y, r, ansi)
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

// lines renders the grid. Colour is written around single cells rather than
// around a row: the ember is red while the censer under it is not, and the
// lamp's smoke changes colour while the lamp does not.
func (g *grid) lines(color bool) []string {
	out := make([]string, 0, len(g.cells))
	for y, row := range g.cells {
		var b strings.Builder

		// Runs of one colour open and close once. Per cell would work too, but
		// the lamp is two dozen cells of the same brass and the status line
		// redraws every second.
		open := ""
		for x, r := range row {
			if r == covered {
				continue
			}

			// An unpainted cell keeps the terminal's own foreground, which is
			// the only colour guaranteed to contrast with its background.
			want := ""
			if color {
				want = g.tint[y*g.width+x]
			}
			if want != open {
				if open != "" {
					b.WriteString(ansiReset)
				}
				b.WriteString(want)
				open = want
			}
			b.WriteRune(r)
		}
		if open != "" {
			b.WriteString(ansiReset)
		}

		// Trimming has to ignore the escapes, which never end a row anyway:
		// only spaces do, and a space is never painted, so TrimRight sees them
		// plainly.
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

// CenserRows returns the censer's rows, for callers that check the art
// against what is published rather than redrawing it by hand.
func CenserRows() []string { return append([]string(nil), censer...) }

// SceneHeight is how many rows the scene draws above the status text. A
// published copy has to match it exactly: comparing only the last rows lets a
// dropped row slide the whole comparison along and match anyway.
func SceneHeight() int { return smokeRows + emberRow + len(censer) }

// Indent is exported for the same reason: a published copy of the art has to
// carry the same leading blanks the status line emits.
func Indent(row string) string { return indent(row) }
