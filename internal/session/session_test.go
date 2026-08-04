package session

import (
	"fmt"
	"testing"
	"time"

	"github.com/cruellaDev/claude-code-prayops/contracts"
)

var base = time.Unix(1700000000, 0).UTC()

func event(id string, kind contracts.EventType, offset time.Duration) contracts.RitualEvent {
	return contracts.RitualEvent{
		SchemaVersion: 1,
		ID:            id,
		Host:          contracts.HostClaude,
		SessionID:     "s1",
		ProjectKey:    "abc123",
		ProjectName:   "payment-api",
		Type:          kind,
		OccurredAt:    base.Add(offset),
	}
}

func TestLifecyclePhases(t *testing.T) {
	cases := []struct {
		event contracts.EventType
		want  contracts.SessionPhase
	}{
		{contracts.EventSessionStarted, contracts.PhaseIdle},
		{contracts.EventPromptSubmitted, contracts.PhaseThinking},
		{contracts.EventToolStarted, contracts.PhaseWorking},
		{contracts.EventToolFinished, contracts.PhaseWorking},
		{contracts.EventToolFailed, contracts.PhaseWorking},
		{contracts.EventApprovalRequired, contracts.PhaseApprovalRequired},
		{contracts.EventTurnCompleted, contracts.PhaseTurnCompleted},
		{contracts.EventTurnFailed, contracts.PhaseTurnFailed},
		{contracts.EventSessionEnded, contracts.PhaseSessionEnded},
	}

	for _, tc := range cases {
		t.Run(string(tc.event), func(t *testing.T) {
			got := Reduce(contracts.SessionState{}, event("e1", tc.event, 0))
			if got.Phase != tc.want {
				t.Fatalf("phase = %q, want %q", got.Phase, tc.want)
			}
			if got.SessionID != "s1" || got.ProjectName != "payment-api" || got.LastEventID != "e1" {
				t.Fatalf("identity was not carried: %+v", got)
			}
		})
	}
}

func TestToolFailureMarkerClearsOnTheNextTurn(t *testing.T) {
	state := Reduce(contracts.SessionState{}, event("e1", contracts.EventToolFailed, 0))
	if !state.ToolErrored {
		t.Fatal("a failed tool did not set the error marker")
	}

	state = Reduce(state, event("e2", contracts.EventToolStarted, time.Second))
	if !state.ToolErrored {
		t.Fatal("the marker cleared during the same turn")
	}

	state = Reduce(state, event("e3", contracts.EventPromptSubmitted, 2*time.Second))
	if state.ToolErrored {
		t.Fatal("the marker survived into a new turn")
	}
}

// A replayed or clock-skewed event must not drag a session backwards.
func TestStaleEventsAreIgnored(t *testing.T) {
	state := Reduce(contracts.SessionState{}, event("e2", contracts.EventTurnCompleted, 10*time.Second))
	got := Reduce(state, event("e1", contracts.EventToolStarted, time.Second))

	if got.Phase != contracts.PhaseTurnCompleted {
		t.Fatalf("phase = %q, want the newer TURN_COMPLETED", got.Phase)
	}
	if got.LastEventID != "e2" {
		t.Fatalf("last event = %q, want e2", got.LastEventID)
	}
}

func prayer(id string, offset time.Duration, attrs map[string]string) contracts.RitualEvent {
	e := event(id, contracts.EventPrayerRequested, offset)
	e.Attributes = attrs
	return e
}

func TestPrayerQueueing(t *testing.T) {
	state := Reduce(contracts.SessionState{}, event("e1", contracts.EventToolStarted, 0))
	state = Reduce(state, prayer("p1", time.Second, map[string]string{
		"prayerKind": string(contracts.PrayerSourceText),
		"cacheId":    "cache-1",
		"effectSeed": "42",
	}))

	if len(state.PrayerQueue) != 1 {
		t.Fatalf("queue holds %d, want 1", len(state.PrayerQueue))
	}
	queued := state.PrayerQueue[0]
	if queued.ID != "p1" || queued.Source.Kind != contracts.PrayerSourceText ||
		queued.Source.CacheID != "cache-1" || queued.Seed != 42 {
		t.Fatalf("unexpected request: %+v", queued)
	}

	// A prayer is an overlay, not a lifecycle change.
	if state.Phase != contracts.PhaseWorking {
		t.Fatalf("phase = %q, want WORKING to be preserved", state.Phase)
	}
}

func TestDuplicatePrayersAreIgnored(t *testing.T) {
	state := Reduce(contracts.SessionState{}, prayer("p1", 0, nil))
	state = Reduce(state, prayer("p1", time.Second, nil))

	if len(state.PrayerQueue) != 1 {
		t.Fatalf("queue holds %d, want 1", len(state.PrayerQueue))
	}
}

