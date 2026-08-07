package statusline

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/cruellaDev/claude-code-prayops/contracts"
)

// luminance of an xterm-256 colour, 0 for black and 1 for white.
func luminance(code int) float64 {
	var r, g, b float64

	switch {
	case code >= 232:
		v := float64(8+10*(code-232)) / 255
		r, g, b = v, v, v
	case code >= 16:
		level := []float64{0, 95, 135, 175, 215, 255}
		i := code - 16
		r = level[i/36] / 255
		g = level[(i/6)%6] / 255
		b = level[i%6] / 255
	default:
		// The sixteen system colours are whatever the user's scheme says, so
		// they cannot be reasoned about at all.
		return -1
	}
	return 0.2126*r + 0.7152*g + 0.0722*b
}

// Every colour the scene paints has to be legible on a dark background and on
// a light one, because a terminal is either.
//
// This is not hypothetical: the lamp's outline was briefly near black, which
// looked right on a light background and erased nearly half the lamp on a dark
// one. The fix was to stop painting the outline at all - an unpainted cell
// takes the terminal's own foreground, which contrasts with its own background
// by definition.
func TestEveryPaintedColourSurvivesBothBackgrounds(t *testing.T) {
	code := regexp.MustCompile(`38;5;(\d+)m`)

	// Every prayer in the range, so both of the lamp's answers render. Checking
	// one prayer only exercises whichever outcome its seed happens to give,
	// and the colour that goes unrendered goes unchecked.
	states := []contracts.SessionState{working(time.Minute), idle()}
	for k := 0; k < 20; k++ {
		states = append(states, praying(now.Add(time.Duration(k)*time.Second)))
	}

	for _, theme := range []Theme{ThemeCenser, ThemeLamp} {
		for _, state := range states {
			for second := 0; second < 8; second++ {
				at := now
				if !state.LastPrayerAt.IsZero() {
					at = state.LastPrayerAt
				}

				lines := Scene(state, SceneOptions{
					Now:     at.Add(time.Duration(second) * time.Second),
					Columns: 80, Color: true, Motion: true, Theme: theme,
				})

				for _, match := range code.FindAllStringSubmatch(strings.Join(lines, "\n"), -1) {
					var n int
					for _, r := range match[1] {
						n = n*10 + int(r-'0')
					}

					l := luminance(n)
					if l < 0 {
						t.Fatalf("%s paints with system colour %d, which the user's scheme redefines", theme, n)
					}
					if l < 0.20 {
						t.Fatalf("%s paints with colour %d (luminance %.2f), which vanishes on a dark background", theme, n, l)
					}
					if l > 0.90 {
						t.Fatalf("%s paints with colour %d (luminance %.2f), which vanishes on a light background", theme, n, l)
					}
				}
			}
		}
	}
}
