package statusline

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cruellaDev/claude-code-prayops/contracts"
)

func scene(t *testing.T, state contracts.SessionState, at time.Time, columns int) []string {
	t.Helper()
	return Scene(state, SceneOptions{Now: at, Columns: columns, Motion: true})
}

func idle() contracts.SessionState {
	state := working(time.Minute)
	state.Phase = contracts.PhaseIdle
	return state
}

func praying(at time.Time) contracts.SessionState {
	state := working(time.Minute)
	state.LastPrayerAt = at
	return state
}

// The censer is the whole point of the plugin, and it has to be drawn where
// the user already is - not in a terminal they have to open.
func TestSceneDrawsTheCenser(t *testing.T) {
	lines := scene(t, working(time.Minute), now, 60)

	if len(lines) != smokeRows+emberRow+len(censer)+1 {
		t.Fatalf("%d rows:\n%s", len(lines), strings.Join(lines, "\n"))
	}

	joined := strings.Join(lines, "\n")
	for name, glyphs := range map[string]string{
		"mouth":   "▗▄▄▄▄▄▖",
		"belly":   "███████",
		"handles": "▗▄▟",
		"stand":   "▝▀▀▀▀▀▘",
		"feet":    "▝  ▀  ▘",
		"incense": "▮ ▮ ▮",
		"ember":   "▪ ▪ ▪",
	} {
		if !strings.Contains(joined, glyphs) {
			t.Fatalf("no %s in the scene:\n%s", name, joined)
		}
	}

	// The last row is the text, so the status is never lost behind the art.
	if !strings.Contains(lines[len(lines)-1], "WORKING") {
		t.Fatalf("last row = %q", lines[len(lines)-1])
	}
}

// Box drawing reads as a diagram. The whole complaint was that it did not look
// like pixels.
func TestSceneUsesBlockGlyphsNotBoxDrawing(t *testing.T) {
	joined := strings.Join(scene(t, working(time.Minute), now, 60), "\n")

	for _, thin := range []string{"─", "│", "┌", "┐", "└", "┘", "╭", "╮", "╰", "╯"} {
		if strings.Contains(joined, thin) {
			t.Fatalf("box drawing glyph %q in the scene:\n%s", thin, joined)
		}
	}
	if !strings.ContainsAny(joined, "█▓▒░▄▀▐▌") {
		t.Fatalf("no block glyphs:\n%s", joined)
	}
}

// No row may exceed the terminal, or Claude Code wraps it and the censer
// breaks apart.
func TestSceneFitsTheTerminal(t *testing.T) {
	for columns := CenserWidth + 2; columns <= 120; columns++ {
		for _, state := range []contracts.SessionState{working(time.Minute), praying(now)} {
			for _, row := range scene(t, state, now, columns) {
				if width(row) > columns {
					t.Fatalf("columns=%d produced a %d cell row: %q", columns, width(row), row)
				}
			}
		}
	}
}

// A window too narrow for the censer gets nothing, and the caller falls back.
func TestSceneDeclinesWhenTooNarrow(t *testing.T) {
	// A width of zero means "unknown", which falls back to the default and
	// still draws; anything positive but too small declines.
	for _, columns := range []int{1, 10, CenserWidth, CenserWidth + 1} {
		if got := scene(t, working(time.Minute), now, columns); got != nil {
			t.Fatalf("columns=%d drew a censer anyway:\n%s", columns, strings.Join(got, "\n"))
		}
	}
}

// A prayer should feel like a lot of them.
func TestPrayerFillsTheScene(t *testing.T) {
	lines := scene(t, praying(now), now, 60)
	count := strings.Count(strings.Join(lines, "\n"), Prayer)

	if count < 10 {
		t.Fatalf("only %d prayers:\n%s", count, strings.Join(lines, "\n"))
	}
	if count > MaxPrayers+1 {
		t.Fatalf("%d prayers, over the cap of %d", count, MaxPrayers)
	}
}

// They thin out rather than vanishing all at once, and they do stop.
func TestPrayersThinOutAndStop(t *testing.T) {
	count := func(after time.Duration) int {
		return strings.Count(strings.Join(scene(t, praying(now), now.Add(after), 60), "\n"), Prayer)
	}

	start := count(0)
	middle := count(PrayerShows / 2)
	if !(start > middle) {
		t.Fatalf("prayers did not thin out: %d then %d", start, middle)
	}
	if middle == 0 {
		t.Fatal("prayers vanished halfway through")
	}
	if got := count(PrayerShows); got != 0 {
		t.Fatalf("%d prayers still showing after the effect ended", got)
	}
	if got := count(-time.Second); got != 0 {
		t.Fatalf("%d prayers showing before the prayer was sent", got)
	}
}

// Offerings pile onto the censer. Reserving its block made the burst look
// fenced off, which is the opposite of what a burst is.
func TestPrayersMayLandOnTheCenser(t *testing.T) {
	landed := false
	for second := 0; second < int(PrayerShows/time.Second) && !landed; second++ {
		lines := scene(t, praying(now), now.Add(time.Duration(second)*time.Second), 60)

		for y, row := range lines[:len(lines)-1] {
			if y < smokeRows {
				continue
			}
			head := []rune(row)
			if len(head) > CenserWidth {
				head = head[:CenserWidth]
			}
			if strings.Contains(string(head), Prayer) {
				landed = true
				break
			}
		}
	}
	if !landed {
		t.Fatal("no prayer ever reached the censer")
	}
}

