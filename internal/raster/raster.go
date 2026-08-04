// Package raster turns a prayer into terminal cells.
//
// A cell is two vertical pixels drawn as a half block, so an image gets twice
// the vertical resolution the character grid would otherwise allow.
package raster

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/webp"

	"github.com/cruellaDev/claude-code-prayops/contracts"
	"github.com/mattn/go-runewidth"
)

// Limits from the effect specification.
const (
	MaxFileBytes  = 4 << 20
	MaxDecodePx   = 1024
	MaxCellWidth  = 24
	MaxCellHeight = 12

	// MaxTextCells is the grapheme budget for a text card.
	MaxTextCells = 24
	// MaxTextLines is how many lines a card wraps to.
	MaxTextLines = 3
)

// HalfBlock is the glyph an image cell uses: the top half is the foreground
// colour and the bottom half is the background.
const HalfBlock = '▀'

// narrow pins East Asian ambiguous runes to one cell, matching the renderer.
var narrow = &runewidth.Condition{EastAsianWidth: false}

// presets are the built-in prayers. Keeping them here rather than in files
// means a preset cannot go missing from an install.
var presets = map[string][]string{
	"deploy":  {"무사 배포"},
	"release": {"무사 배포", "無 事 故"},
	"green":   {"모든 테스트가", "초록이기를"},
	"debug":   {"근본 원인을", "찾게 하소서"},
	"review":  {"리뷰가", "너그럽기를"},
}

// PresetNames lists the built-in prayers in a stable order.
func PresetNames() []string {
	names := make([]string, 0, len(presets))
	for name := range presets {
		names = append(names, name)
	}
	// Sorted so the help output does not shuffle between runs.
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j] < names[j-1]; j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}
	return names
}

// Preset renders a built-in prayer.
func Preset(name string) (contracts.TerminalRaster, error) {
	lines, ok := presets[name]
	if !ok {
		return contracts.TerminalRaster{}, fmt.Errorf("raster: unknown preset %q", name)
	}
	return card(lines), nil
}

// Text renders a user's words as a bordered card.
//
// The text is the user's own, and it is rendered rather than parsed: nothing
// here inspects it for meaning.
func Text(text string) (contracts.TerminalRaster, error) {
	lines := wrap(strings.TrimSpace(text), MaxTextCells, MaxTextLines)
	if len(lines) == 0 {
		return contracts.TerminalRaster{}, fmt.Errorf("raster: empty prayer")
	}
	return card(lines), nil
}

// wrap splits text into at most maxLines lines of at most width cells,
// breaking on spaces where it can and mid-word where it cannot.
func wrap(text string, width, maxLines int) []string {
	if text == "" {
		return nil
	}

	var lines []string
	var line strings.Builder
	lineWidth := 0

	flush := func() {
		if line.Len() > 0 {
			lines = append(lines, line.String())
			line.Reset()
			lineWidth = 0
		}
	}

	for _, word := range strings.Fields(text) {
		wordWidth := narrow.StringWidth(word)

		// A word longer than the line is cut rather than dropped.
		for wordWidth > width {
			cut, rest := split(word, width)
			flush()
			lines = append(lines, cut)
			if len(lines) >= maxLines {
				return lines
			}
			word, wordWidth = rest, narrow.StringWidth(rest)
		}

		switch {
		case lineWidth == 0:
			line.WriteString(word)
			lineWidth = wordWidth
		case lineWidth+1+wordWidth <= width:
			line.WriteString(" " + word)
			lineWidth += 1 + wordWidth
		default:
			flush()
			if len(lines) >= maxLines {
				return lines
			}
			line.WriteString(word)
			lineWidth = wordWidth
		}
	}
	flush()

	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return lines
}

// split cuts a string at a cell width, never inside a rune.
func split(s string, width int) (string, string) {
	cells := 0
	for i, r := range s {
		w := max(narrow.RuneWidth(r), 1)
		if cells+w > width {
			return s[:i], s[i:]
		}
		cells += w
	}
	return s, ""
}

// card draws lines inside a rounded border, centred.
func card(lines []string) contracts.TerminalRaster {
	inner := 0
	for _, line := range lines {
		inner = max(inner, narrow.StringWidth(line))
	}
	inner = min(inner, MaxTextCells)

	width := inner + 4
	height := len(lines) + 2

	r := contracts.TerminalRaster{Width: width, Height: height}
	put := func(x, y int, glyph rune) {
		r.Cells = append(r.Cells, contracts.RasterCell{X: x, Y: y, Glyph: glyph, Alpha: 1})
	}

	for x := 1; x < width-1; x++ {
		put(x, 0, '─')
		put(x, height-1, '─')
	}
	put(0, 0, '╭')
	put(width-1, 0, '╮')
	put(0, height-1, '╰')
	put(width-1, height-1, '╯')

	for i, line := range lines {
		y := i + 1
		put(0, y, '│')
		put(width-1, y, '│')

		x := 2 + (inner-narrow.StringWidth(line))/2
		for _, glyph := range line {
			put(x, y, glyph)
			x += max(narrow.RuneWidth(glyph), 1)
		}
	}
	return r
}

