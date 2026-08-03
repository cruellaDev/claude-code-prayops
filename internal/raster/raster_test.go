package raster

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cruellaDev/claude-code-prayops/contracts"
)

// draw renders a raster back to lines so a test can read it. A wide glyph
// covers the cell after it, which is why that cell is skipped rather than
// printed as a space.
func draw(r contracts.TerminalRaster) []string {
	grid := make([][]rune, r.Height)
	for y := range grid {
		grid[y] = []rune(strings.Repeat(" ", r.Width))
	}
	for _, cell := range r.Cells {
		if cell.Y < len(grid) && cell.X < len(grid[cell.Y]) {
			grid[cell.Y][cell.X] = cell.Glyph
		}
	}

	lines := make([]string, len(grid))
	for i, row := range grid {
		var b strings.Builder
		for x := 0; x < len(row); x++ {
			b.WriteRune(row[x])
			if narrow.RuneWidth(row[x]) == 2 {
				x++ // the covered cell
			}
		}
		lines[i] = b.String()
	}
	return lines
}

func TestTextCardIsBordered(t *testing.T) {
	r, err := Text("무사배포")
	if err != nil {
		t.Fatalf("text: %v", err)
	}

	lines := draw(r)
	if len(lines) != 3 {
		t.Fatalf("card is %d lines:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	if !strings.HasPrefix(lines[0], "╭") || !strings.HasSuffix(lines[0], "╮") {
		t.Fatalf("top = %q", lines[0])
	}
	if !strings.Contains(lines[1], "무사배포") {
		t.Fatalf("text is missing:\n%s", strings.Join(lines, "\n"))
	}
	if !strings.HasPrefix(lines[2], "╰") || !strings.HasSuffix(lines[2], "╯") {
		t.Fatalf("bottom = %q", lines[2])
	}
}

func TestTextWrapsAndIsBounded(t *testing.T) {
	long := "please let the deployment go through without waking anyone up tonight or tomorrow"

	r, err := Text(long)
	if err != nil {
		t.Fatalf("text: %v", err)
	}

	if r.Height > MaxTextLines+2 {
		t.Fatalf("card is %d rows, want at most %d", r.Height, MaxTextLines+2)
	}
	if r.Width > MaxTextCells+4 {
		t.Fatalf("card is %d cells wide, want at most %d", r.Width, MaxTextCells+4)
	}
	for _, cell := range r.Cells {
		if cell.X >= r.Width || cell.Y >= r.Height || cell.X < 0 || cell.Y < 0 {
			t.Fatalf("cell outside the card: %+v", cell)
		}
	}
}

// A single word longer than the card must be cut, not dropped.
func TestUnbreakableTextIsCut(t *testing.T) {
	r, err := Text(strings.Repeat("x", 200))
	if err != nil {
		t.Fatalf("text: %v", err)
	}

	if r.Width > MaxTextCells+4 {
		t.Fatalf("card is %d cells wide", r.Width)
	}
	if r.Height <= 2 {
		t.Fatalf("nothing was rendered: %d rows", r.Height)
	}
}

func TestEmptyTextIsRefused(t *testing.T) {
	for _, input := range []string{"", "   ", "\n\t"} {
		if _, err := Text(input); err == nil {
			t.Fatalf("empty prayer %q was accepted", input)
		}
	}
}

func TestPresets(t *testing.T) {
	names := PresetNames()
	if len(names) == 0 {
		t.Fatal("no presets")
	}
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Fatalf("preset names are not sorted: %v", names)
		}
	}

	for _, name := range names {
		r, err := Preset(name)
		if err != nil {
			t.Fatalf("preset %q: %v", name, err)
		}
		if r.Width == 0 || r.Height == 0 || len(r.Cells) == 0 {
			t.Fatalf("preset %q rendered nothing", name)
		}
	}

	if _, err := Preset("nonexistent"); err == nil {
		t.Fatal("an unknown preset was accepted")
	}
}

// The default prayer has to exist, since /prayops:pray with no argument uses
// it.
func TestDeployPresetExists(t *testing.T) {
	if _, err := Preset("deploy"); err != nil {
		t.Fatalf("deploy preset: %v", err)
	}
}

func writePNG(t *testing.T, dir string, width, height int, opaque bool) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			alpha := uint8(255)
			if !opaque && (x < width/4 || y < height/4) {
				alpha = 0
			}
			img.Set(x, y, color.NRGBA{R: uint8(x * 255 / max(width-1, 1)), G: uint8(y * 255 / max(height-1, 1)), B: 128, A: alpha})
		}
	}

	path := filepath.Join(dir, "prayer.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return path
}

