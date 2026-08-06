package userconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	here      = "/repos/payment-api"
	elsewhere = "/repos/blog"
)

// A fresh install draws everywhere. Anything else would make the plugin
// invisible until the user found a setting they did not know existed.
func TestAnEmptyScopeAllowsEveryProject(t *testing.T) {
	if !LoadScope(t.TempDir()).Allows(here) {
		t.Fatal("a fresh scope refused a project")
	}
}

// The user's actual request: this project, not every window.
func TestAddingOneProjectExcludesTheRest(t *testing.T) {
	data := t.TempDir()

	if err := SaveScope(data, LoadScope(data).Add(here)); err != nil {
		t.Fatalf("save: %v", err)
	}

	scope := LoadScope(data)
	if !scope.Allows(here) {
		t.Fatal("the chosen project was excluded")
	}
	if scope.Allows(elsewhere) {
		t.Fatal("every other project is still included")
	}
}

// Removing the last entry has to restore every project. A list that matches
// nothing looks exactly like the bug this replaced: no status line anywhere,
// no error saying why.
func TestRemovingTheLastProjectRestoresEveryProject(t *testing.T) {
	data := t.TempDir()

	if err := SaveScope(data, LoadScope(data).Add(here)); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := SaveScope(data, LoadScope(data).Remove(here)); err != nil {
		t.Fatalf("save: %v", err)
	}

	for _, project := range []string{here, elsewhere} {
		if !LoadScope(data).Allows(project) {
			t.Fatalf("%s is still excluded after the list was emptied", project)
		}
	}
}

func TestAddingTwiceIsNotTwoEntries(t *testing.T) {
	data := t.TempDir()

	for i := 0; i < 2; i++ {
		if err := SaveScope(data, LoadScope(data).Add(here)); err != nil {
			t.Fatalf("save: %v", err)
		}
	}
	if got := LoadScope(data).Projects; len(got) != 1 {
		t.Fatalf("scope holds %d entries: %v", len(got), got)
	}
}

// The file travels no further than the plugin's own directory, but it is one
// more place a path could leak into.
func TestScopeStoresHashesRatherThanPaths(t *testing.T) {
	data := t.TempDir()

	if err := SaveScope(data, LoadScope(data).Add(here)); err != nil {
		t.Fatalf("save: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(data, "statusline-scope.json"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	for _, secret := range []string{here, "payment", "repos"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("the scope file contains %q:\n%s", secret, raw)
		}
	}
}

// A corrupt file must not blank the status line for everyone.
func TestACorruptScopeAllowsEveryProject(t *testing.T) {
	data := t.TempDir()

	if err := os.WriteFile(filepath.Join(data, "statusline-scope.json"),
		[]byte("{not json"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !LoadScope(data).Allows(here) {
		t.Fatal("a corrupt scope hid the status line")
	}
}
