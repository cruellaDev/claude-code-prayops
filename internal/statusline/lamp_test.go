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

// Rubbing first, then an answer, and the answer holds for the whole effect -
// a wish that flickered between granted and not would read as a bug.
func TestAWishRubsThenAnswersConsistently(t *testing.T) {
	var flipped int

	for k := 0; k < 60; k++ {
		state := praying(now.Add(time.Duration(k) * time.Second))

		var answers []string
		for second := 0; second < int(PrayerShows/time.Second); second++ {
			at := state.LastPrayerAt.Add(time.Duration(second) * time.Second)
			label := lampScene(t, state, at, 80)[LampHeight()]

			switch {
			case second < rubSeconds:
				if !strings.Contains(label, "RUBBING") {
					t.Fatalf("second %d of the wish says %q, want RUBBING", second, label)
				}
			case strings.Contains(label, "GRANTED"), strings.Contains(label, "NOTHING"):
				answers = append(answers, label)
			default:
				t.Fatalf("second %d of the wish says %q", second, label)
			}
		}

		for _, answer := range answers[1:] {
			if answer != answers[0] {
				t.Fatalf("the wish changed its mind: %q then %q", answers[0], answer)
			}
		}
		if strings.Contains(answers[0], "NOTHING") {
			flipped++
		}
	}

	// Both outcomes have to actually happen, or the chance is not a chance.
	if flipped == 0 || flipped == 60 {
		t.Fatalf("%d of 60 wishes failed - the outcome is not random", flipped)
	}
}

// After the effect the lamp goes back to smoking, rather than staying stuck on
// the last frame of the wish.
func TestTheWishEnds(t *testing.T) {
	state := praying(now)
	after := lampScene(t, state, now.Add(PrayerShows), 80)

	label := after[LampHeight()]
	for _, word := range []string{"RUBBING", "GRANTED", "NOTHING"} {
		if strings.Contains(label, word) {
			t.Fatalf("the wish is still showing %q after it ended", word)
		}
	}
	if strings.Contains(strings.Join(after, ""), Rub) {
		t.Fatal("the hand is still on the lamp after the wish ended")
	}
}
