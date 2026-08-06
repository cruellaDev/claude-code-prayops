// Package statusline renders the compact Claude Code status line.
//
// This is a snapshot, not an animation: Claude Code re-runs the command on its
// own refresh interval, and the full incense and dissolve animation lives in
// `prayops watch`.
package statusline

import (
	"strconv"
	"strings"
	"time"

	"github.com/cruellaDev/claude-code-prayops/contracts"
	"github.com/mattn/go-runewidth"
)

// BurnDuration is how long one stick of incense lasts. It is the scale for the
// percentage only - nothing expires when it reaches zero.
const BurnDuration = 10 * time.Minute

// DefaultColumns is assumed when the terminal width is unknown.
const DefaultColumns = 80

// compactBelow is the width under which the label and separators are dropped.
const compactBelow = 34

// The label carried 祈 and 香 for no better reason than that a censer is an
// East Asian object. They are not a requirement, and a reader who does not
// know the characters gets nothing from them.
const sep = " · "

// ANSI colours, kept to the four the design calls for.
const (
	ansiReset = "\x1b[0m"
	ansiRed   = "\x1b[38;5;174m" // ritual red
	ansiEmber = "\x1b[38;5;179m" // muted gold
	ansiGray  = "\x1b[38;5;245m" // smoke
	ansiGreen = "\x1b[38;5;108m"
)

// phaseLabels are the short labels shown in the status line. The longer
// ceremonial phrases belong to the full TUI.
var phaseLabels = map[contracts.SessionPhase]string{
	contracts.PhaseIdle:             "IDLE",
	contracts.PhaseThinking:         "THINKING",
	contracts.PhaseWorking:          "WORKING",
	contracts.PhaseApprovalRequired: "APPROVAL",
	contracts.PhaseTurnCompleted:    "DONE",
	contracts.PhaseTurnFailed:       "FAILED",
	contracts.PhaseSessionEnded:     "ENDED",
}

// burning reports whether a turn is in flight. The incense percentage is only
// meaningful while one is.
func burning(phase contracts.SessionPhase) bool {
	switch phase {
	case contracts.PhaseThinking, contracts.PhaseWorking, contracts.PhaseApprovalRequired:
		return true
	default:
		return false
	}
}

// Remaining is how much of the current stick is left, from 100 down to 0.
func Remaining(state contracts.SessionState, now time.Time) int {
	if state.TurnStartedAt.IsZero() {
		return 100
	}

	elapsed := now.Sub(state.TurnStartedAt)
	if elapsed <= 0 {
		return 100
	}
	if elapsed >= BurnDuration {
		return 0
	}
	return 100 - int(elapsed*100/BurnDuration)
}

// Options controls rendering. Colour is off by default so the zero value is
// the safe one.
type Options struct {
	Now     time.Time
	Columns int
	Color   bool
}

// Render returns the single status line for a session.
func Render(state contracts.SessionState, opts Options) string {
	columns := opts.Columns
	if columns <= 0 {
		columns = DefaultColumns
	}

	// An unset phase - the state before the first hook fires - is idle. The
	// fallback happens once so the label and the colour cannot disagree.
	phase := state.Phase
	label, known := phaseLabels[phase]
	if !known {
		phase = contracts.PhaseIdle
		label = phaseLabels[phase]
	}
	if state.ToolErrored && burning(phase) {
		label += "!"
	}

	var percent string
	if burning(phase) {
		percent = strconv.Itoa(Remaining(state, opts.Now)) + "%"
	}

	full := full(label, percent)
	if width(full) <= columns {
		return colorize(full, phase, opts.Color)
	}
	return colorize(compact(label, percent), phase, opts.Color)
}

func full(label, percent string) string {
	parts := []string{"PrayOps", label}
	if percent != "" {
		parts = append(parts, percent)
	}
	return strings.Join(parts, sep)
}

func compact(label, percent string) string {
	if percent == "" {
		return label
	}
	return label + " " + percent
}

func colorize(line string, phase contracts.SessionPhase, color bool) string {
	if !color {
		return line
	}

	var tone string
	switch phase {
	case contracts.PhaseApprovalRequired:
		tone = ansiEmber
	case contracts.PhaseTurnFailed:
		tone = ansiRed
	case contracts.PhaseTurnCompleted:
		tone = ansiGreen
	case contracts.PhaseSessionEnded, contracts.PhaseIdle:
		tone = ansiGray
	default:
		tone = ansiRed
	}
	return tone + line + ansiReset
}

// width counts terminal cells, treating East Asian ambiguous runes as one.
//
// The condition is pinned rather than detected: box drawing and similar
// ambiguous runes would otherwise change width with the user's locale.
func width(s string) int { return narrow.StringWidth(s) }

var narrow = &runewidth.Condition{EastAsianWidth: false}
