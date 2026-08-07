package statusline

import (
	"time"

	"github.com/cruellaDev/claude-code-prayops/contracts"
	"github.com/cruellaDev/claude-code-prayops/internal/effect"
)

// The holder is the third theme: a leaf-shaped tray with a hook at one end and
// a stick of incense lying almost flat across it, which is what the reference
// the user picked is.
//
// It is the one theme that reports rather than decorates. The other two answer
// a finished turn with a flourish; this one shows the turn happening - the
// stick is as long as the turn has left, so a glance says how long this has
// been going.
var holder = []string{
	"   ▄                   ",
	"  ▐▌                   ",
	"  ▝▌                   ",
	"▗▄▟█████████▙▄▖        ",
	" ▝▀▀▀▀▀▀▀▀▀▀▀▘         ",
}

// HolderWidth is the width of every row of the holder, in cells.
const HolderWidth = 23

// holderSmokeRows is how many rows rise above the stick.
const holderSmokeRows = 2

// The stick lies on the row the hook holds it at, running from the hook to the
// far end of the tray.
// The hook lifts the stick clear of the tray. Resting it straight on the wood
// read as a line drawn on a plank rather than incense in a holder - the gap
// under it is what says the thing is holding something.
const (
	stickRow   = 1
	stickStart = 4
	stickEnd   = 21
)

// The stick is a pale green and its lit end is red. Both sit inside the
// luminance band the contrast test enforces, so neither disappears into a
// terminal's background, whichever one it is.
const (
	ansiStick    = "\x1b[38;5;151m"
	ansiEmberTip = "\x1b[38;5;203m"
)

// Ash is what the stick leaves behind as it burns down.
const Ash = '·'

// HolderScene renders the tray, the stick burned down to wherever the turn has
// got to, and the status text.
func HolderScene(state contracts.SessionState, opts SceneOptions) []string {
	columns := opts.Columns
	if columns <= 0 {
		columns = DefaultColumns
	}
	if columns < HolderWidth+2 {
		return nil
	}

	g := newGrid(columns, holderSmokeRows+len(holder))

	frame := uint64(opts.Now.Unix())
	if !opts.Motion {
		frame = 0
	}

	for i, row := range holder {
		g.text(0, holderSmokeRows+i, row)
	}
	tip := drawStick(g, state, opts.Now)
	drawStickSmoke(g, tip, frame, burning(phaseOf(state)))

	return append(g.lines(opts.Color), Render(state, Options{
		Now: opts.Now, Columns: columns, Color: opts.Color,
	}))
}

// HolderHeight is how many rows the holder scene draws above the status text.
func HolderHeight() int { return holderSmokeRows + len(holder) }

// HolderRows returns the holder's static rows.
func HolderRows() []string { return append([]string(nil), holder...) }

// drawStick lays the incense across the tray, burned down to whatever is left
// of the turn, and returns the column the lit end is at.
//
// It burns from the far end towards the hook, which is the direction a stick
// in a holder actually burns, and leaves ash where it has been.
func drawStick(g *grid, state contracts.SessionState, now time.Time) int {
	length := stickEnd - stickStart + 1
	left := Remaining(state, now) * length / 100

	y := holderSmokeRows + stickRow
	for x := stickStart; x <= stickEnd; x++ {
		switch {
		case x < stickStart+left-1:
			g.paint(x, y, '▄', ansiStick)
		case x == stickStart+left-1:
			g.paint(x, y, '▄', ansiEmberTip)
		default:
			// Burnt already. The ash stays put rather than vanishing, so the
			// stick reads as consumed rather than as never having been there.
			g.set(x, y, Ash)
		}
	}

	if left <= 0 {
		// Burnt out. The caller stops the plume rather than smoking ash.
		return -1
	}
	return stickStart + left - 1
}

// drawStickSmoke lifts smoke off the lit end, wherever that has got to.
//
// This is the one plume in the project that moves, and it moves for a reason:
// it follows the ember down the stick over ten minutes, not at random every
// second. The columns are fixed relative to the ember, so within any second it
// is as still as the others.
func drawStickSmoke(g *grid, tip int, frame uint64, active bool) {
	dense := 0.25
	if active {
		dense = 0.55
	}

	// A stick that has burned out gives one last wisp above the hook and
	// nothing else. It cannot give nothing at all: an empty row is dropped by
	// the host, and the scene would lose height the moment the incense did.
	if tip < 0 {
		for row := 0; row < holderSmokeRows; row++ {
			g.set(stickStart-1+row, row, '·')
		}
		return
	}

	for row := 0; row < holderSmokeRows; row++ {
		columns := []int{tip}
		if row == 0 {
			columns = []int{tip - 1, tip + 1}
		}

		for i, column := range columns {
			glyph := '░'
			if effect.Hash01(frame, row*8+i, 0, "puff") < dense {
				glyph = '▒'
			}
			g.set(column, row, glyph)
		}
	}
}
