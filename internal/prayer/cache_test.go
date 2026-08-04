package prayer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cruellaDev/claude-code-prayops/contracts"
)

func newCache(t *testing.T) *Cache {
	t.Helper()
	return New(filepath.Join(t.TempDir(), "state"))
}

func sample() contracts.TerminalRaster {
	return contracts.TerminalRaster{
		Width: 3, Height: 1,
		Cells: []contracts.RasterCell{
			{X: 0, Y: 0, Glyph: '▀', Top: contracts.RGB{R: 200, G: 10, B: 10}, Bottom: contracts.RGB{B: 90}, Alpha: 1},
			{X: 1, Y: 0, Glyph: '가', Alpha: 1},
		},
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	c := newCache(t)
	id := NewID()

	if err := c.Save(id, sample()); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := c.Load(id)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Width != 3 || got.Height != 1 || len(got.Cells) != 2 {
		t.Fatalf("round trip lost shape: %+v", got)
	}
	if got.Cells[0].Top.R != 200 || got.Cells[0].Bottom.B != 90 {
		t.Fatalf("round trip lost colour: %+v", got.Cells[0])
	}
	if got.Cells[1].Glyph != '가' {
		t.Fatalf("round trip lost the glyph: %q", got.Cells[1].Glyph)
	}
}

func TestIDsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		id := NewID()
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
}

// A cache id travels through an event file, so it must not be able to choose
// where the prayer lands.
func TestIDsCannotEscapeTheCache(t *testing.T) {
	c := newCache(t)

	if err := c.Save("../../etc/passwd", sample()); err != nil {
		t.Fatalf("save: %v", err)
	}

	entries, err := os.ReadDir(c.dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("wrote %d files", len(entries))
	}
	if strings.Contains(entries[0].Name(), "..") || strings.Contains(entries[0].Name(), "/") {
		t.Fatalf("unsafe name %q", entries[0].Name())
	}
	if _, err := c.Load("../../etc/passwd"); err != nil {
		t.Fatalf("load: %v", err)
	}
}

func TestLoadOfAnUnknownPrayer(t *testing.T) {
	c := newCache(t)

	if _, err := c.Load("nothing"); err == nil {
		t.Fatal("an unknown prayer loaded")
	}
	if _, err := c.Load(""); err == nil {
		t.Fatal("an empty id loaded")
	}
}

func TestDiscardRemovesTheEntry(t *testing.T) {
	c := newCache(t)
	id := NewID()
	if err := c.Save(id, sample()); err != nil {
		t.Fatalf("save: %v", err)
	}

	c.Discard(id)

	if _, err := c.Load(id); err == nil {
		t.Fatal("the prayer survived being discarded")
	}
	c.Discard(id) // discarding twice must not panic
}

// A prayer sent with no watcher running is never discarded, so something has
// to collect it.
func TestCleanupCollectsUnseenPrayers(t *testing.T) {
	c := newCache(t)
	now := time.Unix(1700000000, 0).UTC()
	c.SetClock(func() time.Time { return now })

	old, fresh := NewID(), NewID()
	for _, id := range []string{old, fresh} {
		if err := c.Save(id, sample()); err != nil {
			t.Fatalf("save: %v", err)
		}
	}
	stale := now.Add(-48 * time.Hour)
	if err := os.Chtimes(c.path(old), stale, stale); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	if err := os.Chtimes(c.path(fresh), now, now); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	if err := c.Cleanup(24 * time.Hour); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	if _, err := c.Load(old); err == nil {
		t.Fatal("an old prayer survived cleanup")
	}
	if _, err := c.Load(fresh); err != nil {
		t.Fatalf("cleanup removed a fresh prayer: %v", err)
	}
}

func TestCleanupOnAMissingCache(t *testing.T) {
	if err := New(filepath.Join(t.TempDir(), "nothing")).Cleanup(time.Hour); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
}

func TestSaveLeavesNoTempFiles(t *testing.T) {
	c := newCache(t)
	if err := c.Save(NewID(), sample()); err != nil {
		t.Fatalf("save: %v", err)
	}

	entries, err := os.ReadDir(c.dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("cache holds %d files, want 1", len(entries))
	}
}

func TestOversizedRastersAreRefused(t *testing.T) {
	c := newCache(t)

	huge := contracts.TerminalRaster{Width: 1000, Height: 1000}
	for i := 0; i < 20000; i++ {
		huge.Cells = append(huge.Cells, contracts.RasterCell{X: i, Y: i, Glyph: '▀', Alpha: 1})
	}

	if err := c.Save(NewID(), huge); err == nil {
		t.Fatal("an oversized raster was cached")
	}
}
