package layout

import (
	"fmt"
	"testing"

	"github.com/cruellaDev/claude-code-prayops/contracts"
)

func TestModeThresholds(t *testing.T) {
	cases := []struct {
		width, height int
		want          contracts.LayoutMode
	}{
		{80, 24, contracts.LayoutFull},
		{FullWidth, FullHeight, contracts.LayoutFull},
		{FullWidth - 1, FullHeight, contracts.LayoutMedium},
		{FullWidth, FullHeight - 1, contracts.LayoutMedium},
		{MediumWidth, MediumHeight, contracts.LayoutMedium},
		{MediumWidth - 1, MediumHeight, contracts.LayoutCompact},
		{20, 6, contracts.LayoutCompact},
	}

	for _, tc := range cases {
		if got := Compute(tc.width, tc.height).Mode; got != tc.want {
			t.Fatalf("%dx%d = %q, want %q", tc.width, tc.height, got, tc.want)
		}
	}
}

// Every rectangle has to stay inside the content area, at every size. Testing
// only against the terminal missed furniture drawn over the border and the
// title on short terminals.
func TestFurnitureStaysInsideTheContentArea(t *testing.T) {
	for width := MinWidth; width <= 120; width++ {
		for height := MinHeight; height <= 40; height++ {
			l := Compute(width, height)
			bounds := l.Content

			for name, rect := range map[string]contracts.Rect{
				"altar":   l.Altar,
				"plates":  l.Plates,
				"burner":  l.Burner,
				"incense": l.Incense,
				"status":  l.Status,
			} {
				if rect.Width == 0 || rect.Height == 0 {
					continue
				}
				if !contains(bounds, rect) {
					t.Fatalf("%dx%d: %s %+v escapes the content area %+v", width, height, name, rect, bounds)
				}
			}
		}
	}
}

func contains(outer, inner contracts.Rect) bool {
	return inner.X >= outer.X && inner.Y >= outer.Y &&
		inner.X+inner.Width <= outer.X+outer.Width &&
		inner.Y+inner.Height <= outer.Y+outer.Height
}

// The scene has to read as a scene: incense above the burner, burner behind
// the plates, plates on the altar, status at the bottom.
func TestFurnitureIsStacked(t *testing.T) {
	l := Compute(80, 24)

	if !(l.Incense.Y < l.Burner.Y) {
		t.Fatalf("incense %+v is not above the burner %+v", l.Incense, l.Burner)
	}
	if !(l.Burner.Y+l.Burner.Height <= l.Plates.Y) {
		t.Fatalf("burner %+v overlaps the plates %+v", l.Burner, l.Plates)
	}
	if !(l.Plates.Y < l.Altar.Y) {
		t.Fatalf("plates %+v are not on the altar %+v", l.Plates, l.Altar)
	}
	if !(l.Altar.Y < l.Status.Y) {
		t.Fatalf("altar %+v is not above the status %+v", l.Altar, l.Status)
	}
}

func TestBurnerIsCentred(t *testing.T) {
	for _, width := range []int{52, 60, 80, 100, 121} {
		l := Compute(width, 24)
		leftGap := l.Burner.X - l.Content.X
		rightGap := l.Content.X + l.Content.Width - (l.Burner.X + l.Burner.Width)

		if diff := leftGap - rightGap; diff < -1 || diff > 1 {
			t.Fatalf("width %d: burner off centre by %d (%d vs %d)", width, diff, leftGap, rightGap)
		}
	}
}

// A terminal too small for the scene must still produce something to draw
// into rather than a negative rectangle.
func TestTinyTerminalsDegradeInsteadOfBreaking(t *testing.T) {
	for _, size := range [][2]int{{0, 0}, {1, 1}, {10, 4}, {MinWidth - 1, MinHeight - 1}} {
		l := Compute(size[0], size[1])

		if l.Mode != contracts.LayoutCompact {
			t.Fatalf("%dx%d mode = %q", size[0], size[1], l.Mode)
		}
		for name, rect := range map[string]contracts.Rect{
			"content": l.Content, "status": l.Status, "burner": l.Burner,
		} {
			if rect.Width < 0 || rect.Height < 0 {
				t.Fatalf("%dx%d: %s has a negative dimension: %+v", size[0], size[1], name, rect)
			}
		}
	}
}

func TestSafeZonesAvoidTheFurniture(t *testing.T) {
	l := Compute(80, 24)
	zones := SafeZones(l, 10, 4)

	if len(zones) == 0 {
		t.Fatal("no safe zone on an 80x24 terminal")
	}

	for zone, rect := range zones {
		for name, furniture := range map[string]contracts.Rect{
			"burner": l.Burner,
			"plates": l.Plates,
			"altar":  l.Altar,
			"status": l.Status,
		} {
			if rect.Intersects(furniture) {
				t.Fatalf("zone %q %+v overlaps the %s %+v", zone, rect, name, furniture)
			}
		}
		if !contains(l.Content, rect) {
			t.Fatalf("zone %q %+v escapes the content area %+v", zone, rect, l.Content)
		}
	}
}

// A zone that cannot hold the effect must not be offered, so placement never
// has to bounds check.
func TestSafeZonesDropWhatCannotFit(t *testing.T) {
	l := Compute(80, 24)

	if len(SafeZones(l, 10, 4)) < 2 {
		t.Fatal("a modest effect should fit in more than one zone")
	}
	if got := SafeZones(l, 500, 500); len(got) != 0 {
		t.Fatalf("an oversized effect was offered %d zones", len(got))
	}
	if got := SafeZones(l, 0, 0); len(got) != 0 {
		t.Fatalf("a zero sized effect was offered %d zones", len(got))
	}

	for zone, rect := range SafeZones(l, 10, 4) {
		if rect.Width < 10 || rect.Height < 4 {
			t.Fatalf("zone %q is %dx%d, too small for the effect", zone, rect.Width, rect.Height)
		}
	}
}

func TestSafeZonesOnATinyTerminal(t *testing.T) {
	if got := SafeZones(Compute(10, 4), 4, 2); len(got) != 0 {
		t.Fatalf("a terminal with no scene offered %d zones", len(got))
	}
}

// Resizing must be a pure function of the size, or an effect would move for
// reasons the user cannot see.
func TestComputeIsDeterministic(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {52, 18}, {36, 12}} {
		first := Compute(size[0], size[1])
		for i := 0; i < 5; i++ {
			if got := Compute(size[0], size[1]); got != first {
				t.Fatalf("%v: layout changed between calls\n%+v\n%+v", size, first, got)
			}
		}
	}
}

func TestZoneCoverageAcrossSizes(t *testing.T) {
	// A sanity sweep: at every size the scene either offers zones or has no
	// room, and never offers a zone that overlaps the burner.
	for width := MinWidth; width <= 100; width += 7 {
		for height := MinHeight; height <= 30; height += 3 {
			l := Compute(width, height)
			for zone, rect := range SafeZones(l, 6, 2) {
				if rect.Intersects(l.Burner) {
					t.Fatalf("%dx%d zone %q overlaps the burner", width, height, zone)
				}
			}
		}
	}
}

func ExampleCompute() {
	l := Compute(80, 24)
	fmt.Println(l.Mode, l.Burner.Width, l.Incense.Width)
	// Output: FULL 9 5
}
