// Package userconfig edits the user's Claude Code settings on request.
//
// A plugin cannot set the user's statusLine from its own settings, so setup
// has to write into ~/.claude/settings.json. Everything here is reversible:
// the whole file is copied before the first change, the replaced value is
// recorded, and uninstall puts it back.
package userconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// StatusLineKey is the settings key PrayOps writes.
const StatusLineKey = "statusLine"

// StatusLine is the settings value that runs the PrayOps status line.
type StatusLine struct {
	Type            string `json:"type"`
	Command         string `json:"command"`
	Padding         int    `json:"padding"`
	RefreshInterval int    `json:"refreshInterval"`
}

// NewStatusLine returns the settings value for an installed runtime.
func NewStatusLine(runtimePath string) StatusLine {
	return StatusLine{
		Type:            "command",
		Command:         fmt.Sprintf("%q statusline claude", runtimePath),
		Padding:         1,
		RefreshInterval: 1,
	}
}

// Record is what setup remembers so uninstall can undo its work.
type Record struct {
	Installed   bool            `json:"installed"`
	Command     string          `json:"command,omitempty"`
	Previous    json.RawMessage `json:"previous,omitempty"`
	HadPrevious bool            `json:"hadPrevious"`
	BackupPath  string          `json:"backupPath,omitempty"`
	InstalledAt time.Time       `json:"installedAt,omitempty"`
}

// Manager edits one settings file and keeps its undo record in the plugin
// data directory.
type Manager struct {
	SettingsPath string
	DataDir      string
	Now          func() time.Time
}

// NewManager returns a manager for the given settings file.
func NewManager(settingsPath, dataDir string) *Manager {
	return &Manager{SettingsPath: settingsPath, DataDir: dataDir, Now: time.Now}
}

// SettingsPath returns the user's Claude Code settings file.
func SettingsPath() string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "settings.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "settings.json")
}

func (m *Manager) recordPath() string { return filepath.Join(m.DataDir, "install.json") }

// readSettings returns the settings as a generic map so keys PrayOps knows
// nothing about survive the round trip.
//
// A missing file is an empty object. A malformed one is an error: rewriting a
// file that failed to parse would discard whatever the user actually had.
func (m *Manager) readSettings() (map[string]json.RawMessage, error) {
	raw, err := os.ReadFile(m.SettingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]json.RawMessage{}, nil
		}
		return nil, fmt.Errorf("userconfig: read settings: %w", err)
	}
	if len(raw) == 0 {
		return map[string]json.RawMessage{}, nil
	}

	settings := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &settings); err != nil {
		return nil, fmt.Errorf("userconfig: %s is not valid JSON, refusing to rewrite it: %w", m.SettingsPath, err)
	}
	return settings, nil
}

func (m *Manager) writeSettings(settings map[string]json.RawMessage) error {
	payload, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("userconfig: marshal settings: %w", err)
	}
	payload = append(payload, '\n')

	if err := os.MkdirAll(filepath.Dir(m.SettingsPath), 0o755); err != nil {
		return fmt.Errorf("userconfig: create settings dir: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(m.SettingsPath), "settings-*.json")
	if err != nil {
		return fmt.Errorf("userconfig: create temp settings: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("userconfig: write settings: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("userconfig: close settings: %w", err)
	}
	if err := os.Rename(tmpName, m.SettingsPath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("userconfig: replace settings: %w", err)
	}
	return nil
}

// backup copies the whole settings file, not just the key being replaced, so
// a mistake anywhere in this package is still recoverable by hand.
func (m *Manager) backup() (string, error) {
	raw, err := os.ReadFile(m.SettingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("userconfig: read settings for backup: %w", err)
	}

	dir := filepath.Join(m.DataDir, "backups")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("userconfig: create backup dir: %w", err)
	}

	path := filepath.Join(dir, fmt.Sprintf("settings-%s.json", m.Now().UTC().Format("20060102T150405Z")))
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return "", fmt.Errorf("userconfig: write backup: %w", err)
	}
	return path, nil
}