func TestPrayerQueueDropsOldestWhenFull(t *testing.T) {
	var state contracts.SessionState
	total := contracts.MaxPendingPrayers + 3

	for i := 0; i < total; i++ {
		state = Reduce(state, prayer(fmt.Sprintf("p%d", i), time.Duration(i)*time.Second, nil))
	}

	if len(state.PrayerQueue) != contracts.MaxPendingPrayers {
		t.Fatalf("queue holds %d, want %d", len(state.PrayerQueue), contracts.MaxPendingPrayers)
	}
	if state.PrayerQueue[0].ID != "p3" {
		t.Fatalf("oldest queued = %q, want p3", state.PrayerQueue[0].ID)
	}
	if state.PrayerQueue[len(state.PrayerQueue)-1].ID != fmt.Sprintf("p%d", total-1) {
		t.Fatalf("newest queued = %q", state.PrayerQueue[len(state.PrayerQueue)-1].ID)
	}
}

func TestUnknownPrayerKindFallsBackToPreset(t *testing.T) {
	state := Reduce(contracts.SessionState{}, prayer("p1", 0, map[string]string{"prayerKind": "SOMETHING_ELSE"}))

	if state.PrayerQueue[0].Source.Kind != contracts.PrayerSourcePreset {
		t.Fatalf("kind = %q, want PRESET", state.PrayerQueue[0].Source.Kind)
	}
}

func sessionEvent(session, id string, kind contracts.EventType, offset time.Duration) contracts.RitualEvent {
	e := event(id, kind, offset)
	e.SessionID = session
	return e
}

func TestStoreTracksSessionsIndependently(t *testing.T) {
	store := NewStore()
	store.ApplyAll([]contracts.RitualEvent{
		sessionEvent("a", "e1", contracts.EventPromptSubmitted, 0),
		sessionEvent("b", "e2", contracts.EventToolStarted, time.Second),
		sessionEvent("a", "e3", contracts.EventApprovalRequired, 2*time.Second),
	})

	sessions := store.Sessions()
	if len(sessions) != 2 {
		t.Fatalf("store tracks %d sessions, want 2", len(sessions))
	}
	if sessions[0].SessionID != "a" || sessions[0].Phase != contracts.PhaseApprovalRequired {
		t.Fatalf("newest session = %+v", sessions[0])
	}
}

// FR-072: the watcher renders the latest active session.
func TestActivePrefersTheNewestRunningSession(t *testing.T) {
	store := NewStore()
	store.ApplyAll([]contracts.RitualEvent{
		sessionEvent("a", "e1", contracts.EventToolStarted, 0),
		sessionEvent("b", "e2", contracts.EventPromptSubmitted, time.Second),
	})

	got, ok := store.Active()
	if !ok || got.SessionID != "b" {
		t.Fatalf("active = %+v", got)
	}

	// Once the newest session ends, the still-running one takes over.
	store.Apply(sessionEvent("b", "e3", contracts.EventSessionEnded, 2*time.Second))
	got, ok = store.Active()
	if !ok || got.SessionID != "a" {
		t.Fatalf("active after end = %+v", got)
	}

	// With everything ended, the most recent is still shown rather than
	// nothing at all.
	store.Apply(sessionEvent("a", "e4", contracts.EventSessionEnded, 3*time.Second))
	got, ok = store.Active()
	if !ok || got.SessionID != "a" {
		t.Fatalf("active after all ended = %+v", got)
	}
}

func TestActiveOnAnEmptyStore(t *testing.T) {
	if _, ok := NewStore().Active(); ok {
		t.Fatal("an empty store reported an active session")
	}
}

// A long-lived watcher must not accumulate one entry per session forever.
func TestStoreEvictsOldSessions(t *testing.T) {
	store := NewStore()
	for i := 0; i < maxSessions+5; i++ {
		store.Apply(sessionEvent(fmt.Sprintf("s%02d", i), fmt.Sprintf("e%02d", i),
			contracts.EventToolStarted, time.Duration(i)*time.Second))
	}

	if len(store.states) != maxSessions {
		t.Fatalf("store holds %d sessions, want %d", len(store.states), maxSessions)
	}
	if _, ok := store.states["s00"]; ok {
		t.Fatal("the oldest session was not evicted")
	}
	if _, ok := store.states[fmt.Sprintf("s%02d", maxSessions+4)]; !ok {
		t.Fatal("the newest session was evicted")
	}
}

// Not every hook payload carries a cwd. An event without one must not erase
// the project a session already belongs to - a prayer looks its session up by
// project key, and a blanked key sends it to the wrong session.
func TestAnEventWithoutACwdKeepsTheProject(t *testing.T) {
	state := Reduce(contracts.SessionState{}, event("e1", contracts.EventPromptSubmitted, 0))
	if state.ProjectKey == "" || state.ProjectName == "" {
		t.Fatalf("the first event carried no project: %+v", state)
	}

	anonymous := event("e2", contracts.EventToolStarted, time.Second)
	anonymous.ProjectKey = ""
	anonymous.ProjectName = ""

	got := Reduce(state, anonymous)

	if got.ProjectKey != state.ProjectKey {
		t.Fatalf("project key was erased: %q -> %q", state.ProjectKey, got.ProjectKey)
	}
	if got.ProjectName != state.ProjectName {
		t.Fatalf("project name was erased: %q -> %q", state.ProjectName, got.ProjectName)
	}
	if got.Phase != contracts.PhaseWorking {
		t.Fatalf("phase = %q", got.Phase)
	}
}
