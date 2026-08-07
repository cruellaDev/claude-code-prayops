package userconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/cruellaDev/claude-code-prayops/internal/hook"
)

// Scope decides which projects the status line draws in.
//
// It cannot be done with Claude Code's settings files. `statusLine` is read
// from user settings only - a copy in a project's .claude/settings.json or
// settings.local.json is ignored, silently, so a user who scopes it that way
// gets no status line anywhere and no error explaining why. The setting
// therefore stays at user scope, and the decision moves in here, where the
// status line already knows which project it was invoked for.
//
// An empty list means every project, which is what a fresh install wants.
// Adding one project turns the list into an allowlist.
type Scope struct {
	// Projects holds hashed project keys, never paths: the same hash the
	// events use, so nothing here says where anyone works.
	Projects []string `json:"projects"`

	// Sessions narrows further, to particular windows. Two windows open on
	// the same project cannot be told apart by Projects, so this is the only
	// way to say "this one, not the other".
	//
	// A session id dies with its window. If a dead one were left here the
	// censer would be hidden everywhere with nothing to explain it - the
	// exact failure this scoping replaced - so the SessionEnd hook drops it.
	Sessions []string `json:"sessions,omitempty"`

	// Paused hides the censer everywhere without touching the user's
	// settings.json. Uninstalling would work too, but it throws away the
	// setting and whatever it replaced - a switch should be a switch.
	Paused bool `json:"paused,omitempty"`
}

// scopePath is where the list lives inside the plugin's data directory.
func scopePath(dataDir string) string { return filepath.Join(dataDir, "statusline-scope.json") }

// LoadScope reads the list. A missing or unreadable file is an empty scope,
// because failing to read it must not blank the status line.
func LoadScope(dataDir string) Scope {
	var scope Scope

	raw, err := os.ReadFile(scopePath(dataDir))
	if err != nil {
		return scope
	}
	if err := json.Unmarshal(raw, &scope); err != nil {
		return Scope{}
	}
	return scope
}

// Allows reports whether the status line should draw here and now.
//
// Every rule has to agree. An empty list means "no opinion", so a fresh
// install draws everywhere and each narrowing only ever removes.
func (s Scope) Allows(projectDir, sessionID string) bool {
	if s.Paused {
		return false
	}
	return listAllows(s.Projects, hook.ProjectKey(projectDir)) &&
		listAllows(s.Sessions, sessionID)
}

func listAllows(list []string, key string) bool {
	if len(list) == 0 {
		return true
	}
	for _, allowed := range list {
		if allowed == key {
			return true
		}
	}
	return false
}

// Add restricts the status line to projectDir, plus anything already listed.
func (s Scope) Add(projectDir string) Scope {
	s.Projects = add(s.Projects, hook.ProjectKey(projectDir))
	return s
}

// AddSession restricts the status line to one window.
func (s Scope) AddSession(sessionID string) Scope {
	if sessionID != "" {
		s.Sessions = add(s.Sessions, sessionID)
	}
	return s
}

// RemoveSession drops a window. The SessionEnd hook calls this so a closed
// window cannot go on narrowing the scope from beyond the grave.
func (s Scope) RemoveSession(sessionID string) Scope {
	s.Sessions = remove(s.Sessions, sessionID)
	return s
}

func add(list []string, key string) []string {
	for _, allowed := range list {
		if allowed == key {
			return list
		}
	}
	list = append(list, key)
	sort.Strings(list)
	return list
}

func remove(list []string, key string) []string {
	kept := list[:0]
	for _, allowed := range list {
		if allowed != key {
			kept = append(kept, allowed)
		}
	}
	return kept
}

// Remove drops projectDir. Removing the last one restores every project
// rather than leaving a list that matches nothing - a scope that hides the
// status line everywhere is indistinguishable from the bug this replaced.
func (s Scope) Remove(projectDir string) Scope {
	s.Projects = remove(s.Projects, hook.ProjectKey(projectDir))
	return s
}

// SaveScope writes the list, creating the directory if it is not there.
func SaveScope(dataDir string, scope Scope) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}

	payload, err := json.MarshalIndent(scope, "", "  ")
	if err != nil {
		return err
	}

	// Same write-then-rename as everything else here: a status line that
	// reads a half-written list would draw for the wrong projects.
	tmp := scopePath(dataDir) + ".new"
	if err := os.WriteFile(tmp, append(payload, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, scopePath(dataDir))
}
