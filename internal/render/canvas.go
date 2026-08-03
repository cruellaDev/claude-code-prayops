// Package render draws the altar. It produces plain strings and knows nothing
// about Bubble Tea, so every frame can be asserted in a test.
package render

import (
	"fmt"
	"strings"

	"github.com/cruellaDev/claude-code-prayops/contracts"
	"github.com/mattn/go-runewidth"
)

// Canvas is a fixed grid of terminal cells.
//
// Wide runes occupy two cells: the rune itself and a continuation cell that
// renders as nothing. Without that bookkeeping a CJK glyph would push
// everything after it one column right and tear the border.
type Canvas struct {
	width  int
	height int
	cells  []cell
}

// cell is one terminal cell: a rune and, for prayer images, the colours of
// the half block drawn there.
type cell struct {
	r     rune
	fg    contracts.RGB
	bg    contracts.RGB
	hasFG bool
	hasBG bool
}

// continuation marks the second cell of a wide rune.
const continuation rune = 0

// width treats East Asian ambiguous runes as one cell.
//
// Box drawing characters are ambiguous, so with the default condition their
// width follows the user's locale: on a Korean or Japanese system every ─ and
// │ would claim two cells and overwrite its neighbour, erasing the frame.
// Terminals draw them in one cell, so the condition is pinned rather than
// detected. Hangul and Han are unambiguously wide and are unaffected.
var narrow = &runewidth.Condition{EastAsianWidth: false}

// NewCanvas returns a canvas filled with spaces.
func NewCanvas(width, height int) *Canvas {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}

	c := &Canvas{width: width, height: height, cells: make([]cell, width*height)}
	for i := range c.cells {
		c.cells[i].r = ' '
	}
	return c
}

func (c *Canvas) inside(x, y int) bool {
	return x >= 0 && y >= 0 && x < c.width && y < c.height
}

// Set writes one rune. Out of bounds writes are dropped rather than panicking,
// because layout arithmetic at the edge of a resize is not worth crashing a
// watcher over.
func (c *Canvas) Set(x, y int, r rune) {
	if !c.inside(x, y) {
		return
	}

	// Overwriting the first half of a wide rune would leave its orphaned
	// continuation behind, so clear the pair first.
	c.clearPairAt(x, y)

	cells := narrow.RuneWidth(r)
	if cells <= 0 {
		return
	}
	if cells == 2 {
		if !c.inside(x+1, y) {
			// No room for the second half; drop it rather than render half a
			// glyph over the border.
			return
		}
		c.clearPairAt(x+1, y)
		c.cells[y*c.width+x] = cell{r: r}
		c.cells[y*c.width+x+1] = cell{r: continuation}
		return
	}
	c.cells[y*c.width+x] = cell{r: r}
}

// SetColored writes a rune with explicit foreground and background colours,
// which is how a prayer image half block is drawn.
func (c *Canvas) SetColored(x, y int, r rune, fg, bg contracts.RGB) {
	if !c.inside(x, y) {
		return
	}
	c.Set(x, y, r)
	if c.At(x, y) != r {
		return // the write was dropped
	}
	c.cells[y*c.width+x].fg = fg
	c.cells[y*c.width+x].bg = bg
	c.cells[y*c.width+x].hasFG = true
	c.cells[y*c.width+x].hasBG = true
}

// At returns the rune at a position, or a space outside the canvas.
func (c *Canvas) At(x, y int) rune {
	if !c.inside(x, y) {
		return ' '
	}
	return c.cells[y*c.width+x].r
}

// clearPairAt blanks the cell at x and whichever half of a wide rune it
// belongs to.
func (c *Canvas) clearPairAt(x, y int) {
	if !c.inside(x, y) {
		return
	}
	index := y*c.width + x

	if c.cells[index].r == continuation {
		if x > 0 {
			c.cells[index-1] = cell{r: ' '}
		}
		c.cells[index] = cell{r: ' '}
		return
	}
	if narrow.RuneWidth(c.cells[index].r) == 2 && c.inside(x+1, y) {
		c.cells[index+1] = cell{r: ' '}
	}
	c.cells[index] = cell{r: ' '}
}

// Text writes a string starting at x, advancing by each rune's display width.
// It returns the number of cells written.
func (c *Canvas) Text(x, y int, text string) int {
	cursor := x
	for _, r := range text {
		c.Set(cursor, y, r)
		cursor += max(narrow.RuneWidth(r), 1)
	}
	return cursor - x
}

// HLine fills a horizontal run with one rune.
func (c *Canvas) HLine(x, y, width int, r rune) {
	for i := 0; i < width; i++ {
		c.Set(x+i, y, r)
	}
}

// Border draws a rounded box around the whole canvas.
func (c *Canvas) Border() {
	if c.width < 2 || c.height < 2 {
		return
	}

	c.HLine(1, 0, c.width-2, '─')
	c.HLine(1, c.height-1, c.width-2, '─')
	for y := 1; y < c.height-1; y++ {
		c.Set(0, y, '│')
		c.Set(c.width-1, y, '│')
	}
	c.Set(0, 0, '╭')
	c.Set(c.width-1, 0, '╮')
	c.Set(0, c.height-1, '╰')
	c.Set(c.width-1, c.height-1, '╯')
}

// Title writes a centred title into the top border.
func (c *Canvas) Title(text string) {
	if c.width < narrow.StringWidth(text)+6 {
		return
	}
	label := " " + text + " "
	c.Text((c.width-narrow.StringWidth(label))/2, 0, label)
}

// Lines renders the canvas without colour, dropping continuation cells.
func (c *Canvas) Lines() []string { return c.lines(false) }

// ColorLines renders the canvas with ANSI colour where cells carry it.
func (c *Canvas) ColorLines() []string { return c.lines(true) }

func (c *Canvas) lines(color bool) []string {
	lines := make([]string, 0, c.height)

	for y := 0; y < c.height; y++ {
		var b strings.Builder
		colored := false

		for x := 0; x < c.width; x++ {
			cell := c.cells[y*c.width+x]
			if cell.r == continuation {
				continue
			}

			switch {
			case color && (cell.hasFG || cell.hasBG):
				if cell.hasFG {
					fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm", cell.fg.R, cell.fg.G, cell.fg.B)
				}
				if cell.hasBG {
					fmt.Fprintf(&b, "\x1b[48;2;%d;%d;%dm", cell.bg.R, cell.bg.G, cell.bg.B)
				}
				b.WriteRune(cell.r)
				b.WriteString(ansiReset)
				colored = true
			default:
				b.WriteRune(cell.r)
			}
		}

		line := b.String()
		if !colored {
			// Only uncoloured lines can be trimmed safely; a reset sequence at
			// the end is not a space.
			line = strings.TrimRight(line, " ")
		}
		lines = append(lines, line)
	}
	return lines
}

// ansiReset returns the terminal to its own colours.
const ansiReset = "\x1b[0m"

// String renders the canvas as one newline-separated block, without colour.
func (c *Canvas) String() string { return strings.Join(c.Lines(), "\n") }

// ColorString renders the canvas with colour.
func (c *Canvas) ColorString() string { return strings.Join(c.ColorLines(), "\n") }
