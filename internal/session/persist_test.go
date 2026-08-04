package session

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cruellaDev/claude-code-prayops/contracts"
)

func newSessions(t *testing.T) *Sessions {
	t.Helper()
	return NewSessions(filepath.Join(t.TempDir(), "state"))
}

func TestRecordAccumulatesAcrossCalls(t *testing.T) {
	sessions := newSessions(t)

	if _, err := sessions.Record(event("e1", contracts.EventPromptSubmitted, 0)); err != nil {
		t.Fatalf("record: %v", err)
	}
	got, err := sessions.Record(event("e2", contracts.EventToolStarted, time.Second))
	if err != nil {
		t.Fatalf("record: %v", err)
	}

	if got.Phase != contracts.PhaseWorking || got.LastEventID != "e2" {
		t.Fatalf("unexpected state: %+v", got)
	}

	loaded, found, err := sessions.Load("s1")
	if err != nil || !found {
		t.Fatalf("load: %v found=%v", err, found)
	}
	if loaded.Phase != contracts.PhaseWorking || loaded.ProjectName != "payment-api" {
		t.Fatalf("state did not survive the round trip: %+v", loaded)
	}
}

func TestLoadOfAnUnknownSessionIsNotAnError(t *testing.T) {
	sessions := newSessions(t)

	if _, found, err := sessions.Load("nobody"); err != nil || found {
		t.Fatalf("err = %v, found = %v", err, found)
	}
	if _, found, err := sessions.Load(""); err != nil || found {
		t.Fatalf("err = %v, found = %v", err, found)
	}
}

// A session id arrives from outside the process, so it must not be able to
// choose where the file lands.
func TestSessionIDsCannotEscapeTheDirectory(t *testing.T) {
	sessions := newSessions(t)

	state := contracts.SessionState{SessionID: "../../etc/passwd", Phase: contracts.PhaseIdle}
	if err := sessions.Save(state); err != nil {
		t.Fatalf("save: %v", err)
	}

	entries, err := os.ReadDir(sessions.dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("wrote %d files, want 1", len(entries))
	}
	if strings.Contains(entries[0].Name(), "/") || strings.Contains(entries[0].Name(), "..") {
		t.Fatalf("unsafe filename %q", entries[0].Name())
	}

	// It must still be readable under the same id.
	if _, found, err := sessions.Load("../../etc/passwd"); err != nil || !found {
		t.Fatalf("err = %v, found = %v", err, found)
	}
}

// A corrupt state file must not wedge the session forever.
func TestRecordRecoversFromACorruptStateFile(t *testing.T) {
	sessions := newSessions(t)
	if err := os.MkdirAll(sessions.dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(sessions.path("s1"), []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}

	got, err := sessions.Record(event("e1", contracts.EventToolStarted, 0))
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if got.Phase != contracts.PhaseWorking {
		t.Fatalf("phase = %q", got.Phase)
	}
}

func TestSaveLeavesNoTempFiles(t *testing.T) {
	sessions := newSessions(t)
	if _, err := sessions.Record(event("e1", contracts.EventToolStarted, 0)); err != nil {
		t.Fatalf("record: %v", err)
	}

	entries, err := os.ReadDir(sessions.dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("sessions dir holds %d files, want 1", len(entries))
	}
}

// Every hook is its own process, so concurrent writes to one session are
// normal. A lost update is acceptable; a corrupt file is not.
func TestConcurrentRecordsLeaveReadableState(t *testing.T) {
	sessions := newSessions(t)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = sessions.Record(event("e", contracts.EventToolStarted, time.Duration(i)*time.Millisecond))
		}(i)
	}
	wg.Wait()

	got, found, err := sessions.Load("s1")
	if err != nil || !found {
		t.Fatalf("load: %v found=%v", err, found)
	}
	if got.Phase != contracts.PhaseWorking {
		t.Fatalf("phase = %q, want WORKING", got.Phase)
	}
}
