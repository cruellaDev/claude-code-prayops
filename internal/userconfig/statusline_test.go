package userconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var frozen = time.Unix(1700000000, 0).UTC()

// TestMain clears the plugin environment so a real CLAUDE_CONFIG_DIR inherited
// from the developer's shell cannot be edited by these tests.
func TestMain(m *testing.M) {
	for _, key := range []string{"CLAUDE_PLUGIN_DATA", "CLAUDE_PLUGIN_ROOT", "CLAUDE_CONFIG_DIR"} {
		if err := os.Unsetenv(key); err != nil {
			panic(err)
		}
	}
	os.Exit(m.Run())
}

func newManager(t *testing.T) *Manager {
	t.Helper()
	m := NewManager(filepath.Join(t.TempDir(), ".claude", "settings.json"), t.TempDir())
	m.Now = func() time.Time { return frozen }
	return m
}

func writeSettings(t *testing.T, m *Manager, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(m.SettingsPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(m.SettingsPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}
}

func readSettings(t *testing.T, m *Manager) map[string]json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile(m.SettingsPath)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	out := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	return out
}

var line = NewStatusLine("/data/bin/prayops")

func TestInstallCreatesSettings(t *testing.T) {
	m := newManager(t)

	if _, err := m.InstallStatusLine(line); err != nil {
		t.Fatalf("install: %v", err)
	}

	settings := readSettings(t, m)
	if len(settings) != 1 {
		t.Fatalf("wrote %d keys, want 1", len(settings))
	}

	var got StatusLine
	if err := json.Unmarshal(settings[StatusLineKey], &got); err != nil {
		t.Fatalf("parse statusLine: %v", err)
	}
	if got.Type != "command" || got.Padding != 1 || got.RefreshInterval != 1 {
		t.Fatalf("statusLine = %+v", got)
	}
	if !strings.Contains(got.Command, "/data/bin/prayops") || !strings.Contains(got.Command, "statusline claude") {
		t.Fatalf("command = %q", got.Command)
	}
}

// FR-051: the rest of the user's settings must survive untouched.
func TestInstallPreservesOtherSettings(t *testing.T) {
	m := newManager(t)
	writeSettings(t, m, `{
	  "theme": "dark",
	  "permissions": {"allow": ["Bash(git status)"]},
	  "env": {"FOO": "bar"}
	}`)

	if _, err := m.InstallStatusLine(line); err != nil {
		t.Fatalf("install: %v", err)
	}

	settings := readSettings(t, m)
	if string(settings["theme"]) != `"dark"` {
		t.Fatalf("theme = %s", settings["theme"])
	}
	if !strings.Contains(string(settings["permissions"]), "git status") {
		t.Fatalf("permissions = %s", settings["permissions"])
	}
	if !strings.Contains(string(settings["env"]), "bar") {
		t.Fatalf("env = %s", settings["env"])
	}
}

