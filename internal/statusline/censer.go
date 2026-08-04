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
// The shape is a three-legged censer: a flared mouth with ash showing, a belly
// wider than the mouth, ears on both sides, and three feet.
var censer = []string{
	"   ▄▄▄▄▄▄▄▄▄▄▄▄▄",
	"  ▐▒▒▒▒▒▒▒▒▒▒▒▒▒▌",
	"▐▌▟███████████████▙▐▌",
	"  ▜█████████████▛",
	"   ▝▀▀▀▀▀▀▀▀▀▀▀▘",
	"    █▘   █    ▝█",
}

// incenseRow sits on the censer's mouth; smoke rises above it.
const (
	incenseRow = "     ▕▏ ▕▏ ▕▏"
	smokeRows  = 3
)

// CenserWidth is the widest row of the scene, in cells.
const CenserWidth = 21

// Prayer is the emoji a prayer appears as.
const Prayer = "🙏"

// PrayerShows is how long after a prayer the emoji keep appearing. The status
// line refreshes about once a second, so this is also roughly the frame count.
const PrayerShows = 5 * time.Second

// MaxPrayers is how many emoji can be on screen at once. A prayer should feel
// like a lot of them, not a polite few.
const MaxPrayers = 22

// smokeGlyphs thin out as smoke rises.
var smokeGlyphs = []rune{'░', '▒', '░', '·'}

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

	height := smokeRows + 1 + len(censer)
	g := newGrid(columns, height)

	// One frame per second. Anything finer would be invisible: the status line
	// cannot refresh faster than that.
	frame := uint64(opts.Now.Unix())
	if !opts.Motion {
		frame = 0
	}

	drawSmoke(g, frame, burning(phaseOf(state)))
	g.text(0, smokeRows, incenseRow)
	for i, row := range censer {
		g.text(0, smokeRows+1+i, row)
	}
	drawPrayers(g, state, opts.Now, frame)

	lines := g.lines()
	return append(lines, Render(state, Options{
		Now: opts.Now, Columns: columns, Color: opts.Color,
	}))
}

// drawSmoke lifts a few puffs above each stick. Positions come from a hash of
// the frame, so the smoke drifts without any state being carried between
// refreshes.
func drawSmoke(g *grid, frame uint64, active bool) {
	puffs := 3
	if active {
		puffs = 6
	}

	for i := 0; i < puffs; i++ {
		// Sticks stand at these columns of the incense row.
		column := []int{5, 8, 11}[i%3]
		drift := int(effect.Hash01(frame, i, 0, "drift")*5) - 2
		row := int(effect.Hash01(frame, i, 1, "rise") * float64(smokeRows))

		glyph := smokeGlyphs[min(row, len(smokeGlyphs)-1)]
		g.set(column+drift, smokeRows-1-row, glyph)
	}
}

// drawPrayers scatters emoji anywhere the censer is not, thinning out as the
// prayer ages.
func drawPrayers(g *grid, state contracts.SessionState, now time.Time, frame uint64) {
	if state.LastPrayerAt.IsZero() {
		return
	}
	// Whole seconds, not the exact age: Claude Code re-runs the status line on
	// events as well as on its timer, and a count derived from a fractional
	// age would redraw a different picture twice within the same second.
	elapsed := now.Unix() - state.LastPrayerAt.Unix()
	shows := int64(PrayerShows / time.Second)
	if elapsed < 0 || elapsed >= shows {
		return
	}

	// Start dense and thin out, so the burst reads as a burst.
	remaining := 1 - float64(elapsed)/float64(shows)
	count := int(float64(MaxPrayers)*remaining) + 1

	placed := 0
	for attempt := 0; attempt < 400 && placed < count; attempt++ {
		x := int(effect.Hash01(frame, attempt, placed, "x") * float64(g.width))
		y := int(effect.Hash01(frame, attempt, placed, "y") * float64(len(g.cells)))

		// The censer's own block is reserved, not just its filled cells: a
		// prayer wedged between the legs reads as noise inside the vessel
		// rather than an offering around it.
		if x < CenserWidth && y >= smokeRows {
			continue
		}
		if !g.free(x, y, prayerCells) {
			continue
		}
		g.text(x, y, Prayer)
		placed++
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

func (g *grid) lines() []string {
	out := make([]string, 0, len(g.cells))
	for _, row := range g.cells {
		var b strings.Builder
		for _, r := range row {
			if r == covered {
				continue
			}
			b.WriteRune(r)
		}
		out = append(out, strings.TrimRight(b.String(), " "))
	}
	return out
}

// phaseOf resolves an unset phase to idle, the same way Render does.
func phaseOf(state contracts.SessionState) contracts.SessionPhase {
	if _, known := phaseLabels[state.Phase]; !known {
		return contracts.PhaseIdle
	}
	return state.Phase
}
