package render

import (
	"strings"
	"testing"

	"github.com/cruellaDev/claude-code-prayops/contracts"
	"github.com/cruellaDev/claude-code-prayops/internal/layout"
)

func frame(t *testing.T, width, height int, state contracts.SessionState) []string {
	t.Helper()
	return strings.Split(Frame(layout.Compute(width, height), state, Options{}), "\n")
}

func working() contracts.SessionState {
	return contracts.SessionState{
		Phase:       contracts.PhaseWorking,
		Host:        contracts.HostClaude,
		ProjectName: "payment-api",
	}
}

// Box drawing characters are East Asian ambiguous. If their width followed the
// user's locale, every one of them would claim two cells on a Korean or
// Japanese system and erase its neighbour - which is exactly what happened
// before the width condition was pinned.
func TestBoxDrawingIsOneCell(t *testing.T) {
	for _, r := range []rune{'─', '│', '╭', '╮', '╰', '╯', '┌', '┐', '└', '┘', '░', '╹'} {
		if got := narrow.RuneWidth(r); got != 1 {
			t.Fatalf("RuneWidth(%q) = %d, want 1", r, got)
		}
	}
	for _, r := range []rune{'한', '글', '香', '祈'} {
		if got := narrow.RuneWidth(r); got != 2 {
			t.Fatalf("RuneWidth(%q) = %d, want 2", r, got)
		}
	}
}

// The strongest invariant available: every rendered line is exactly as wide as
// the terminal. A width miscount anywhere shows up here.
func TestEveryLineIsExactlyTerminalWidth(t *testing.T) {
	for width := layout.MinWidth; width <= 100; width += 3 {
		for height := layout.MinHeight; height <= 30; height += 2 {
			lines := frame(t, width, height, working())

			if len(lines) != height {
				t.Fatalf("%dx%d produced %d lines", width, height, len(lines))
			}
			for i, line := range lines {
				if got := narrow.StringWidth(line); got != width {
					t.Fatalf("%dx%d line %d is %d cells, want %d:\n%q", width, height, i, got, width, line)
				}
			}
		}
	}
}

func TestFrameIsBordered(t *testing.T) {
	lines := frame(t, 56, 18, working())

	if !strings.HasPrefix(lines[0], "╭") || !strings.HasSuffix(lines[0], "╮") {
		t.Fatalf("top border = %q", lines[0])
	}
	last := lines[len(lines)-1]
	if !strings.HasPrefix(last, "╰") || !strings.HasSuffix(last, "╯") {
		t.Fatalf("bottom border = %q", last)
	}
	if !strings.Contains(lines[0], Title) {
		t.Fatalf("no title in %q", lines[0])
	}
	for i, line := range lines[1 : len(lines)-1] {
		if !strings.HasPrefix(line, "│") || !strings.HasSuffix(line, "│") {
			t.Fatalf("line %d has no side border: %q", i+1, line)
		}
	}
}

// The Korean phrases are the widest thing drawn. If they were counted as one
// cell each they would push the right border off the screen.
func TestKoreanPhrasesDoNotBreakTheBorder(t *testing.T) {
	for phase := range phrases {
		state := working()
		state.Phase = phase

		for _, line := range frame(t, 56, 18, state) {
			if narrow.StringWidth(line) != 56 {
				t.Fatalf("phase %q produced a %d cell line: %q",
					phase, narrow.StringWidth(line), line)
			}
		}
	}
}

func TestSceneContainsTheFurniture(t *testing.T) {
	rendered := Frame(layout.Compute(56, 18), working(), Options{})

	for name, glyphs := range map[string]string{
		"burner":  "┌───────┐",
		"ash":     "░",
		"plates":  "(____)",
		"incense": "╹",
	} {
		if !strings.Contains(rendered, glyphs) {
			t.Fatalf("no %s in the frame:\n%s", name, rendered)
		}
	}
	if strings.Count(rendered, "(____)") != 3 {
		t.Fatalf("want 3 plates:\n%s", rendered)
	}
	if strings.Count(rendered, "╹") != 3 {
		t.Fatalf("want 3 incense sticks:\n%s", rendered)
	}
}