// FR-051 / acceptance test 7: an existing status line is recorded and can be
// restored exactly.
func TestExistingStatusLineIsBackedUpAndRestored(t *testing.T) {
	m := newManager(t)
	const theirs = `{"type":"command","command":"~/bin/my-statusline.sh","padding":0}`
	writeSettings(t, m, `{"theme": "dark", "statusLine": `+theirs+`}`)

	record, err := m.InstallStatusLine(line)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !record.HadPrevious {
		t.Fatal("the existing status line was not recorded")
	}
	if record.BackupPath == "" {
		t.Fatal("no backup was taken")
	}
	backup, err := os.ReadFile(record.BackupPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if !strings.Contains(string(backup), "my-statusline.sh") {
		t.Fatalf("backup does not contain the original: %s", backup)
	}

	restored, err := m.UninstallStatusLine()
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if !restored {
		t.Fatal("uninstall reported no change")
	}

	settings := readSettings(t, m)
	var want, got any
	if err := json.Unmarshal([]byte(theirs), &want); err != nil {
		t.Fatalf("parse expected: %v", err)
	}
	if err := json.Unmarshal(settings[StatusLineKey], &got); err != nil {
		t.Fatalf("parse restored: %v", err)
	}
	if !equalJSON(want, got) {
		t.Fatalf("restored %s, want %s", settings[StatusLineKey], theirs)
	}
	if string(settings["theme"]) != `"dark"` {
		t.Fatalf("uninstall disturbed other settings: %s", settings["theme"])
	}
}

// With nothing there before, uninstall removes the key rather than leaving an
// empty one behind.
func TestUninstallRemovesTheKeyWhenThereWasNoneBefore(t *testing.T) {
	m := newManager(t)
	writeSettings(t, m, `{"theme": "dark"}`)

	if _, err := m.InstallStatusLine(line); err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := m.UninstallStatusLine(); err != nil {
		t.Fatalf("uninstall: %v", err)
	}

	settings := readSettings(t, m)
	if _, ok := settings[StatusLineKey]; ok {
		t.Fatalf("statusLine survived uninstall: %s", settings[StatusLineKey])
	}
	if string(settings["theme"]) != `"dark"` {
		t.Fatalf("theme = %s", settings["theme"])
	}
}

// Installing twice must not make PrayOps the value that uninstall restores.
func TestReinstallDoesNotOverwriteTheRecordedPrevious(t *testing.T) {
	m := newManager(t)
	writeSettings(t, m, `{"statusLine": {"type":"command","command":"mine.sh"}}`)

	if _, err := m.InstallStatusLine(line); err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := m.InstallStatusLine(NewStatusLine("/data/bin/prayops")); err != nil {
		t.Fatalf("reinstall: %v", err)
	}
	if _, err := m.InstallStatusLine(NewStatusLine("/other/bin/prayops")); err != nil {
		t.Fatalf("reinstall elsewhere: %v", err)
	}

	if _, err := m.UninstallStatusLine(); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	settings := readSettings(t, m)
	if !strings.Contains(string(settings[StatusLineKey]), "mine.sh") {
		t.Fatalf("restored %s, want the user's original", settings[StatusLineKey])
	}
}

// If the user replaced the status line themselves, uninstall must not undo
// their choice.
func TestUninstallLeavesAUserReplacedStatusLineAlone(t *testing.T) {
	m := newManager(t)
	if _, err := m.InstallStatusLine(line); err != nil {
		t.Fatalf("install: %v", err)
	}
	writeSettings(t, m, `{"statusLine": {"type":"command","command":"something-else.sh"}}`)

	changed, err := m.UninstallStatusLine()
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if changed {
		t.Fatal("uninstall changed a status line it did not install")
	}

	settings := readSettings(t, m)
	if !strings.Contains(string(settings[StatusLineKey]), "something-else.sh") {
		t.Fatalf("statusLine = %s", settings[StatusLineKey])
	}
}

func TestUninstallWithoutInstallIsANoOp(t *testing.T) {
	m := newManager(t)
	writeSettings(t, m, `{"theme": "dark"}`)

	changed, err := m.UninstallStatusLine()
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if changed {
		t.Fatal("uninstall reported a change it did not make")
	}
	if _, err := os.Stat(m.SettingsPath); err != nil {
		t.Fatalf("settings disappeared: %v", err)
	}
}

// Rewriting a file that failed to parse would discard whatever the user had.
func TestMalformedSettingsAreRefused(t *testing.T) {
	m := newManager(t)
	const broken = `{"theme": "dark",,,}`
	writeSettings(t, m, broken)

	if _, err := m.InstallStatusLine(line); err == nil {
		t.Fatal("install rewrote a settings file it could not parse")
	}

	raw, err := os.ReadFile(m.SettingsPath)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	if string(raw) != broken {
		t.Fatalf("settings were modified: %s", raw)
	}
}

func TestSettingsPathHonoursTheConfigDir(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "/tmp/somewhere")
	if got := SettingsPath(); got != filepath.Join("/tmp/somewhere", "settings.json") {
		t.Fatalf("path = %q", got)
	}
}

func equalJSON(a, b any) bool {
	left, err := json.Marshal(a)
	if err != nil {
		return false
	}
	right, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(left) == string(right)
}
