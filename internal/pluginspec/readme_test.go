package pluginspec

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/cruellaDev/claude-code-prayops/internal/statusline"
)

// The README opened on a censer that had not existed for three releases,
// because the art was transcribed by hand every time it changed. It is the
// program's own output now, and this keeps it that way.
func TestReadmeShowsTheCenserThatShips(t *testing.T) {
	raw, err := os.ReadFile(repoRoot + "/README.md")
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	readme := string(raw)

	blocks := regexp.MustCompile("(?s)```text\n(.*?)```").FindAllStringSubmatch(readme, -1)
	if len(blocks) == 0 {
		t.Fatal("the README has no text blocks")
	}
	hero := blocks[0][1]

	// The censer sits immediately above the status line, so the hero's last
	// rows must be exactly the art, in order. Checking only that each row
	// appears somewhere lets a row be dropped without anyone noticing - the
	// first version of this test did, and a shortened censer slipped past it.
	rows := statusline.CenserRows()
	for i := range rows {
		rows[i] = strings.TrimRight(statusline.Indent(rows[i]), " ")
	}

	shown := strings.Split(strings.TrimRight(hero, "\n"), "\n")

	// The height first. Without it a dropped row slides the comparison below
	// along by one and every remaining row still matches - which is exactly
	// what the first version of this test did.
	if want := statusline.SceneHeight() + 1; len(shown) != want {
		t.Fatalf("the README hero has %d rows, the runtime draws %d:\n%s", len(shown), want, hero)
	}

	// One line for the status text under the art.
	got := shown[len(shown)-1-len(rows) : len(shown)-1]
	for i, want := range rows {
		if got[i] != want {
			t.Fatalf("README row %d is %q, the runtime draws %q.\n"+
				"Re-capture it: prayops statusline claude", i, got[i], want)
		}
	}

	// And nothing from a censer that no longer ships.
	for _, gone := range []string{"祈", "香", "▕▏", "█▘"} {
		if strings.Contains(readme, gone) {
			t.Fatalf("the README still shows %q, which no release draws any more", gone)
		}
	}
}