func TestStatusShowsPhaseAndProject(t *testing.T) {
	rendered := Frame(layout.Compute(56, 18), working(), Options{})

	if !strings.Contains(rendered, "WORKING") {
		t.Fatalf("no phase:\n%s", rendered)
	}
	if !strings.Contains(rendered, Phrase(contracts.PhaseWorking)) {
		t.Fatalf("no ceremonial phrase:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Claude · payment-api") {
		t.Fatalf("no host and project:\n%s", rendered)
	}
}

func TestToolFailureIsMarked(t *testing.T) {
	state := working()
	state.ToolErrored = true

	if !strings.Contains(Frame(layout.Compute(56, 18), state, Options{}), "WORKING!") {
		t.Fatal("a failed tool is not marked")
	}
}

func TestCodexHostIsNamed(t *testing.T) {
	state := working()
	state.Host = contracts.HostCodex

	if !strings.Contains(Frame(layout.Compute(56, 18), state, Options{}), "Codex") {
		t.Fatal("the Codex host is not named")
	}
}

// The compact layout has one status row, so the phrase is dropped rather than
// truncated into nonsense.
func TestCompactLayoutDropsThePhrase(t *testing.T) {
	rendered := Frame(layout.Compute(30, 9), working(), Options{})

	if !strings.Contains(rendered, "WORKING") {
		t.Fatalf("no phase:\n%s", rendered)
	}
	if strings.Contains(rendered, Phrase(contracts.PhaseWorking)) {
		t.Fatalf("the compact layout kept the phrase:\n%s", rendered)
	}
}

func TestUnknownPhaseFallsBackToIdle(t *testing.T) {
	rendered := Frame(layout.Compute(56, 18), contracts.SessionState{}, Options{})

	if !strings.Contains(rendered, Phrase(contracts.PhaseIdle)) {
		t.Fatalf("no idle phrase:\n%s", rendered)
	}
}

// A resize can briefly hand the renderer a size with no room for anything.
func TestTinyTerminalsDoNotPanic(t *testing.T) {
	for _, size := range [][2]int{{0, 0}, {1, 1}, {2, 2}, {5, 3}, {23, 7}} {
		rendered := Frame(layout.Compute(size[0], size[1]), working(), Options{})
		if strings.Contains(rendered, "\x00") {
			t.Fatalf("%v leaked a continuation cell: %q", size, rendered)
		}
	}
}

func TestSmokeIsDrawnBehindTheFurniture(t *testing.T) {
	l := layout.Compute(56, 18)
	opts := Options{Smoke: []Particle{
		{X: l.Burner.X + 1, Y: l.Burner.Y + 1, Glyph: '~'}, // inside the burner
		{X: l.Content.X + 2, Y: l.Content.Y + 2, Glyph: '~'},
	}}

	lines := strings.Split(Frame(l, working(), opts), "\n")

	if !strings.Contains(lines[l.Content.Y+2], "~") {
		t.Fatalf("smoke in open space was not drawn:\n%s", lines[l.Content.Y+2])
	}
	if strings.Contains(lines[l.Burner.Y+1], "~") {
		t.Fatalf("smoke was drawn over the burner:\n%s", lines[l.Burner.Y+1])
	}
}

func TestCanvasHandlesWideRunes(t *testing.T) {
	c := NewCanvas(6, 1)
	c.Text(0, 0, "한글")

	if got := c.Lines()[0]; got != "한글" {
		t.Fatalf("line = %q", got)
	}
	if got := narrow.StringWidth(c.Lines()[0]); got != 4 {
		t.Fatalf("width = %d, want 4", got)
	}

	// Overwriting the first half of a wide rune must not leave its second half
	// orphaned on screen.
	c.Set(0, 0, 'x')
	if got := c.Lines()[0]; got != "x 글" {
		t.Fatalf("after overwrite = %q, want %q", got, "x 글")
	}
}

func TestCanvasDropsOutOfBoundsWrites(t *testing.T) {
	c := NewCanvas(4, 2)
	c.Set(-1, 0, 'x')
	c.Set(0, -1, 'x')
	c.Set(9, 0, 'x')
	c.Set(0, 9, 'x')
	c.Set(3, 0, '한') // no room for the second half

	for _, line := range c.Lines() {
		if strings.ContainsAny(line, "x한") {
			t.Fatalf("an out of bounds write landed: %q", line)
		}
	}
}
