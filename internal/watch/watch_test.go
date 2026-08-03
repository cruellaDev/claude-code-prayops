package watch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cruellaDev/claude-code-prayops/contracts"
	"github.com/cruellaDev/claude-code-prayops/internal/spool"
)

var base = time.Unix(1700000000, 0).UTC()

func newModel(t *testing.T) (*Model, *spool.Spool) {
	t.Helper()
	stateDir := filepath.Join(t.TempDir(), "state")

	m := New(Options{
		StateDir: stateDir,
		Host:     contracts.HostClaude,
		Motion:   true,
		Seed:     42,
		Now:      func() time.Time { return base },
	})
	m.Resize(56, 18)
	return m, spool.New(stateDir)
}

func event(id string, kind contracts.EventType, offset time.Duration) contracts.RitualEvent {
	return contracts.RitualEvent{
		SchemaVersion: spool.SchemaVersion,
		ID:            id,
		Host:          contracts.HostClaude,
		SessionID:     "s1",
		ProjectKey:    "abc123",
		ProjectName:   "payment-api",
		Type:          kind,
		OccurredAt:    base.Add(offset),
	}
}

func TestAdvanceConsumesEventsAndUpdatesTheScene(t *testing.T) {
	m, events := newModel(t)

	for i, kind := range []contracts.EventType{
		contracts.EventPromptSubmitted,
		contracts.EventToolStarted,
	} {
		if err := events.Write(event(string(rune('a'+i)), kind, time.Duration(i)*time.Second)); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	m.Advance()

	if got := m.State().Phase; got != contracts.PhaseWorking {
		t.Fatalf("phase = %q, want WORKING", got)
	}
	if !strings.Contains(m.Frame(), "payment-api") {
		t.Fatalf("frame does not name the project:\n%s", m.Frame())
	}
}

// The watcher is told which host it displays; another host's events belong to
// a different watcher.
func TestEventsFromAnotherHostAreIgnored(t *testing.T) {
	m, events := newModel(t)

	codex := event("c1", contracts.EventToolStarted, 0)
	codex.Host = contracts.HostCodex
	if err := events.Write(codex); err != nil {
		t.Fatalf("write: %v", err)
	}

	m.Advance()

	if got := m.State().Phase; got == contracts.PhaseWorking {
		t.Fatal("a Codex event drove the Claude watcher")
	}
}

// NFR-002: an idle altar must not animate at the working frame rate.
func TestFrameRateFollowsActivity(t *testing.T) {
	m, events := newModel(t)

	if got := m.Interval(); got != time.Second/IdleFPS {
		t.Fatalf("idle interval = %v, want %v", got, time.Second/IdleFPS)
	}

	if err := events.Write(event("a", contracts.EventToolStarted, 0)); err != nil {
		t.Fatalf("write: %v", err)
	}
	m.Advance()

	if got := m.Interval(); got != time.Second/ActiveFPS {
		t.Fatalf("working interval = %v, want %v", got, time.Second/ActiveFPS)
	}
}

// FR-072 / D-028: a resize must not restart the scene.
func TestResizeKeepsTheSmokeField(t *testing.T) {
	m, _ := newModel(t)
	for i := 0; i < 40; i++ {
		m.Advance()
	}
	before := len(m.smoke.Render(m.layout))

	m.Resize(40, 13)

	if m.layout.Width != 40 || m.layout.Mode != contracts.LayoutMedium {
		t.Fatalf("layout did not change: %+v", m.layout)
	}
	if m.smoke.Len() == 0 && before > 0 {
		t.Fatal("the smoke field was thrown away on resize")
	}
}

func TestFrameFitsEverySize(t *testing.T) {
	m, events := newModel(t)
	if err := events.Write(event("a", contracts.EventToolStarted, 0)); err != nil {
		t.Fatalf("write: %v", err)
	}

	for _, size := range [][2]int{{80, 24}, {56, 18}, {40, 13}, {30, 9}, {24, 8}, {10, 4}} {
		m.Resize(size[0], size[1])
		for i := 0; i < 20; i++ {
			m.Advance()
		}

		lines := strings.Split(m.Frame(), "\n")
		if len(lines) != max(size[1], 0) {
			t.Fatalf("%v produced %d lines", size, len(lines))
		}
	}
}

// Doctor decides whether a watcher is running from this file.
func TestHeartbeatIsPublishedAndRemoved(t *testing.T) {
	m, _ := newModel(t)

	m.startHeartbeat()
	if m.heartbeat == "" {
		t.Fatal("no heartbeat file")
	}
	if _, err := os.Stat(m.heartbeat); err != nil {
		t.Fatalf("heartbeat not written: %v", err)
	}

	path := m.heartbeat
	m.stopHeartbeat()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("the heartbeat survived shutdown; doctor would report a dead watcher as running")
	}
}

// A watcher runs for hours, so the heartbeat must not be rewritten every frame.
func TestHeartbeatIsRateLimited(t *testing.T) {
	m, _ := newModel(t)
	now := base
	m.opts.Now = func() time.Time { return now }

	m.startHeartbeat()
	first := m.lastHeartbeat

	for i := 0; i < 100; i++ {
		m.beat()
	}
	if !m.lastHeartbeat.Equal(first) {
		t.Fatal("the heartbeat was rewritten within its interval")
	}

	now = now.Add(heartbeatEvery + time.Second)
	m.beat()
	if m.lastHeartbeat.Equal(first) {
		t.Fatal("the heartbeat stopped updating")
	}
}

// A missing state directory is normal before the first hook runs.
func TestWatcherWithoutStateDirectory(t *testing.T) {
	m := New(Options{StateDir: filepath.Join(t.TempDir(), "missing"), Seed: 1,
		Now: func() time.Time { return base }})
	m.Resize(56, 18)

	m.Advance()

	if !strings.Contains(m.Frame(), "PrayOps") {
		t.Fatalf("no frame rendered:\n%s", m.Frame())
	}
}

func TestSameSeedSameScene(t *testing.T) {
	build := func() string {
		stateDir := filepath.Join(t.TempDir(), "state")
		m := New(Options{StateDir: stateDir, Host: contracts.HostClaude, Motion: true,
			Seed: 7, Now: func() time.Time { return base }})
		m.Resize(56, 18)
		for i := 0; i < 30; i++ {
			m.Advance()
		}
		return m.Frame()
	}

	if build() != build() {
		t.Fatal("the same seed produced a different scene")
	}
}

// A queued prayer must puff once, not on every frame for the rest of the
// session.
func TestQueuedPrayerBurstsOnce(t *testing.T) {
	m, events := newModel(t)

	prayer := event("p1", contracts.EventPrayerRequested, 0)
	prayer.Attributes = map[string]string{"prayerKind": "TEXT", "effectSeed": "1"}
	if err := events.Write(prayer); err != nil {
		t.Fatalf("write: %v", err)
	}

	m.Advance()
	if len(m.State().PrayerQueue) != 0 {
		t.Fatalf("the prayer was not consumed: %+v", m.State().PrayerQueue)
	}

	after := m.smoke.Len()
	for i := 0; i < 5; i++ {
		m.Advance()
	}
	if m.smoke.Len() > after+5 {
		t.Fatalf("the burst kept firing: %d particles, was %d", m.smoke.Len(), after)
	}
}
