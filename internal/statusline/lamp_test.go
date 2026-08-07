package statusline

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cruellaDev/claude-code-prayops/contracts"
)

func lampScene(t *testing.T, state contracts.SessionState, at time.Time, columns int) []string {
	t.Helper()
	return Scene(state, SceneOptions{
		Now: at, Columns: columns, Motion: true, Theme: ThemeLamp})
}

// A theme is only a theme if choosing it changes the picture.
func TestTheLampIsADifferentPicture(t *testing.T) {
	lamp := strings.Join(lampScene(t, working(time.Minute), now, 60), "\n")

	for name, glyphs := range map[string]string{
		"finial": "▄",
		"lid":    "███",
		"handle": "▗▄▖",
		"spout":  "▄▄▄▄▄▄▄▄▄▄▖",
		"foot":   "▄▄▄███▄▄▄",
	} {
		if !strings.Contains(lamp, glyphs) {
			t.Fatalf("no %s in the lamp:\n%s", name, lamp)
		}
	}

	censer := strings.Join(scene(t, working(time.Minute), now, 60), "\n")
	if lamp == censer {
		t.Fatal("the lamp draws the censer")
	}
	// The censer's incense has no business in a lamp.
	if strings.Contains(lamp, "▮ ▮ ▮") {
		t.Fatalf("the lamp is drawing incense sticks:\n%s", lamp)
	}
}

// The same invariant the censer has, for the same reason: a scene that loses
// height moves the prompt underneath it.
func TestTheLampNeverChangesHeight(t *testing.T) {
	for _, state := range []contracts.SessionState{
		working(time.Minute), praying(now), idle(),
	} {
		for second := 0; second < 120; second++ {
			at := now.Add(time.Duration(second) * time.Second)
			lines := lampScene(t, state, at, 80)

			if len(lines) != LampHeight()+1 {
				t.Fatalf("second %d drew %d rows, want %d", second, len(lines), LampHeight()+1)
			}
			for y, row := range lines {
				if row == "" {
					t.Fatalf("second %d left row %d blank:\n%s", second, y, strings.Join(lines, "\n"))
				}
			}
		}
	}
}

// The idle plume holds still. Three earlier versions of the censer's smoke
// moved and all three had to be taken out again.
func TestTheLampPlumeNeverMoves(t *testing.T) {
	occupied := func(second int) string {
		lines := lampScene(t, working(time.Minute), now.Add(time.Duration(second)*time.Second), 80)

		var cells []string
		for y, row := range lines[:lampSmokeRows] {
			for x, r := range []rune(row) {
				if r != ' ' && r != blank {
					cells = append(cells, fmt.Sprintf("%d,%d", x, y))
				}
			}
		}
		return strings.Join(cells, " ")
	}

	first := occupied(0)
	for second := 0; second < 60; second++ {
		if got := occupied(second); got != first {
			t.Fatalf("second %d put smoke at [%s], second 0 at [%s]", second, got, first)
		}
	}
}

// The answer comes at once and holds for the whole effect - a wish that
// flickered between granted and not would read as a bug.
func TestAWishAnswersAtOnceAndConsistently(t *testing.T) {
	var failed int

	for k := 0; k < 60; k++ {
		state := praying(now.Add(time.Duration(k) * time.Second))

		var answers []string
		for second := 0; second < int(PrayerShows/time.Second); second++ {
			at := state.LastPrayerAt.Add(time.Duration(second) * time.Second)
			label := lampScene(t, state, at, 80)[LampHeight()]

			if !strings.Contains(label, "GRANTED") && !strings.Contains(label, "NOTHING") {
				t.Fatalf("second %d of the wish says %q", second, label)
			}
			answers = append(answers, label)
		}

		for _, answer := range answers[1:] {
			if answer != answers[0] {
				t.Fatalf("the wish changed its mind: %q then %q", answers[0], answer)
			}
		}
		if strings.Contains(answers[0], "NOTHING") {
			failed++
		}
	}

	// Both outcomes have to actually happen, or the chance is not a chance.
	if failed == 0 || failed == 60 {
		t.Fatalf("%d of 60 wishes failed - the outcome is not random", failed)
	}
}

