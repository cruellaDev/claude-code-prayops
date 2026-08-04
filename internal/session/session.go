// Package session folds ritual events into the state the watcher and status
// line render.
package session

import (
	"sort"
	"strconv"

	"github.com/cruellaDev/claude-code-prayops/contracts"
)

// maxSessions bounds how many sessions are tracked at once. A long-lived
// watcher would otherwise accumulate one entry per session forever.
const maxSessions = 16

// Reduce applies one event to a session state and returns the result. It is
// pure: no clock, no I/O, no randomness.
func Reduce(state contracts.SessionState, event contracts.RitualEvent) contracts.SessionState {
	// The spool delivers in order, but a replayed or clock-skewed event must
	// not drag a session backwards.
	if !state.UpdatedAt.IsZero() && event.OccurredAt.Before(state.UpdatedAt) {
		return state
	}

	state.SchemaVersion = event.SchemaVersion
	state.Host = event.Host
	state.SessionID = event.SessionID
	state.ProjectKey = event.ProjectKey
	if event.ProjectName != "" {
		state.ProjectName = event.ProjectName
	}
	state.LastEventID = event.ID
	state.UpdatedAt = event.OccurredAt

	switch event.Type {
	case contracts.EventSessionStarted:
		state.Phase = contracts.PhaseIdle
		state.ToolErrored = false
	case contracts.EventPromptSubmitted:
		state.Phase = contracts.PhaseThinking
		state.TurnStartedAt = event.OccurredAt
		// A new turn starts clean, so a failure from the previous one stops
		// colouring the display.
		state.ToolErrored = false
	case contracts.EventToolStarted, contracts.EventToolFinished:
		state.Phase = contracts.PhaseWorking
	case contracts.EventToolFailed:
		state.Phase = contracts.PhaseWorking
		state.ToolErrored = true
	case contracts.EventApprovalRequired:
		state.Phase = contracts.PhaseApprovalRequired
	case contracts.EventTurnCompleted:
		state.Phase = contracts.PhaseTurnCompleted
	case contracts.EventTurnFailed:
		state.Phase = contracts.PhaseTurnFailed
	case contracts.EventSessionEnded:
		state.Phase = contracts.PhaseSessionEnded
	case contracts.EventPrayerRequested:
		// A prayer is an overlay on whatever the session is doing, so the
		// phase is left alone.
		state.PrayerQueue = enqueuePrayer(state.PrayerQueue, prayerFrom(event))
	}

	return state
}

// prayerFrom builds a request from the allowlisted attributes. The prayer's
// text or image never travels in the event; the runtime resolves cacheId.
func prayerFrom(event contracts.RitualEvent) contracts.PrayerRequest {
	seed, _ := strconv.ParseUint(event.Attributes["effectSeed"], 10, 64)

	kind := contracts.PrayerSourceKind(event.Attributes["prayerKind"])
	switch kind {
	case contracts.PrayerSourceImage, contracts.PrayerSourceText, contracts.PrayerSourcePreset:
	default:
		kind = contracts.PrayerSourcePreset
	}

	return contracts.PrayerRequest{
		ID:        event.ID,
		SessionID: event.SessionID,
		Source: contracts.PrayerSource{
			Kind:    kind,
			CacheID: event.Attributes["cacheId"],
		},
		Seed: seed,
	}
}

// enqueuePrayer appends a request, ignoring duplicates and dropping the oldest
// pending entry when the queue is full.
func enqueuePrayer(queue []contracts.PrayerRequest, request contracts.PrayerRequest) []contracts.PrayerRequest {
	for _, queued := range queue {
		if queued.ID == request.ID {
			return queue
		}
	}

	queue = append(queue, request)
	if len(queue) > contracts.MaxPendingPrayers {
		queue = queue[len(queue)-contracts.MaxPendingPrayers:]
	}
	return queue
}

// Store tracks one state per session and knows which one to render.
type Store struct {
	states map[string]contracts.SessionState
}

// NewStore returns an empty store.
func NewStore() *Store {
	return &Store{states: make(map[string]contracts.SessionState)}
}

// Apply folds one event into the store and returns the updated session.
func (s *Store) Apply(event contracts.RitualEvent) contracts.SessionState {
	updated := Reduce(s.states[event.SessionID], event)
	s.states[event.SessionID] = updated
	s.evict()
	return updated
}

// Put replaces the stored state for a session.
//
// The watcher uses it to consume a prayer from the queue: the reducer only
// appends, since events never say a prayer has been shown.
func (s *Store) Put(state contracts.SessionState) {
	if state.SessionID == "" {
		return
	}
	s.states[state.SessionID] = state
}

// ApplyAll folds a batch in order.
func (s *Store) ApplyAll(events []contracts.RitualEvent) {
	for _, event := range events {
		s.Apply(event)
	}
}

// Active returns the session to display: the most recently updated one that is
// still running, or the most recent overall when every session has ended.
func (s *Store) Active() (contracts.SessionState, bool) {
	var best contracts.SessionState
	var found bool

	for _, state := range s.states {
		if state.Phase == contracts.PhaseSessionEnded {
			continue
		}
		if !found || state.UpdatedAt.After(best.UpdatedAt) {
			best, found = state, true
		}
	}
	if found {
		return best, true
	}

	for _, state := range s.states {
		if !found || state.UpdatedAt.After(best.UpdatedAt) {
			best, found = state, true
		}
	}
	return best, found
}

// Sessions returns every tracked session, newest first.
func (s *Store) Sessions() []contracts.SessionState {
	all := make([]contracts.SessionState, 0, len(s.states))
	for _, state := range s.states {
		all = append(all, state)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].UpdatedAt.After(all[j].UpdatedAt) })
	return all
}

func (s *Store) evict() {
	if len(s.states) <= maxSessions {
		return
	}
	stale := s.Sessions()[maxSessions:]
	for _, state := range stale {
		delete(s.states, state.SessionID)
	}
}