// LoadRecord returns what setup remembers about the status line.
func (m *Manager) LoadRecord() (Record, error) {
	var record Record

	raw, err := os.ReadFile(m.recordPath())
	if err != nil {
		if os.IsNotExist(err) {
			return record, nil
		}
		return record, fmt.Errorf("userconfig: read install record: %w", err)
	}

	var file struct {
		StatusLine Record `json:"statusLine"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		return record, fmt.Errorf("userconfig: parse install record: %w", err)
	}
	return file.StatusLine, nil
}

func (m *Manager) saveRecord(record Record) error {
	if err := os.MkdirAll(m.DataDir, 0o700); err != nil {
		return fmt.Errorf("userconfig: create data dir: %w", err)
	}

	payload, err := json.MarshalIndent(struct {
		StatusLine Record `json:"statusLine"`
	}{record}, "", "  ")
	if err != nil {
		return fmt.Errorf("userconfig: marshal install record: %w", err)
	}
	return os.WriteFile(m.recordPath(), append(payload, '\n'), 0o600)
}

// Current returns the statusLine value in the settings file, if any.
func (m *Manager) Current() (json.RawMessage, bool, error) {
	settings, err := m.readSettings()
	if err != nil {
		return nil, false, err
	}
	value, ok := settings[StatusLineKey]
	return value, ok, nil
}

// InstallStatusLine points the user's statusLine at the PrayOps runtime.
//
// Any existing value is copied into the install record and the whole file is
// backed up first. Installing twice is a no-op rather than a second backup of
// the value PrayOps itself wrote.
func (m *Manager) InstallStatusLine(line StatusLine) (Record, error) {
	settings, err := m.readSettings()
	if err != nil {
		return Record{}, err
	}

	desired, err := json.Marshal(line)
	if err != nil {
		return Record{}, fmt.Errorf("userconfig: marshal status line: %w", err)
	}

	record, err := m.LoadRecord()
	if err != nil {
		return Record{}, err
	}

	existing, had := settings[StatusLineKey]
	if had && string(existing) == string(desired) {
		return record, nil
	}

	backupPath, err := m.backup()
	if err != nil {
		return Record{}, err
	}

	// Only remember a previous value that is not one of ours, or reinstalling
	// over a PrayOps line would make PrayOps the thing uninstall restores.
	if !record.Installed || !isPrayOps(existing) {
		record.Previous = nil
		record.HadPrevious = false
		if had {
			record.Previous = append(json.RawMessage(nil), existing...)
			record.HadPrevious = true
		}
	}
	record.Installed = true
	record.Command = line.Command
	record.BackupPath = backupPath
	record.InstalledAt = m.Now().UTC()

	settings[StatusLineKey] = desired
	if err := m.writeSettings(settings); err != nil {
		return Record{}, err
	}
	return record, m.saveRecord(record)
}

// UninstallStatusLine puts back whatever was there before.
//
// If the current value is not the one PrayOps installed, the user has changed
// it since and it is left alone.
func (m *Manager) UninstallStatusLine() (bool, error) {
	record, err := m.LoadRecord()
	if err != nil {
		return false, err
	}
	if !record.Installed {
		return false, nil
	}

	settings, err := m.readSettings()
	if err != nil {
		return false, err
	}

	current, ok := settings[StatusLineKey]
	if !ok || !isPrayOps(current) {
		record.Installed = false
		return false, m.saveRecord(record)
	}

	if record.HadPrevious {
		settings[StatusLineKey] = record.Previous
	} else {
		delete(settings, StatusLineKey)
	}
	if err := m.writeSettings(settings); err != nil {
		return false, err
	}

	record.Installed = false
	record.Previous = nil
	record.HadPrevious = false
	return true, m.saveRecord(record)
}

// isPrayOps reports whether a statusLine value runs the PrayOps runtime.
func isPrayOps(value json.RawMessage) bool {
	var line StatusLine
	if err := json.Unmarshal(value, &line); err != nil {
		return false
	}
	return line.Type == "command" &&
		strings.Contains(line.Command, "prayops") &&
		strings.Contains(line.Command, "statusline")
}
