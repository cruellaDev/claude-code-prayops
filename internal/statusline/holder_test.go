package statusline

import (
	"strings"
	"testing"
	"time"

	"github.com/cruellaDev/claude-code-prayops/contracts"
)

func holderScene(t *testing.T, state contracts.SessionState, at time.Time, columns int) []string {
	t.Helper()
	return Scene(state, SceneOptions{
		Now: at, Columns: columns, Motion: true, Theme: ThemeHolder})
}

// The stick is the progress bar. That is the whole point of this theme: the
// other two decorate, this one reports.
func TestTheStickIsAsLongAsTheTurnHasLeft(t *testing.T) {
	stick := func(minutes int) int {
		lines := holderScene(t, working(time.Duration(minutes)*time.Minute), now, 60)
		return strings.Count(lines[holderSmokeRows+stickRow], "▄")
	}

	full := stick(0)
	if full < 10 {
		t.Fatalf("a fresh stick is only %d cells", full)
	}

	last := full + 1
	for _, minutes := range []int{0, 2, 4, 6, 8, 10} {
		got := stick(minutes)
		if got >= last {
			t.Fatalf("after %d minutes the stick is %d cells, was %d", minutes, got, last)
		}
		last = got
	}
	if last != 0 {
		t.Fatalf("the stick never burned out: %d cells left", last)
	}
}

// What it burns away it leaves behind, so the picture says "this has been
// going a while" rather than "there is a short stick here".
func TestBurntStickLeavesAsh(t *testing.T) {
	row := holderScene(t, working(6*time.Minute), now, 60)[holderSmokeRows+stickRow]

	if !strings.ContainsRune(row, Ash) {
		t.Fatalf("a half burnt stick left no ash: %q", row)
	}
	// Ash to the right of what is left, never the other way round.
	if strings.Index(row, string(Ash)) < strings.LastIndex(row, "▄") {
		t.Fatalf("the ash is on the wrong side of the ember: %q", row)
	}
}

// The lit end is the one coloured thing, and it is where the smoke comes from.
func TestTheEmberIsRedAndCarriesTheSmoke(t *testing.T) {
	for _, minutes := range []int{0, 5} {
		state := working(time.Duration(minutes) * time.Minute)

		lines := Scene(state, SceneOptions{
			Now: now, Columns: 60, Color: true, Motion: true, Theme: ThemeHolder})
		if !strings.Contains(strings.Join(lines, ""), ansiEmberTip) {
			t.Fatalf("after %d minutes nothing is lit", minutes)
		}

		// The smoke sits over the ember, not over where it started.
		plain := holderScene(t, state, now, 60)
		ember := strings.LastIndex(plain[holderSmokeRows+stickRow], "▄")
		smoke := strings.IndexAny(plain[holderSmokeRows-1], "░▒")

		if smoke < ember-2 || smoke > ember+2 {
			t.Fatalf("after %d minutes the ember is at %d and the smoke at %d", minutes, ember, smoke)
		}
	}
}

// The same invariant as the other two themes, for the same reason.
func TestTheHolderNeverChangesHeight(t *testing.T) {
	for _, minutes := range []int{0, 3, 7, 10, 20} {
		state := working(time.Duration(minutes) * time.Minute)

		for second := 0; second < 60; second++ {
			lines := holderScene(t, state, now.Add(time.Duration(second)*time.Second), 80)

			if len(lines) != HolderHeight()+1 {
				t.Fatalf("%d minutes in, second %d drew %d rows", minutes, second, len(lines))
			}
			for y, row := range lines {
				if row == "" {
					t.Fatalf("%d minutes in, second %d left row %d blank", minutes, second, y)
				}
			}
		}
	}
}

// A stick that has burned out stops smoking. It gave a full plume off a tray
// of ash for a while, which read as an incense that never runs out.
func TestBurntOutIncenseStopsSmoking(t *testing.T) {
	plume := func(minutes int) int {
		lines := holderScene(t, working(time.Duration(minutes)*time.Minute), now, 60)

		total := 0
		for _, row := range lines[:holderSmokeRows] {
			total += strings.Count(row, "░") + strings.Count(row, "▒")
		}
		return total
	}

	if plume(1) == 0 {
		t.Fatal("a fresh stick is not smoking")
	}
	for _, minutes := range []int{10, 20, 60} {
		if got := plume(minutes); got != 0 {
			t.Fatalf("after %d minutes the ash is still giving %d puffs", minutes, got)
		}
	}
}

// The stick burns by the context window when the host reports one, and by the
// clock when it does not. The unset case matters: an int field would make
// "the host said nothing" and "there is nothing left" the same value, and
// every caller that forgot the field would draw a burnt-out stick.
func TestTheStickPrefersTheContextWindow(t *testing.T) {
	sticks := func(opts SceneOptions) int {
		lines := Scene(working(time.Minute), opts)
		return strings.Count(lines[holderSmokeRows+stickRow], "▄")
	}

	base := SceneOptions{Now: now, Columns: 60, Motion: true, Theme: ThemeHolder}

	// Unset: the clock, which after one minute is nearly full.
	byClock := sticks(base)
	if byClock < 10 {
		t.Fatalf("with no context reported the stick is only %d cells", byClock)
	}

	// A full context is a full stick, which after a minute the clock is not.
	full, empty := 100, 0
	base.Fuel = &full
	if got := sticks(base); got <= byClock {
		t.Fatalf("a full context gave %d cells, the clock after a minute gave %d", got, byClock)
	}

	base.Fuel = &empty
	if got := sticks(base); got != 0 {
		t.Fatalf("an exhausted context left %d cells of stick", got)
	}
}
