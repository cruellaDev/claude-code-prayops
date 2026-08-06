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

// Allows reports whether the status line should draw for a project.
func (s Scope) Allows(projectDir string) bool {
	if len(s.Projects) == 0 {
		return true
	}
	key := hook.ProjectKey(projectDir)
	for _, allowed := range s.Projects {
		if allowed == key {
			return true
		}
	}
	return false
}

// Add restricts the status line to projectDir, plus anything already listed.
func (s Scope) Add(projectDir string) Scope {
	key := hook.ProjectKey(projectDir)
	for _, allowed := range s.Projects {
		if allowed == key {
			return s
		}
	}
	s.Projects = append(s.Projects, key)
	sort.Strings(s.Projects)
	return s
}

// Remove drops projectDir. Removing the last one restores every project
// rather than leaving a list that matches nothing - a scope that hides the
// status line everywhere is indistinguishable from the bug this replaced.
func (s Scope) Remove(projectDir string) Scope {
	key := hook.ProjectKey(projectDir)

	kept := s.Projects[:0]
	for _, allowed := range s.Projects {
		if allowed != key {
			kept = append(kept, allowed)
		}
	}
	s.Projects = kept
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