// A granted wish rises as hearts; a failed one goes off like a firework.
func TestTheTwoAnswersLookDifferent(t *testing.T) {
	var hearts, sparks bool

	for k := 0; k < 60 && !(hearts && sparks); k++ {
		state := praying(now.Add(time.Duration(k) * time.Second))
		scene := strings.Join(lampScene(t, state, state.LastPrayerAt.Add(time.Second), 80), "")

		if granted(state) {
			if !strings.ContainsRune(scene, Heart) {
				t.Fatalf("a granted wish drew no hearts:\n%s", scene)
			}
			if strings.ContainsRune(scene, Spark) {
				t.Fatalf("a granted wish drew sparks:\n%s", scene)
			}
			hearts = true
			continue
		}
		if !strings.ContainsRune(scene, Spark) {
			t.Fatalf("a failed wish drew no sparks:\n%s", scene)
		}
		if strings.ContainsRune(scene, Heart) {
			t.Fatalf("a failed wish drew hearts:\n%s", scene)
		}
		sparks = true
	}

	if !hearts || !sparks {
		t.Fatal("only one of the two answers ever happened")
	}
}

// After the effect the lamp goes back to smoking, rather than staying stuck on
// the last frame of the wish.
func TestTheWishEnds(t *testing.T) {
	state := praying(now)
	after := lampScene(t, state, now.Add(PrayerShows), 80)

	label := after[LampHeight()]
	for _, word := range []string{"GRANTED", "NOTHING"} {
		if strings.Contains(label, word) {
			t.Fatalf("the wish is still showing %q after it ended", word)
		}
	}
	joined := strings.Join(after, "")
	if strings.ContainsRune(joined, Heart) || strings.ContainsRune(joined, Spark) {
		t.Fatal("the answer is still on screen after the wish ended")
	}
}

// A finished turn sets the lamp off by itself. A slash command costs a whole
// model turn before anything can be drawn; the hook that records the end of a
// turn does not, so this is the version that feels immediate.
func TestTheLampAnswersAFinishedTurn(t *testing.T) {
	for phase, want := range map[contracts.SessionPhase]string{
		contracts.PhaseTurnCompleted: "GRANTED",
		contracts.PhaseTurnFailed:    "NOTHING",
	} {
		state := working(time.Minute)
		state.Phase = phase
		state.UpdatedAt = now

		if got := lampScene(t, state, now, 80)[LampHeight()]; !strings.Contains(got, want) {
			t.Fatalf("a %s turn says %q, want %s", phase, got, want)
		}

		// And it is a report, not a wish: the same phase always says the same
		// thing, however many times it happens.
		for k := 1; k < 20; k++ {
			state.UpdatedAt = now.Add(time.Duration(k) * time.Minute)
			at := state.UpdatedAt.Add(time.Second)

			if got := lampScene(t, state, at, 80)[LampHeight()]; !strings.Contains(got, want) {
				t.Fatalf("turn %d of phase %s says %q, want %s", k, phase, got, want)
			}
		}
	}
}

// The reaction is over as quickly as any other, so a session that ended an
// hour ago is not still showering hearts.
func TestAFinishedTurnStopsAnsweringAfterAWhile(t *testing.T) {
	state := working(time.Minute)
	state.Phase = contracts.PhaseTurnCompleted
	state.UpdatedAt = now

	label := lampScene(t, state, now.Add(PrayerShows), 80)[LampHeight()]
	for _, word := range []string{"GRANTED", "NOTHING"} {
		if strings.Contains(label, word) {
			t.Fatalf("still answering %q long after the turn ended", word)
		}
	}
}

// A prayer sent during a turn is not overruled the instant the turn ends.
func TestTheMoreRecentTriggerWins(t *testing.T) {
	state := working(time.Minute)
	state.Phase = contracts.PhaseTurnFailed
	state.UpdatedAt = now
	state.LastPrayerAt = now.Add(time.Second)

	at, asked, ok := effectStart(state)
	if !ok || !asked || !at.Equal(state.LastPrayerAt) {
		t.Fatalf("effectStart = %v %v %v, want the prayer", at, asked, ok)
	}

	state.LastPrayerAt = now.Add(-time.Second)
	at, asked, ok = effectStart(state)
	if !ok || asked || !at.Equal(state.UpdatedAt) {
		t.Fatalf("effectStart = %v %v %v, want the turn", at, asked, ok)
	}
}