// The burst travels. If every refresh drew a fresh random spread instead, the
// emoji would rearrange without ever going anywhere.
func TestPrayersTravelOutward(t *testing.T) {
	spread := func(second int) int {
		lines := scene(t, praying(now), now.Add(time.Duration(second)*time.Second), 120)

		widest := 0
		for _, row := range lines[:len(lines)-1] {
			if i := strings.LastIndex(row, Prayer); i > widest {
				widest = i
			}
		}
		return widest
	}

	if !(spread(2) > spread(0)) {
		t.Fatalf("the burst did not travel: %d cells then %d", spread(0), spread(2))
	}
}

// The status line re-runs on events as well as on its timer, so two refreshes
// within the same second must not redraw the scene differently.
func TestSceneIsStableWithinASecond(t *testing.T) {
	state := praying(now)
	first := scene(t, state, now.Add(200*time.Millisecond), 60)

	for _, offset := range []time.Duration{0, 400 * time.Millisecond, 900 * time.Millisecond} {
		got := scene(t, state, now.Add(offset), 60)
		if strings.Join(got, "\n") != strings.Join(first, "\n") {
			t.Fatalf("the scene changed within one second (+%v)", offset)
		}
	}

	later := scene(t, state, now.Add(time.Second), 60)
	if strings.Join(later, "\n") == strings.Join(first, "\n") {
		t.Fatal("the scene never changes between seconds")
	}
}

// Reduced motion stills the smoke instead of removing the censer.
func TestMotionOffStillsTheScene(t *testing.T) {
	still := func(at time.Time) []string {
		return Scene(working(time.Minute), SceneOptions{Now: at, Columns: 60})
	}

	first := still(now)
	if len(first) != smokeRows+emberRow+len(censer)+1 {
		t.Fatalf("motion off changed the scene:\n%s", strings.Join(first, "\n"))
	}
	if strings.Join(still(now.Add(3*time.Second)), "\n") != strings.Join(first, "\n") {
		t.Fatal("the scene still moves with motion off")
	}
}

// An idle session smokes more thinly than a working one, so the scene reads
// as activity without being read. Counting puffs cannot say so any more - the
// footprint is fixed on purpose - so the density does.
func TestSmokeFollowsThePhase(t *testing.T) {
	puffs := func(phase contracts.SessionPhase) int {
		state := working(time.Minute)
		state.Phase = phase

		total := 0
		for second := 0; second < 60; second++ {
			rows := scene(t, state, now.Add(time.Duration(second)*time.Second), 60)[:smokeRows]
			total += strings.Count(strings.Join(rows, ""), "▒")
		}
		return total
	}

	if !(puffs(contracts.PhaseWorking) > puffs(contracts.PhaseTurnCompleted)) {
		t.Fatalf("working smokes %d, done smokes %d",
			puffs(contracts.PhaseWorking), puffs(contracts.PhaseTurnCompleted))
	}
}

// Every row of the censer sits on the same axis. They used to be centred one
// cell apart from each other, which read as a wobble rather than a shape.
func TestTheCenserRowsShareOneAxis(t *testing.T) {
	var axis float64
	for i, row := range censer {
		runes := []rune(row)

		first, last := -1, -1
		for x, r := range runes {
			if r != ' ' {
				if first < 0 {
					first = x
				}
				last = x
			}
		}
		if first < 0 {
			t.Fatalf("censer row %d is blank", i)
		}
		if width(row) != CenserWidth {
			t.Fatalf("censer row %d is %d cells, want %d: %q", i, width(row), CenserWidth, row)
		}

		centre := float64(first+last) / 2
		if i == 0 {
			axis = centre
		} else if centre != axis {
			t.Fatalf("censer row %d is centred on %.1f, the first on %.1f", i, centre, axis)
		}
	}
}

// The scene must be exactly as tall every second. Smoke used to land on random
// rows, so whenever the top one stayed empty it collapsed - the scene was 9,
// 10, or 11 rows depending on the second, and the prompt under it moved.
func TestTheSceneNeverChangesHeight(t *testing.T) {
	for _, state := range []contracts.SessionState{
		working(time.Minute), praying(now), idle(),
	} {
		for second := 0; second < 120; second++ {
			lines := scene(t, state, now.Add(time.Duration(second)*time.Second), 80)

			if len(lines) != smokeRows+emberRow+len(censer)+1 {
				t.Fatalf("second %d drew %d rows", second, len(lines))
			}
			for y, row := range lines {
				if row == "" {
					t.Fatalf("second %d left row %d blank, which collapses the scene:\n%s",
						second, y, strings.Join(lines, "\n"))
				}
			}
		}
	}
}

// The smoke may change density but never position. Two earlier versions moved
// it - onto random rows, then one column either way - and both made the plume
// shift every second directly above the line the user types on. That is the
// complaint this whole redraw started from, and it came back twice.
func TestSmokeNeverMoves(t *testing.T) {
	occupied := func(second int) string {
		lines := scene(t, working(time.Minute), now.Add(time.Duration(second)*time.Second), 80)

		var cells []string
		for y, row := range lines[:smokeRows] {
			for x, r := range []rune(row) {
				if r != ' ' {
					cells = append(cells, fmt.Sprintf("%d,%d", x, y))
				}
			}
		}
		return strings.Join(cells, " ")
	}

	first := occupied(0)
	shapes := map[string]bool{}

	for second := 0; second < 120; second++ {
		if got := occupied(second); got != first {
			t.Fatalf("second %d put smoke at [%s], second 0 at [%s]", second, got, first)
		}
		rows := scene(t, working(time.Minute), now.Add(time.Duration(second)*time.Second), 80)
		shapes[strings.Join(rows[:smokeRows], "")] = true
	}

	// Fixed in place, but not frozen: the user noticed at once when an earlier
	// version stopped moving altogether.
	if len(shapes) < 4 {
		t.Fatalf("the smoke only ever drew %d different patterns", len(shapes))
	}
}
