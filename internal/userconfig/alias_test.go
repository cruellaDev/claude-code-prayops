package userconfig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func aliasWorld(t *testing.T) (*Manager, string) {
	t.Helper()
	base := t.TempDir()
	configDir := filepath.Join(base, "config")
	m := NewManager(filepath.Join(configDir, "settings.json"), filepath.Join(base, "data"))
	m.Now = func() time.Time { return frozen }
	return m, configDir
}

func TestInstallAliasWritesAValidSkill(t *testing.T) {
	m, configDir := aliasWorld(t)

	record, err := m.InstallAlias(configDir, "/data/bin/prayops")
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !record.Installed {
		t.Fatal("record does not say installed")
	}

	body, err := os.ReadFile(AliasPath(configDir))
	if err != nil {
		t.Fatalf("read alias: %v", err)
	}
	text := string(body)

	if !strings.HasPrefix(text, "---\n") {
		t.Fatalf("alias has no frontmatter:\n%s", text)
	}
	for _, want := range []string{
		"name: pray",
		"disable-model-invocation: true",
		"/data/bin/prayops",
		"--preset deploy",
		"Never search for an image",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("alias does not contain %q:\n%s", want, text)
		}
	}
}

// FR-040 / acceptance test 8: an existing /pray belongs to whoever wrote it.
func TestExistingAliasIsNeverOverwritten(t *testing.T) {
	m, configDir := aliasWorld(t)
	path := AliasPath(configDir)
	const theirs = "---\nname: pray\n---\n\nmy own pray skill\n"

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(theirs), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := m.InstallAlias(configDir, "/data/bin/prayops")
	var conflict ErrAliasConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("err = %v, want an alias conflict", err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(body) != theirs {
		t.Fatalf("their file was modified:\n%s", body)
	}

	record, err := m.LoadAliasRecord()
	if err != nil {
		t.Fatalf("load record: %v", err)
	}
	if record.Installed {
		t.Fatal("a conflict was recorded as an installation")
	}
}

func TestReinstallRefreshesTheRuntimePath(t *testing.T) {
	m, configDir := aliasWorld(t)

	if _, err := m.InstallAlias(configDir, "/old/bin/prayops"); err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := m.InstallAlias(configDir, "/new/bin/prayops"); err != nil {
		t.Fatalf("reinstall: %v", err)
	}

	body, err := os.ReadFile(AliasPath(configDir))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(body), "/new/bin/prayops") {
		t.Fatalf("alias still points at the old runtime:\n%s", body)
	}
}

func TestUninstallRemovesOnlyOurAlias(t *testing.T) {
	m, configDir := aliasWorld(t)

	if _, err := m.InstallAlias(configDir, "/data/bin/prayops"); err != nil {
		t.Fatalf("install: %v", err)
	}

	removed, err := m.UninstallAlias()
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if !removed {
		t.Fatal("uninstall reported no change")
	}
	if _, err := os.Stat(AliasPath(configDir)); !os.IsNotExist(err) {
		t.Fatal("the alias survived uninstall")
	}
}

// If the user edited the alias, removing it would delete their work.
func TestUninstallLeavesAnEditedAliasAlone(t *testing.T) {
	m, configDir := aliasWorld(t)
	if _, err := m.InstallAlias(configDir, "/data/bin/prayops"); err != nil {
		t.Fatalf("install: %v", err)
	}

	const edited = "---\nname: pray\n---\n\nI rewrote this entirely.\n"
	if err := os.WriteFile(AliasPath(configDir), []byte(edited), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	removed, err := m.UninstallAlias()
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if removed {
		t.Fatal("uninstall deleted a file the user had rewritten")
	}

	body, err := os.ReadFile(AliasPath(configDir))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(body) != edited {
		t.Fatalf("the user's file changed:\n%s", body)
	}
}

func TestUninstallAliasWithoutInstallIsANoOp(t *testing.T) {
	m, _ := aliasWorld(t)

	removed, err := m.UninstallAlias()
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if removed {
		t.Fatal("uninstall reported a change it did not make")
	}
}

func TestUninstallToleratesAManuallyDeletedAlias(t *testing.T) {
	m, configDir := aliasWorld(t)
	if _, err := m.InstallAlias(configDir, "/data/bin/prayops"); err != nil {
		t.Fatalf("install: %v", err)
	}
	if err := os.Remove(AliasPath(configDir)); err != nil {
		t.Fatalf("remove: %v", err)
	}

	if _, err := m.UninstallAlias(); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	record, err := m.LoadAliasRecord()
	if err != nil {
		t.Fatalf("load record: %v", err)
	}
	if record.Installed {
		t.Fatal("the record still claims the alias is installed")
	}
}