func TestImageRendersHalfBlocks(t *testing.T) {
	path := writePNG(t, t.TempDir(), 40, 40, true)

	r, err := Image(path)
	if err != nil {
		t.Fatalf("image: %v", err)
	}

	if r.Width == 0 || r.Height == 0 || len(r.Cells) == 0 {
		t.Fatalf("nothing rendered: %+v", r)
	}
	if r.Width > MaxCellWidth || r.Height > MaxCellHeight {
		t.Fatalf("%dx%d exceeds the cell budget", r.Width, r.Height)
	}

	for _, cell := range r.Cells {
		if cell.Glyph != HalfBlock {
			t.Fatalf("cell glyph = %q, want a half block", cell.Glyph)
		}
		if cell.Alpha <= 0 || cell.Alpha > 1 {
			t.Fatalf("alpha out of range: %v", cell.Alpha)
		}
	}

	// A gradient must not collapse to one colour.
	colours := map[contracts.RGB]bool{}
	for _, cell := range r.Cells {
		colours[cell.Top] = true
	}
	if len(colours) < 4 {
		t.Fatalf("only %d distinct colours sampled", len(colours))
	}
}

// A square image must stay roughly square on screen, where a cell is two
// pixels tall.
func TestImagePreservesAspect(t *testing.T) {
	cases := []struct {
		w, h  int
		ratio float64
	}{
		{40, 40, 1},
		{80, 40, 2},
		{40, 80, 0.5},
	}

	for _, tc := range cases {
		dir := t.TempDir()
		r, err := Image(writePNG(t, dir, tc.w, tc.h, true))
		if err != nil {
			t.Fatalf("%dx%d: %v", tc.w, tc.h, err)
		}

		// One cell is two pixels tall, so the on-screen ratio is width/(2*rows).
		got := float64(r.Width) / float64(r.Height*2)
		if got < tc.ratio*0.55 || got > tc.ratio*1.8 {
			t.Fatalf("%dx%d rendered %dx%d cells, ratio %.2f, want near %.2f",
				tc.w, tc.h, r.Width, r.Height, got, tc.ratio)
		}
	}
}

// Transparent regions are what crop an image to its subject.
func TestTransparentCellsAreDropped(t *testing.T) {
	dir := t.TempDir()

	opaque, err := Image(writePNG(t, dir, 40, 40, true))
	if err != nil {
		t.Fatalf("opaque: %v", err)
	}
	transparent, err := Image(writePNG(t, t.TempDir(), 40, 40, false))
	if err != nil {
		t.Fatalf("transparent: %v", err)
	}

	if len(transparent.Cells) >= len(opaque.Cells) {
		t.Fatalf("transparency dropped nothing: %d vs %d cells",
			len(transparent.Cells), len(opaque.Cells))
	}
	if len(transparent.Cells) == 0 {
		t.Fatal("everything was dropped")
	}
}

func TestJPEGIsSupported(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 8), G: 64, B: 200, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode: %v", err)
	}
	path := filepath.Join(t.TempDir(), "prayer.jpg")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := Image(path); err != nil {
		t.Fatalf("jpeg: %v", err)
	}
}

// The limits exist so a prayer cannot make the watcher chew through a huge
// file the user pointed at by mistake.
func TestOversizedFilesAreRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "huge.png")
	if err := os.WriteFile(path, make([]byte, MaxFileBytes+1), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := Image(path)
	if err == nil {
		t.Fatal("an oversized file was accepted")
	}
	if !strings.Contains(err.Error(), "limit") {
		t.Fatalf("unhelpful error: %v", err)
	}
}

func TestOversizedDimensionsAreRefused(t *testing.T) {
	path := writePNG(t, t.TempDir(), MaxDecodePx+1, 8, true)

	if _, err := Image(path); err == nil {
		t.Fatal("an over-wide image was accepted")
	}
}

func TestMissingAndUndecodableFiles(t *testing.T) {
	dir := t.TempDir()

	if _, err := Image(filepath.Join(dir, "nope.png")); err == nil {
		t.Fatal("a missing file was accepted")
	}
	if _, err := Image(dir); err == nil {
		t.Fatal("a directory was accepted")
	}

	notAnImage := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(notAnImage, []byte("this is not a prayer image"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Image(notAnImage); err == nil {
		t.Fatal("a text file was accepted as an image")
	}
}
