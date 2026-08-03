package userconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AliasRecord is what setup remembers about the optional /pray alias.
type AliasRecord struct {
	Installed   bool      `json:"installed"`
	Path        string    `json:"path,omitempty"`
	InstalledAt time.Time `json:"installedAt,omitempty"`
}

// ErrAliasConflict is returned when something already provides /pray.
type ErrAliasConflict struct{ Path string }

func (e ErrAliasConflict) Error() string {
	return fmt.Sprintf("userconfig: %s already exists", e.Path)
}

// aliasTemplate is the user-scope skill that forwards to the runtime.
//
// A user-scope skill has no ${CLAUDE_PLUGIN_ROOT}, so the runtime path is
// baked in at install time. Reinstalling after the runtime moves is the fix.
const aliasTemplate = `---
name: pray
description: Send one PrayOps prayer effect. Installed by /prayops:setup as an alias for /prayops:pray.
disable-model-invocation: true
---

Send one prayer effect through the PrayOps runtime.

1. With no argument:
   ` + "`%[1]q pray --host claude --cwd \"${CLAUDE_PROJECT_DIR}\" --preset deploy`" + `
2. With text:
   ` + "`%[1]q pray --host claude --cwd \"${CLAUDE_PROJECT_DIR}\" --text \"<user text>\"`" + `
3. With an explicitly provided image path:
   ` + "`%[1]q pray --host claude --cwd \"${CLAUDE_PROJECT_DIR}\" --image \"<path>\"`" + `
4. Never search for an image or inspect image contents.
5. Do not include prompts, code, commands, or tool output.
6. If the runtime is missing, tell the user to run /prayops:setup.

This file was installed by /prayops:setup and can be removed with
` + "`prayops alias uninstall --yes`" + `.
`

// ConfigDir returns the user's Claude Code configuration directory.
func ConfigDir() string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude")
}

// AliasPath is where the /pray alias skill lives.
func AliasPath(configDir string) string {
	return filepath.Join(configDir, "skills", "pray", "SKILL.md")
}

// AliasBody returns the skill file contents for a runtime path.
func AliasBody(runtimePath string) string {
	return fmt.Sprintf(aliasTemplate, runtimePath)
}

func (m *Manager) aliasRecordPath() string { return filepath.Join(m.DataDir, "alias.json") }

// LoadAliasRecord returns what setup remembers about the alias.
func (m *Manager) LoadAliasRecord() (AliasRecord, error) {
	var record AliasRecord

	raw, err := os.ReadFile(m.aliasRecordPath())
	if err != nil {
		if os.IsNotExist(err) {
			return record, nil
		}
		return record, fmt.Errorf("userconfig: read alias record: %w", err)
	}
	if err := json.Unmarshal(raw, &record); err != nil {
		return record, fmt.Errorf("userconfig: parse alias record: %w", err)
	}
	return record, nil
}

func (m *Manager) saveAliasRecord(record AliasRecord) error {
	if err := os.MkdirAll(m.DataDir, 0o700); err != nil {
		return fmt.Errorf("userconfig: create data dir: %w", err)
	}
	payload, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("userconfig: marshal alias record: %w", err)
	}
	return os.WriteFile(m.aliasRecordPath(), append(payload, '\n'), 0o600)
}

// InstallAlias writes the user-scope /pray skill.
//
// An existing file is never overwritten, whoever wrote it. The portable
// /prayops:pray keeps working either way, so there is nothing to gain from
// taking someone else's /pray.
func (m *Manager) InstallAlias(configDir, runtimePath string) (AliasRecord, error) {
	path := AliasPath(configDir)

	record, err := m.LoadAliasRecord()
	if err != nil {
		return record, err
	}

	if _, err := os.Stat(path); err == nil {
		if record.Installed && record.Path == path {
			// Ours already; refresh it in case the runtime moved.
			if err := os.WriteFile(path, []byte(AliasBody(runtimePath)), 0o644); err != nil {
				return record, fmt.Errorf("userconfig: write alias: %w", err)
			}
			return record, nil
		}
		return record, ErrAliasConflict{Path: path}
	} else if !os.IsNotExist(err) {
		return record, fmt.Errorf("userconfig: check alias: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return record, fmt.Errorf("userconfig: create alias dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(AliasBody(runtimePath)), 0o644); err != nil {
		return record, fmt.Errorf("userconfig: write alias: %w", err)
	}

	record = AliasRecord{Installed: true, Path: path, InstalledAt: m.Now().UTC()}
	return record, m.saveAliasRecord(record)
}

// UninstallAlias removes the alias, but only the one PrayOps wrote.
func (m *Manager) UninstallAlias() (bool, error) {
	record, err := m.LoadAliasRecord()
	if err != nil {
		return false, err
	}
	if !record.Installed || record.Path == "" {
		return false, nil
	}

	body, err := os.ReadFile(record.Path)
	if err != nil {
		if os.IsNotExist(err) {
			record.Installed = false
			return false, m.saveAliasRecord(record)
		}
		return false, fmt.Errorf("userconfig: read alias: %w", err)
	}

	// The user may have rewritten the file. Removing it then would delete
	// their work, so leave anything that is not recognisably ours.
	if !isPrayOpsAlias(string(body)) {
		record.Installed = false
		return false, m.saveAliasRecord(record)
	}

	if err := os.Remove(record.Path); err != nil {
		return false, fmt.Errorf("userconfig: remove alias: %w", err)
	}
	os.Remove(filepath.Dir(record.Path)) // only succeeds when empty

	record.Installed = false
	record.Path = ""
	return true, m.saveAliasRecord(record)
}

func isPrayOpsAlias(body string) bool {
	return strings.Contains(body, "Installed by /prayops:setup") ||
		strings.Contains(body, "installed by /prayops:setup")
}
