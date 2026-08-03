package contracts

type RGB struct {
	R uint8
	G uint8
	B uint8
}

type Cell struct {
	Rune rune
	FG   RGB
	BG   RGB
	Z    int
	Set  bool
}

type TerminalRaster struct {
	Width  int
	Height int
	Cells  []RasterCell
}

type RasterCell struct {
	X      int
	Y      int
	Top    RGB
	Bottom RGB
	Alpha  float64
	Glyph  rune
}