// Image renders a local image file the user named explicitly.
//
// The path is used exactly as given: nothing here searches for images, and the
// pixels are never kept beyond this call.
func Image(path string) (contracts.TerminalRaster, error) {
	info, err := os.Stat(path)
	if err != nil {
		return contracts.TerminalRaster{}, fmt.Errorf("raster: %w", err)
	}
	if info.IsDir() {
		return contracts.TerminalRaster{}, fmt.Errorf("raster: %s is a directory", path)
	}
	if info.Size() > MaxFileBytes {
		return contracts.TerminalRaster{}, fmt.Errorf(
			"raster: %s is %d bytes, over the %d byte limit", filepath.Base(path), info.Size(), MaxFileBytes)
	}

	file, err := os.Open(path)
	if err != nil {
		return contracts.TerminalRaster{}, fmt.Errorf("raster: %w", err)
	}
	defer file.Close()

	img, format, err := image.Decode(file)
	if err != nil {
		return contracts.TerminalRaster{}, fmt.Errorf("raster: cannot decode %s: %w", filepath.Base(path), err)
	}
	switch format {
	case "png", "jpeg", "webp":
	default:
		return contracts.TerminalRaster{}, fmt.Errorf("raster: %s is unsupported", format)
	}

	bounds := img.Bounds()
	if bounds.Dx() > MaxDecodePx || bounds.Dy() > MaxDecodePx {
		return contracts.TerminalRaster{}, fmt.Errorf(
			"raster: %dx%d is over the %d pixel limit", bounds.Dx(), bounds.Dy(), MaxDecodePx)
	}

	return fromImage(img), nil
}

// fromImage samples an image down to half-block cells, preserving aspect.
//
// Terminal cells are roughly twice as tall as they are wide, and a half block
// splits one cell into two pixels, so a cell covers about the same aspect as a
// square pixel - the sampling grid is width x height*2.
func fromImage(img image.Image) contracts.TerminalRaster {
	src := img.Bounds()
	if src.Dx() <= 0 || src.Dy() <= 0 {
		return contracts.TerminalRaster{}
	}

	cols, rows := fit(src.Dx(), src.Dy())
	r := contracts.TerminalRaster{Width: cols, Height: rows}

	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			top, topAlpha := sample(img, src, col, row*2, cols, rows*2)
			bottom, bottomAlpha := sample(img, src, col, row*2+1, cols, rows*2)

			alpha := (topAlpha + bottomAlpha) / 2
			if alpha < 0.05 {
				// Fully transparent cells are dropped, which is what crops the
				// image to its subject.
				continue
			}
			r.Cells = append(r.Cells, contracts.RasterCell{
				X: col, Y: row, Top: top, Bottom: bottom, Alpha: alpha, Glyph: HalfBlock,
			})
		}
	}
	return r
}

// fit returns the cell dimensions for an image, capped and aspect-preserving.
func fit(pixelWidth, pixelHeight int) (cols, rows int) {
	cols = min(pixelWidth, MaxCellWidth)
	rows = max(cols*pixelHeight/(pixelWidth*2), 1)

	if rows > MaxCellHeight {
		rows = MaxCellHeight
		cols = max(rows*2*pixelWidth/pixelHeight, 1)
		cols = min(cols, MaxCellWidth)
	}
	return max(cols, 1), max(rows, 1)
}

// sample averages the source pixels covered by one grid cell.
func sample(img image.Image, src image.Rectangle, gx, gy, cols, rows int) (contracts.RGB, float64) {
	x0 := src.Min.X + gx*src.Dx()/cols
	x1 := src.Min.X + (gx+1)*src.Dx()/cols
	y0 := src.Min.Y + gy*src.Dy()/rows
	y1 := src.Min.Y + (gy+1)*src.Dy()/rows

	x1 = max(x1, x0+1)
	y1 = max(y1, y0+1)

	var rSum, gSum, bSum, aSum, n uint64
	for y := y0; y < y1 && y < src.Max.Y; y++ {
		for x := x0; x < x1 && x < src.Max.X; x++ {
			cr, cg, cb, ca := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA).RGBA()
			rSum += uint64(cr >> 8)
			gSum += uint64(cg >> 8)
			bSum += uint64(cb >> 8)
			aSum += uint64(ca >> 8)
			n++
		}
	}
	if n == 0 {
		return contracts.RGB{}, 0
	}

	return contracts.RGB{
		R: uint8(rSum / n),
		G: uint8(gSum / n),
		B: uint8(bSum / n),
	}, float64(aSum/n) / 255
}
