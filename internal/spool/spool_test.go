package spool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cruellaDev/claude-code-prayops/contracts"
)

func event(id string, at time.Time) contracts.RitualEvent {
	return contracts.RitualEvent{
		SchemaVersion: SchemaVersion,
		ID:            id,
		Host:          contracts.HostClaude,
		SessionID:     "session-1",
		ProjectKey:    "abc123",
		ProjectName:   "payment-api",
		Type:          contracts.EventToolStarted,
		OccurredAt:    at,
	}
}

func newSpool(t *testing.T) *Spool {
	t.Helper()
	return New(filepath.Join(t.TempDir(), "state"))
}

func TestWriteThenReadRoundTrip(t *testing.T) {
	s := newSpool(t)
	want := event("e1", time.Unix(1700000000, 0).UTC())
	want.Attributes = map[string]string{"toolCategory": "edit"}

	if err := s.Write(want); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := s.Read(0)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("read %d events, want 1", len(got))
	}
	if got[0].ID != want.ID || got[0].Type != want.Type || !got[0].OccurredAt.Equal(want.OccurredAt) {
		t.Fatalf("round trip lost data: %+v", got[0])
	}
	if got[0].Attributes["toolCategory"] != "edit" {
		t.Fatalf("round trip lost attributes: %+v", got[0].Attributes)
	}
}

