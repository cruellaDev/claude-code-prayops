package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cruellaDev/claude-code-prayops/contracts"
)

// Sessions persists one state file per session under <stateDir>/sessions.
//
// Hooks write it and the status line reads it. Without this the status line
// would have to consume the event spool, which would starve the watcher of the
// events it animates - the spool delivers each event exactly once.
type Sessions struct {
	dir string
}

// NewSessions returns the session store rooted at <stateDir>/sessions.
func NewSessions(stateDir string) *Sessions {
	return &Sessions{dir: filepath.Join(stateDir, "sessions")}
}

// safeName keeps a session id from escaping the sessions directory.
func safeName(sessionID string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, sessionID)
}

func (s *Sessions) path(sessionID string) string {
	return filepath.Join(s.dir, safeName(sessionID)+".json")
}

// Load returns the stored state for a session. A missing file is not an error:
// it just means nothing has happened in that session yet.
func (s *Sessions) Load(sessionID string) (contracts.SessionState, bool, error) {
	var state contracts.SessionState
	if sessionID == "" {
		return state, false, nil
	}

	raw, err := os.ReadFile(s.path(sessionID))
	if err != nil {
		if os.IsNotExist(err) {
			return state, false, nil
		}
		return state, false, fmt.Errorf("session: read state: %w", err)
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return state, false, fmt.Errorf("session: parse state: %w", err)
	}
	return state, true, nil
}

// Save writes the state atomically.
//
// Concurrent hooks can race here. The loser's update is lost, which self-heals
// on the next event, and Reduce ignores stale events so a late writer cannot
// move a session backwards.
func (s *Sessions) Save(state contracts.SessionState) error {
	if state.SessionID == "" {
		return fmt.Errorf("session: state has no session ID")
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("session: create sessions dir: %w", err)
	}

	payload, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("session: marshal state: %w", err)
	}

	tmp, err := os.CreateTemp(s.dir, "state-*.json")
	if err != nil {
		return fmt.Errorf("session: create temp state: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("session: write state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("session: close state: %w", err)
	}

	if err := os.Rename(tmpName, s.path(state.SessionID)); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("session: publish state: %w", err)
	}
	return nil
}

// List returns every stored session, newest first.
func (s *Sessions) List() ([]contracts.SessionState, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("session: read sessions: %w", err)
	}

	states := make([]contracts.SessionState, 0, len(entries))
	for _, entry := range entries {
		raw, err := os.ReadFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			continue
		}
		var state contracts.SessionState
		if err := json.Unmarshal(raw, &state); err != nil || state.SessionID == "" {
			continue
		}
		states = append(states, state)
	}

	sort.Slice(states, func(i, j int) bool { return states[i].UpdatedAt.After(states[j].UpdatedAt) })
	return states, nil
}

// Newest returns the most recently updated session, preferring one in the
// given project. A prayer sent from a project should join that project's
// session rather than whichever was touched last.
func (s *Sessions) Newest(projectKey string) (contracts.SessionState, bool) {
	states, err := s.List()
	if err != nil || len(states) == 0 {
		return contracts.SessionState{}, false
	}

	if projectKey != "" {
		for _, state := range states {
			if state.ProjectKey == projectKey && state.Phase != contracts.PhaseSessionEnded {
				return state, true
			}
		}
	}
	for _, state := range states {
		if state.Phase != contracts.PhaseSessionEnded {
			return state, true
		}
	}
	return states[0], true
}

// Record folds one event into the stored state for its session and saves the
// result. This is what a hook calls.
func (s *Sessions) Record(event contracts.RitualEvent) (contracts.SessionState, error) {
	previous, _, err := s.Load(event.SessionID)
	if err != nil {
		// A corrupt or unreadable state file must not stop the session from
		// moving on; start over from empty.
		previous = contracts.SessionState{}
	}

	updated := Reduce(previous, event)
	return updated, s.Save(updated)
}
