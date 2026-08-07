package userconfig

import (
	"os"
	"path/filepath"
	"strings"
)

// The theme is one word, so it is one line in one file. A JSON object would
// carry no more information and one more way to fail to parse.
func themePath(dataDir string) string { return filepath.Join(dataDir, "statusline-theme") }

// LoadTheme returns the chosen theme, or "" when none has been chosen. An
// unreadable file is treated the same way: the caller falls back to the
// default rather than drawing nothing.
func LoadTheme(dataDir string) string {
	raw, err := os.ReadFile(themePath(dataDir))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

// SaveTheme records the choice.
func SaveTheme(dataDir, theme string) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}

	// Write then rename, like everything else here: the status line reads this
	// file once a second and must never catch it half written.
	tmp := themePath(dataDir) + ".new"
	if err := os.WriteFile(tmp, []byte(theme+"\n"), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, themePath(dataDir))
}