func TestReadIsFIFOAndConsumes(t *testing.T) {
	s := newSpool(t)
	base := time.Unix(1700000000, 0).UTC()

	// Written out of order; the spool must still deliver them in time order.
	for _, offset := range []int{2, 0, 1} {
		if err := s.Write(event(fmt.Sprintf("e%d", offset), base.Add(time.Duration(offset)*time.Second))); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	got, err := s.Read(0)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("read %d events, want 3", len(got))
	}
	for i, want := range []string{"e0", "e1", "e2"} {
		if got[i].ID != want {
			t.Fatalf("event %d = %q, want %q", i, got[i].ID, want)
		}
	}

	again, err := s.Read(0)
	if err != nil {
		t.Fatalf("second read: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("second read returned %d events, want 0", len(again))
	}
}

func TestReadRespectsBatchLimit(t *testing.T) {
	s := newSpool(t)
	base := time.Unix(1700000000, 0).UTC()
	for i := 0; i < 10; i++ {
		if err := s.Write(event(fmt.Sprintf("e%02d", i), base.Add(time.Duration(i)*time.Second))); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	got, err := s.Read(4)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("read %d events, want 4", len(got))
	}
	if got[0].ID != "e00" || got[3].ID != "e03" {
		t.Fatalf("wrong batch: %s..%s", got[0].ID, got[3].ID)
	}

	rest, err := s.Read(0)
	if err != nil {
		t.Fatalf("read rest: %v", err)
	}
	if len(rest) != 6 {
		t.Fatalf("read %d remaining events, want 6", len(rest))
	}
}

func TestWriteLeavesNoTempFiles(t *testing.T) {
	s := newSpool(t)
	if err := s.Write(event("e1", time.Now())); err != nil {
		t.Fatalf("write: %v", err)
	}

	entries, err := os.ReadDir(s.dir("tmp"))
	if err != nil {
		t.Fatalf("read tmp: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("tmp still holds %d files", len(entries))
	}
}

// A corrupt file must be quarantined, not retried forever and not silently
// deleted.
func TestCorruptEventsAreQuarantined(t *testing.T) {
	s := newSpool(t)
	good := event("good", time.Unix(1700000002, 0).UTC())
	if err := s.Write(good); err != nil {
		t.Fatalf("write: %v", err)
	}

	corrupt := filepath.Join(s.dir("inbox"), "00000000001700000001-broken.json")
	if err := os.WriteFile(corrupt, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}

	got, err := s.Read(0)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 1 || got[0].ID != "good" {
		t.Fatalf("corrupt file blocked the batch: %+v", got)
	}

	if _, err := os.Stat(filepath.Join(s.dir("rejected"), "00000000001700000001-broken.json")); err != nil {
		t.Fatalf("corrupt file was not quarantined: %v", err)
	}
	if _, err := os.Stat(corrupt); !os.IsNotExist(err) {
		t.Fatal("corrupt file is still in the inbox and will be retried forever")
	}
}

func TestDuplicateEventIDsAreDeliveredOnce(t *testing.T) {
	s := newSpool(t)
	base := time.Unix(1700000000, 0).UTC()

	if err := s.Write(event("dup", base)); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := s.Read(0); err != nil {
		t.Fatalf("read: %v", err)
	}

	// The same ID arriving later - a retried hook - must not replay.
	if err := s.Write(event("dup", base.Add(time.Second))); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	got, err := s.Read(0)
	if err != nil {
		t.Fatalf("second read: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("duplicate event was delivered again: %+v", got)
	}
}

// The privacy allowlist is enforced before anything reaches the disk.
func TestWriteRejectsNonAllowlistedAttributes(t *testing.T) {
	s := newSpool(t)
	bad := event("e1", time.Now())
	bad.Attributes = map[string]string{"prompt": "refactor the auth module"}

	err := s.Write(bad)
	if err == nil {
		t.Fatal("write accepted a non-allowlisted attribute")
	}
	if !strings.Contains(err.Error(), "privacy allowlist") {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, _ := os.ReadDir(s.dir("inbox"))
	if len(entries) != 0 {
		t.Fatalf("a rejected event still reached the inbox: %d files", len(entries))
	}
}

// An event file written by a tampered or older build must not smuggle a
// payload field past the reader either.
func TestReadRejectsNonAllowlistedAttributes(t *testing.T) {
	s := newSpool(t)
	if err := s.ensureDirs(); err != nil {
		t.Fatalf("ensure dirs: %v", err)
	}

	smuggled := event("e1", time.Unix(1700000000, 0).UTC())
	raw, err := json.Marshal(struct {
		contracts.RitualEvent
		Attributes map[string]string `json:"attributes"`
	}{smuggled, map[string]string{"tool_input": "rm -rf /"}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(s.dir("inbox"), "00000000001700000000-e1.json"), raw, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := s.Read(0)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("reader returned an event carrying a payload field: %+v", got)
	}
	entries, _ := os.ReadDir(s.dir("rejected"))
	if len(entries) != 1 {
		t.Fatalf("the smuggled event was not quarantined: %d rejected files", len(entries))
	}
}

func TestInvalidEventsAreRefused(t *testing.T) {
	base := event("e1", time.Unix(1700000000, 0).UTC())

	cases := map[string]func(*contracts.RitualEvent){
		"no ID":        func(e *contracts.RitualEvent) { e.ID = "" },
		"no session":   func(e *contracts.RitualEvent) { e.SessionID = "" },
		"no host":      func(e *contracts.RitualEvent) { e.Host = "" },
		"no type":      func(e *contracts.RitualEvent) { e.Type = "" },
		"no timestamp": func(e *contracts.RitualEvent) { e.OccurredAt = time.Time{} },
		"wrong schema": func(e *contracts.RitualEvent) { e.SchemaVersion = 99 },
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			s := newSpool(t)
			bad := base
			mutate(&bad)
			if err := s.Write(bad); err == nil {
				t.Fatal("write accepted an invalid event")
			}
		})
	}
}

// Hooks keep firing when no watcher is attached, so the inbox has to stay
// bounded.
func TestInboxIsBounded(t *testing.T) {
	s := newSpool(t)
	base := time.Unix(1700000000, 0).UTC()

	total := maxInbox + 20
	for i := 0; i < total; i++ {
		if err := s.Write(event(fmt.Sprintf("e%05d", i), base.Add(time.Duration(i)*time.Second))); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}

	names, err := inboxNames(s.dir("inbox"))
	if err != nil {
		t.Fatalf("list inbox: %v", err)
	}
	if len(names) != maxInbox {
		t.Fatalf("inbox holds %d events, want %d", len(names), maxInbox)
	}

	// The oldest were dropped, so the newest event must still be there.
	got, err := s.Read(maxInbox)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got[len(got)-1].ID != fmt.Sprintf("e%05d", total-1) {
		t.Fatalf("newest event = %q, want e%05d", got[len(got)-1].ID, total-1)
	}
	if got[0].ID == "e00000" {
		t.Fatal("the oldest event survived; the inbox was not trimmed")
	}
}

func TestCleanupRemovesStaleFiles(t *testing.T) {
	s := newSpool(t)
	if err := s.ensureDirs(); err != nil {
		t.Fatalf("ensure dirs: %v", err)
	}

	now := time.Unix(1700000000, 0).UTC()
	s.SetClock(func() time.Time { return now })

	stale := filepath.Join(s.dir("tmp"), "event-stale.json")
	fresh := filepath.Join(s.dir("rejected"), "event-fresh.json")
	for path, age := range map[string]time.Duration{stale: 48 * time.Hour, fresh: time.Minute} {
		if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		when := now.Add(-age)
		if err := os.Chtimes(path, when, when); err != nil {
			t.Fatalf("chtimes %s: %v", path, err)
		}
	}

	if err := s.Cleanup(24 * time.Hour); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("a stale temp file survived cleanup")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("cleanup removed a fresh file: %v", err)
	}
}

// Every Claude Code hook is its own process, so concurrent writes are normal.
func TestConcurrentWritesDoNotCollide(t *testing.T) {
	s := newSpool(t)
	base := time.Unix(1700000000, 0).UTC()

	const writers = 24
	var wg sync.WaitGroup
	errs := make(chan error, writers)

	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := s.Write(event(fmt.Sprintf("e%03d", i), base.Add(time.Duration(i)*time.Millisecond))); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent write: %v", err)
	}

	got, err := s.Read(writers)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != writers {
		t.Fatalf("read %d events, want %d", len(got), writers)
	}
}

// NFR-001 budgets a hook at 100ms end to end, and the write is the only part
// that touches the disk.
func BenchmarkWrite(b *testing.B) {
	s := New(filepath.Join(b.TempDir(), "state"))
	base := time.Unix(1700000000, 0).UTC()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := s.Write(event(fmt.Sprintf("e%09d", i), base.Add(time.Duration(i)*time.Microsecond))); err != nil {
			b.Fatalf("write: %v", err)
		}
	}
}
