package statusline

import (
	"strings"
	"testing"
	"time"

	"github.com/cruellaDev/claude-code-prayops/contracts"
)

var now = time.Unix(1700000000, 0).UTC()

func working(elapsed time.Duration) contracts.SessionState {
	return contracts.SessionState{
		SessionID:     "s1",
		Phase:         contracts.PhaseWorking,
		TurnStartedAt: now.Add(-elapsed),
		UpdatedAt:     now,
	}
}

func TestFullLine(t *testing.T) {
	// 10 minutes of incense, 3.6 minutes elapsed, so 64% is left.
	got := Render(working(216*time.Second), Options{Now: now, Columns: 80})

	if got != "祈 PrayOps · WORKING · 香 64%" {
		t.Fatalf("line = %q", got)
	}
}

func TestCompactLineOnANarrowTerminal(t *testing.T) {
	got := Render(working(216*time.Second), Options{Now: now, Columns: 20})

	if got != "祈 WORKING 64%" {
		t.Fatalf("line = %q", got)
	}
}

// The line has to fit the terminal it is given, counting CJK glyphs as two
// cells rather than one.
func TestLineFitsTheTerminalWidth(t *testing.T) {
	for columns := 12; columns <= 80; columns++ {
		got := Render(working(0), Options{Now: now, Columns: columns})
		if width(got) > columns && columns >= compactBelow {
			t.Fatalf("columns=%d produced a %d cell line: %q", columns, width(got), got)
		}
		if strings.Contains(got, "\n") {
			t.Fatalf("columns=%d produced more than one line: %q", columns, got)
		}
	}
}

func TestWidthCountsWideRunes(t *testing.T) {
	cases := map[string]int{
		"祈":         2,
		"香":         2,
		"WORKING":   7,
		"祈 PrayOps": 10,
		"한글":        4,
	}
	for input, want := range cases {
		if got := width(input); got != want {
			t.Fatalf("width(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestIncenseBurnsDown(t *testing.T) {
	cases := []struct {
		elapsed time.Duration
		want    int
	}{
		{0, 100},
		{BurnDuration / 4, 75},
		{BurnDuration / 2, 50},
		{BurnDuration, 0},
		{2 * BurnDuration, 0}, // never negative
	}

	for _, tc := range cases {
		got := Remaining(working(tc.elapsed), now)
		if got != tc.want {
			t.Fatalf("after %v: %d%%, want %d%%", tc.elapsed, got, tc.want)
		}
	}
}

// A clock that jumps backwards must not produce a percentage above 100.
func TestIncenseIgnoresABackwardsClock(t *testing.T) {
	state := working(-time.Hour)
	if got := Remaining(state, now); got != 100 {
		t.Fatalf("remaining = %d, want 100", got)
	}
}

// The percentage only means something while a turn is running.
func TestIncenseIsOmittedWhenNoTurnIsRunning(t *testing.T) {
	for _, phase := range []contracts.SessionPhase{
		contracts.PhaseIdle,
		contracts.PhaseTurnCompleted,
		contracts.PhaseTurnFailed,
		contracts.PhaseSessionEnded,
	} {
		state := working(time.Minute)
		state.Phase = phase

		got := Render(state, Options{Now: now, Columns: 80})
		if strings.Contains(got, incense) || strings.Contains(got, "%") {
			t.Fatalf("phase %q showed incense: %q", phase, got)
		}
	}
}

func TestPhaseLabels(t *testing.T) {
	cases := map[contracts.SessionPhase]string{
		contracts.PhaseIdle:             "IDLE",
		contracts.PhaseThinking:         "THINKING",
		contracts.PhaseWorking:          "WORKING",
		contracts.PhaseApprovalRequired: "APPROVAL",
		contracts.PhaseTurnCompleted:    "DONE",
		contracts.PhaseTurnFailed:       "FAILED",
		contracts.PhaseSessionEnded:     "ENDED",
	}

	for phase, want := range cases {
		state := contracts.SessionState{Phase: phase}
		got := Render(state, Options{Now: now, Columns: 80})
		if !strings.Contains(got, want) {
			t.Fatalf("phase %q rendered %q, want it to contain %q", phase, got, want)
		}
	}
}

// An empty state is what the status line sees before the first hook fires.
// The label and the colour must fall back together, or an idle session shows
// up in the colour of a running one.
func TestUnknownPhaseFallsBackToIdle(t *testing.T) {
	got := Render(contracts.SessionState{}, Options{Now: now, Columns: 80})
	if !strings.Contains(got, "IDLE") {
		t.Fatalf("line = %q", got)
	}

	unset := Render(contracts.SessionState{}, Options{Now: now, Columns: 80, Color: true})
	idle := Render(contracts.SessionState{Phase: contracts.PhaseIdle}, Options{Now: now, Columns: 80, Color: true})
	if unset != idle {
		t.Fatalf("an unset phase renders differently from idle:\n unset %q\n idle  %q", unset, idle)
	}
}

func TestToolFailureIsMarked(t *testing.T) {
	state := working(time.Minute)
	state.ToolErrored = true

	got := Render(state, Options{Now: now, Columns: 80})
	if !strings.Contains(got, "WORKING!") {
		t.Fatalf("line = %q", got)
	}
}

// NO_COLOR and non-colour terminals get shape and text only.
func TestColorIsOptional(t *testing.T) {
	plain := Render(working(0), Options{Now: now, Columns: 80})
	if strings.Contains(plain, "\x1b[") {
		t.Fatalf("colour leaked into a plain line: %q", plain)
	}

	colored := Render(working(0), Options{Now: now, Columns: 80, Color: true})
	if !strings.HasPrefix(colored, "\x1b[") || !strings.HasSuffix(colored, ansiReset) {
		t.Fatalf("coloured line = %q", colored)
	}
	if !strings.Contains(colored, "WORKING") {
		t.Fatalf("colour replaced the text: %q", colored)
	}
}

func TestUnknownWidthUsesADefault(t *testing.T) {
	got := Render(working(216*time.Second), Options{Now: now})
	if got != "祈 PrayOps · WORKING · 香 64%" {
		t.Fatalf("line = %q", got)
	}
}
