// Package layout resolves the altar geometry for a terminal size.
//
// It knows nothing about Bubble Tea or drawing: given a width and height it
// returns the rectangles everything else is positioned against, so placement
// and rendering can be tested without a terminal.
package layout

import "github.com/cruellaDev/claude-code-prayops/contracts"

// Size thresholds between the three layouts.
const (
	MinWidth  = 24
	MinHeight = 8

	FullWidth    = 52
	FullHeight   = 18
	MediumWidth  = 36
	MediumHeight = 12
)

// Geometry of the altar furniture, in cells.
const (
	burnerWidth  = 9
	burnerHeight = 3
	incenseCount = 3
	plateCount   = 3
	plateWidth   = 6
)

// Compute returns the layout for a terminal of the given size.
//
// Sizes below the minimum still return a usable layout with empty furniture
// rectangles rather than an error: a cramped terminal should show a status
// line, not a crash.
func Compute(width, height int) contracts.Layout {
	l := contracts.Layout{Width: width, Height: height, Mode: mode(width, height)}

	if width < MinWidth || height < MinHeight {
		l.Content = contracts.Rect{X: 0, Y: 0, Width: max(width, 0), Height: max(height, 0)}
		l.Status = l.Content
		return l
	}

	// One cell of border on every side, then a title row at the top.
	l.Content = contracts.Rect{X: 1, Y: 2, Width: width - 2, Height: height - 3}

	statusHeight := 2
	if l.Mode == contracts.LayoutCompact {
		statusHeight = 1
	}
	l.Status = contracts.Rect{
		X:      l.Content.X,
		Y:      l.Content.Y + l.Content.Height - statusHeight,
		Width:  l.Content.Width,
		Height: statusHeight,
	}

	// The scene occupies everything above the status block.
	scene := contracts.Rect{
		X:      l.Content.X,
		Y:      l.Content.Y,
		Width:  l.Content.Width,
		Height: l.Content.Height - statusHeight - 1,
	}
	if scene.Height < 4 {
		scene.Height = max(l.Content.Height-statusHeight, 1)
	}

	// The altar surface sits at the bottom of the scene, with the plates
	// resting on it and the burner behind them.
	altarY := scene.Y + scene.Height - 1
	l.Altar = contracts.Rect{X: scene.X, Y: altarY, Width: scene.Width, Height: 1}

	plateRowWidth := min(plateCount*plateWidth+2*(plateCount-1), scene.Width)
	l.Plates = contracts.Rect{
		X:      scene.X + (scene.Width-plateRowWidth)/2,
		Y:      altarY - 1,
		Width:  plateRowWidth,
		Height: 1,
	}

	// The altar and plates take one row each. Whatever is left goes to the
	// burner and then the incense; a scene too short for the burner shows
	// neither rather than drawing them over the border.
	spare := scene.Height - 2
	burnerH := min(spare, burnerHeight)
	if burnerH < 2 {
		return l
	}
	incenseH := min(spare-burnerH, 3)

	burnerW := min(burnerWidth, scene.Width)
	l.Burner = contracts.Rect{
		X:      scene.X + (scene.Width-burnerW)/2,
		Y:      l.Plates.Y - burnerH,
		Width:  burnerW,
		Height: burnerH,
	}

	if incenseH > 0 {
		l.Incense = contracts.Rect{
			X:      l.Burner.X + (l.Burner.Width-incenseCount*2+1)/2,
			Y:      l.Burner.Y - incenseH,
			Width:  incenseCount*2 - 1,
			Height: incenseH,
		}
	}

	return l
}

func mode(width, height int) contracts.LayoutMode {
	switch {
	case width >= FullWidth && height >= FullHeight:
		return contracts.LayoutFull
	case width >= MediumWidth && height >= MediumHeight:
		return contracts.LayoutMedium
	default:
		return contracts.LayoutCompact
	}
}

// SafeZones returns the rectangles a prayer effect may occupy, keyed by zone.
//
// A zone is only returned when it can hold the requested effect size, so
// placement never has to re-check bounds.
func SafeZones(l contracts.Layout, effectWidth, effectHeight int) map[contracts.Zone]contracts.Rect {
	zones := map[contracts.Zone]contracts.Rect{}
	if effectWidth <= 0 || effectHeight <= 0 || l.Burner.Width == 0 {
		return zones
	}

	// Everything above the altar surface and outside the furniture is fair
	// game. The scene's top row is left free so the smoke has somewhere to go.
	top := l.Content.Y + 1
	bottom := l.Plates.Y - 1

	left := contracts.Rect{
		X:      l.Content.X,
		Y:      top,
		Width:  l.Burner.X - l.Content.X - 1,
		Height: bottom - top + 1,
	}
	right := contracts.Rect{
		X:      l.Burner.X + l.Burner.Width + 1,
		Y:      top,
		Width:  l.Content.X + l.Content.Width - (l.Burner.X + l.Burner.Width) - 1,
		Height: bottom - top + 1,
	}

	// The upper zones reach across the burner, above the incense tips.
	upperBottom := l.Incense.Y - 1
	upperLeft := contracts.Rect{
		X:      l.Content.X,
		Y:      top,
		Width:  l.Content.Width / 2,
		Height: upperBottom - top + 1,
	}
	upperRight := contracts.Rect{
		X:      l.Content.X + l.Content.Width/2,
		Y:      top,
		Width:  l.Content.Width - l.Content.Width/2,
		Height: upperBottom - top + 1,
	}

	for zone, rect := range map[contracts.Zone]contracts.Rect{
		contracts.ZoneLeft:       left,
		contracts.ZoneRight:      right,
		contracts.ZoneUpperLeft:  upperLeft,
		contracts.ZoneUpperRight: upperRight,
	} {
		if rect.Width >= effectWidth && rect.Height >= effectHeight {
			zones[zone] = rect
		}
	}
	return zones
}
