package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var now = time.Unix(1700000000, 0).UTC()

// world builds a plugin root and plugin data directory to diagnose.
type world struct {
	root     string
	data     string
	settings string
	env      map[string]string
}

func newWorld(t *testing.T) *world {
	t.Helper()
	base := t.TempDir()
	w := &world{
		root:     filepath.Join(base, "plugin"),
		data:     filepath.Join(base, "data"),
		settings: filepath.Join(base, "config", "settings.json"),
		env:      map[string]string{"CLAUDE_CONFIG_DIR": filepath.Join(base, "config")},
	}
	for _, dir := range []string{w.root, w.data, filepath.Join(base, "config")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	return w
}

func (w *world) write(t *testing.T, rel, body string) {
	t.Helper()
	path := filepath.Join(w.root, rel)
	if strings.HasPrefix(rel, "@data/") {
		path = filepath.Join(w.data, strings.TrimPrefix(rel, "@data/"))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func (w *world) run() Report {
	return Run(Options{
		PluginRoot: w.root,
		PluginData: w.data,
		Settings:   w.settings,
		Now:        now,
		Env:        func(key string) string { return w.env[key] },
	})
}

func check(t *testing.T, report Report, name string) Check {
	t.Helper()
	for _, c := range report.Checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no %q check in the report", name)
	return Check{}
}

func healthy(t *testing.T) *world {
	t.Helper()
	w := newWorld(t)
	w.write(t, ".claude-plugin/plugin.json", `{"name":"prayops","version":"0.1.0"}`)
	w.write(t, "runtime-manifest.json", `{"runtimeVersion":"0.1.0"}`)
	w.write(t, "@data/runtime.json", `{"runtimeVersion":"0.1.0","sha256":"abc"}`)
	w.write(t, "@data/bin/prayops", "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(w.data, "bin", "prayops"), 0o755); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	return w
}

func TestHealthyInstallation(t *testing.T) {
	w := healthy(t)
	w.write(t, "@data/state/sessions/s1.json", `{"sessionId":"s1","phase":"WORKING"}`)
	w.write(t, "@data/state/events/inbox/.keep", "")

	report := w.run()
	if report.Failed() {
		t.Fatalf("healthy installation reported a failure:\n%s", report)
	}
	if got := check(t, report, "Plugin"); got.Status != StatusOK || got.Detail != "0.1.0" {
		t.Fatalf("plugin check = %+v", got)
	}
	if got := check(t, report, "Runtime"); got.Status != StatusOK || got.Detail != "0.1.0" {
		t.Fatalf("runtime check = %+v", got)
	}
	if got := check(t, report, "Version"); got.Status != StatusOK {
		t.Fatalf("version check = %+v", got)
	}
	if got := check(t, report, "Hooks"); got.Status != StatusOK {
		t.Fatalf("hooks check = %+v", got)
	}
}

// FR-026 / acceptance test 6: a plugin update that outpaces the runtime has to
// be visible.
func TestRuntimeVersionMismatchIsReported(t *testing.T) {
	w := healthy(t)
	w.write(t, "runtime-manifest.json", `{"runtimeVersion":"0.2.0"}`)

	got := check(t, w.run(), "Version")
	if got.Status != StatusWarn {
		t.Fatalf("version check = %+v", got)
	}
	if !strings.Contains(got.Detail, "0.2.0") || !strings.Contains(got.Detail, "0.1.0") {
		t.Fatalf("detail = %q, want both versions", got.Detail)
	}
	if got.Repair == "" {
		t.Fatal("no repair suggestion for a version mismatch")
	}
}

func TestMissingRuntimeIsAFailureWithARepair(t *testing.T) {
	w := newWorld(t)
	w.write(t, ".claude-plugin/plugin.json", `{"version":"0.1.0"}`)
	w.write(t, "runtime-manifest.json", `{"runtimeVersion":"0.1.0"}`)

	report := w.run()
	if !report.Failed() {
		t.Fatalf("a missing runtime was not a failure:\n%s", report)
	}
	got := check(t, report, "Runtime")
	if got.Status != StatusFail || !strings.Contains(got.Repair, "/prayops:setup") {
		t.Fatalf("runtime check = %+v", got)
	}
}

func TestNonExecutableRuntimeIsAFailure(t *testing.T) {
	w := healthy(t)
	if err := os.Chmod(filepath.Join(w.data, "bin", "prayops"), 0o644); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	got := check(t, w.run(), "Runtime")
	if got.Status != StatusFail || !strings.Contains(got.Detail, "not executable") {
		t.Fatalf("runtime check = %+v", got)
	}
}

// The doctor cannot read Claude Code's hook configuration, so it reports the
// evidence it does have rather than claiming hooks are wired.
func TestHooksReportEvidenceNotConfiguration(t *testing.T) {
	w := healthy(t)

	got := check(t, w.run(), "Hooks")
	if got.Status != StatusWarn || !strings.Contains(got.Detail, "no events") {
		t.Fatalf("hooks check with no events = %+v", got)
	}

	w.write(t, "@data/state/sessions/s1.json", `{"sessionId":"s1"}`)
	got = check(t, w.run(), "Hooks")
	if got.Status != StatusOK || !strings.Contains(got.Detail, "last event") {
		t.Fatalf("hooks check with events = %+v", got)
	}
}

func TestHookErrorsSurface(t *testing.T) {
	w := healthy(t)
	w.write(t, "@data/state/hook-last-error.txt", "2026-08-03T12:00:00Z spool: disk full\n")

	got := check(t, w.run(), "Hooks")
	if got.Status != StatusWarn || !strings.Contains(got.Detail, "disk full") {
		t.Fatalf("hooks check = %+v", got)
	}
}

func TestRejectedEventsSurface(t *testing.T) {
	w := healthy(t)
	w.write(t, "@data/state/events/rejected/bad.json", "{")

	got := check(t, w.run(), "State")
	if got.Status != StatusWarn || !strings.Contains(got.Detail, "1 rejected") {
		t.Fatalf("state check = %+v", got)
	}
}

func TestStatusLineReportsWhoConfiguredIt(t *testing.T) {
	w := healthy(t)

	got := check(t, w.run(), "Status line")
	if !strings.Contains(got.Detail, "not configured") {
		t.Fatalf("status line check = %+v", got)
	}

	if err := os.WriteFile(w.settings, []byte(`{"statusLine":{"type":"command","command":"mine.sh"}}`), 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}
	got = check(t, w.run(), "Status line")
	if !strings.Contains(got.Detail, "configured by you") {
		t.Fatalf("status line check = %+v", got)
	}
}

func TestBrokenSettingsAreReportedNotIgnored(t *testing.T) {
	w := healthy(t)
	if err := os.WriteFile(w.settings, []byte(`{,,}`), 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	got := check(t, w.run(), "Status line")
	if got.Status != StatusWarn {
		t.Fatalf("status line check = %+v", got)
	}
}

func TestAliasDetection(t *testing.T) {
	w := healthy(t)

	got := check(t, w.run(), "Alias /pray")
	if !strings.Contains(got.Detail, "not installed") {
		t.Fatalf("alias check = %+v", got)
	}

	alias := filepath.Join(w.env["CLAUDE_CONFIG_DIR"], "skills", "pray", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(alias), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(alias, []byte("---\nname: pray\n---\n"), 0o644); err != nil {
		t.Fatalf("write alias: %v", err)
	}

	got = check(t, w.run(), "Alias /pray")
	if !strings.Contains(got.Detail, "SKILL.md") {
		t.Fatalf("alias check = %+v", got)
	}
}

// A watcher that is not running is normal, not a problem.
func TestWatcherAbsenceIsNotAFailure(t *testing.T) {
	w := healthy(t)

	got := check(t, w.run(), "Watcher")
	if got.Status != StatusOK || !strings.Contains(got.Detail, "not running") {
		t.Fatalf("watcher check = %+v", got)
	}

	beat := filepath.Join(w.data, "state", "watchers", "w1")
	if err := os.MkdirAll(filepath.Dir(beat), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(beat, []byte("1"), 0o644); err != nil {
		t.Fatalf("write heartbeat: %v", err)
	}
	if err := os.Chtimes(beat, now, now); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	got = check(t, w.run(), "Watcher")
	if got.Detail != "running" {
		t.Fatalf("watcher check = %+v", got)
	}
}

func TestTerminalCapability(t *testing.T) {
	cases := []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"COLORTERM": "truecolor"}, "truecolor"},
		{map[string]string{"TERM": "xterm-256color"}, "256 colours"},
		{map[string]string{"TERM": "vt100"}, "vt100"},
		{map[string]string{"NO_COLOR": "1", "COLORTERM": "truecolor"}, "NO_COLOR"},
		{map[string]string{}, "TERM is not set"},
	}

	for _, tc := range cases {
		w := healthy(t)
		w.env = tc.env
		got := check(t, w.run(), "Terminal")
		if !strings.Contains(got.Detail, tc.want) {
			t.Fatalf("env %v gave %q, want %q", tc.env, got.Detail, tc.want)
		}
	}
}

// Doctor is read-only: it must never create or repair anything itself.
func TestDoctorChangesNothing(t *testing.T) {
	w := newWorld(t)

	before := snapshot(t, filepath.Dir(w.data))
	w.run()
	after := snapshot(t, filepath.Dir(w.data))

	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Fatalf("doctor modified the filesystem:\nbefore %v\nafter  %v", before, after)
	}
}

func snapshot(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		paths = append(paths, strings.TrimPrefix(path, root))
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return paths
}

func TestUnknownPathsDoNotCrash(t *testing.T) {
	report := Run(Options{Now: now, Env: func(string) string { return "" }})

	if len(report.Checks) == 0 {
		t.Fatal("no checks ran")
	}
	if report.String() == "" {
		t.Fatal("empty report")
	}
}

func TestReportIsAligned(t *testing.T) {
	w := healthy(t)
	rendered := w.run().String()

	for _, line := range strings.Split(strings.TrimSpace(rendered), "\n") {
		if strings.TrimSpace(line) == "" {
			t.Fatalf("blank line in the report:\n%s", rendered)
		}
	}
	if !strings.Contains(rendered, "Plugin") || !strings.Contains(rendered, "Terminal") {
		t.Fatalf("report is missing checks:\n%s", rendered)
	}
}
