// Package e2e drives a whole PrayOps installation the way a user would.
//
// The bootstrap tests use a stand-in binary to exercise the installer's
// failure paths. This package installs the real one from a local release and
// then uses it, so the acceptance criteria are checked against what ships.
package e2e

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const version = "0.1.0" // must match plugins/prayops/runtime-manifest.json

// world is one user's machine: a plugin root, a plugin data directory, a
// Claude config directory, and a release server.
type world struct {
	t          *testing.T
	pluginRoot string
	data       string
	config     string
	releaseURL string
}

func setup(t *testing.T) *world {
	t.Helper()

	base := t.TempDir()
	w := &world{
		t:          t,
		pluginRoot: absPath(t, "../../plugins/prayops"),
		data:       filepath.Join(base, "data"),
		config:     filepath.Join(base, "config"),
	}
	w.releaseURL = w.serveRelease(buildRuntime(t, version))
	return w
}

func absPath(t *testing.T, rel string) string {
	t.Helper()
	path, err := filepath.Abs(rel)
	if err != nil {
		t.Fatalf("resolve %s: %v", rel, err)
	}
	return path
}

// buildRuntime compiles the real binary with the release version baked in.
func buildRuntime(t *testing.T, v string) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "prayops")
	cmd := exec.Command("go", "build", "-ldflags", "-X main.version="+v, "-o", binary, "./cmd/prayops")
	cmd.Dir = "../.."

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return binary
}

func assetName(v string) string {
	return fmt.Sprintf("prayops_%s_%s_%s.tar.gz", v, runtime.GOOS, runtime.GOARCH)
}

// serveRelease packages a binary the way GoReleaser does and serves it.
func (w *world) serveRelease(binary string) string {
	w.t.Helper()

	body, err := os.ReadFile(binary)
	if err != nil {
		w.t.Fatalf("read binary: %v", err)
	}

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	if err := tw.WriteHeader(&tar.Header{
		Name: "prayops", Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg,
	}); err != nil {
		w.t.Fatalf("tar header: %v", err)
	}
	if _, err := tw.Write(body); err != nil {
		w.t.Fatalf("tar write: %v", err)
	}
	if err := tw.Close(); err != nil {
		w.t.Fatalf("tar close: %v", err)
	}
	if err := zw.Close(); err != nil {
		w.t.Fatalf("gzip close: %v", err)
	}

	archive := buf.Bytes()
	sum := sha256.Sum256(archive)

	// The version in the asset name has to match whatever the manifest asks
	// for, which is what the update test varies.
	name := assetName(w.manifestVersion())
	checksums := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), name)

	mux := http.NewServeMux()
	mux.HandleFunc("/"+name, func(rw http.ResponseWriter, r *http.Request) { rw.Write(archive) })
	mux.HandleFunc("/checksums.txt", func(rw http.ResponseWriter, r *http.Request) {
		rw.Write([]byte(checksums))
	})

	server := httptest.NewServer(mux)
	w.t.Cleanup(server.Close)
	return server.URL
}

func (w *world) manifestVersion() string {
	w.t.Helper()

	raw, err := os.ReadFile(filepath.Join(w.pluginRoot, "runtime-manifest.json"))
	if err != nil {
		w.t.Fatalf("read manifest: %v", err)
	}
	var manifest struct {
		RuntimeVersion string `json:"runtimeVersion"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		w.t.Fatalf("parse manifest: %v", err)
	}
	return manifest.RuntimeVersion
}

func (w *world) env() []string {
	return append(os.Environ(),
		"CLAUDE_PLUGIN_ROOT="+w.pluginRoot,
		"CLAUDE_PLUGIN_DATA="+w.data,
		"CLAUDE_CONFIG_DIR="+w.config,
		"PRAYOPS_RELEASE_BASE_URL="+w.releaseURL,
		"NO_COLOR=1",
	)
}

type result struct {
	code   int
	output string
}

// script runs a bundled plugin script, the way a skill would.
func (w *world) script(name string, args ...string) result {
	w.t.Helper()
	return w.exec(filepath.Join(w.pluginRoot, "scripts", name), "", args...)
}

// runtime runs the installed binary.
func (w *world) runtime(stdin string, args ...string) result {
	w.t.Helper()
	return w.exec(filepath.Join(w.data, "bin", "prayops"), stdin, args...)
}

func (w *world) exec(command, stdin string, args ...string) result {
	w.t.Helper()

	cmd := exec.Command(command, args...)
	cmd.Env = w.env()
	cmd.Stdin = strings.NewReader(stdin)

	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			w.t.Fatalf("run %s %v: %v\n%s", command, args, err, out)
		}
		code = exitErr.ExitCode()
	}
	return result{code: code, output: string(out)}
}

func (w *world) install() {
	w.t.Helper()
	if got := w.script("setup.sh", "--yes"); got.code != 0 {
		w.t.Fatalf("install failed: exit %d\n%s", got.code, got.output)
	}
}

// REL-01: the whole path a new user walks, in order.
func TestInstallationEndToEnd(t *testing.T) {
	w := setup(t)

	// Acceptance 3: with no runtime, the shipped hook command is a silent
	// no-op rather than an error.
	before := w.exec("/bin/sh", `{"session_id":"s1","hook_event_name":"Stop"}`,
		"-c", `test ! -x "${CLAUDE_PLUGIN_DATA}/bin/prayops" || "${CLAUDE_PLUGIN_DATA}/bin/prayops" hook claude`)
	if before.code != 0 || before.output != "" {
		t.Fatalf("hook before install: exit %d, output %q", before.code, before.output)
	}

	// Acceptance 4: the first setup shows what it would install and stops.
	plan := w.script("setup.sh")
	if plan.code != 10 {
		t.Fatalf("plan exit = %d, want 10\n%s", plan.code, plan.output)
	}
	for _, needed := range []string{version, "SHA-256", assetName(version), runtime.GOOS} {
		if !strings.Contains(plan.output, needed) {
			t.Fatalf("plan does not mention %q:\n%s", needed, plan.output)
		}
	}
	if _, err := os.Stat(filepath.Join(w.data, "bin", "prayops")); !os.IsNotExist(err) {
		t.Fatal("the plan installed something")
	}

	w.install()

	// The installed binary is the real one and reports the release version.
	got := w.runtime("", "version", "--json")
	if got.code != 0 || !strings.Contains(got.output, `"version":"`+version+`"`) {
		t.Fatalf("version = %q", got.output)
	}

	// A session's worth of hooks.
	for _, payload := range []string{
		`{"session_id":"s1","hook_event_name":"UserPromptSubmit","cwd":"/repos/payment-api","prompt":"secret"}`,
		`{"session_id":"s1","hook_event_name":"PreToolUse","cwd":"/repos/payment-api","tool_name":"Bash"}`,
	} {
		if got := w.runtime(payload, "hook", "claude"); got.code != 0 || got.output != "" {
			t.Fatalf("hook: exit %d, output %q", got.code, got.output)
		}
	}

	// The status line reflects them.
	line := w.runtime(`{"session_id":"s1"}`, "statusline", "claude")
	if !strings.Contains(line.output, "WORKING") {
		t.Fatalf("status line = %q", line.output)
	}

	// A prayer is accepted and cached.
	if got := w.runtime("", "pray", "--cwd", "/repos/payment-api", "--text", "무사배포"); got.code != 0 {
		t.Fatalf("pray: exit %d\n%s", got.code, got.output)
	}
	cached, err := os.ReadDir(filepath.Join(w.data, "state", "cache"))
	if err != nil || len(cached) != 1 {
		t.Fatalf("cache holds %d entries: %v", len(cached), err)
	}

	// Doctor sees a healthy installation.
	health := w.runtime("", "doctor")
	if health.code != 0 {
		t.Fatalf("doctor reported a problem:\n%s", health.output)
	}
	for _, needed := range []string{"Plugin", "Runtime", "Hooks", "Terminal"} {
		if !strings.Contains(health.output, needed) {
			t.Fatalf("doctor is missing %q:\n%s", needed, health.output)
		}
	}

	// Acceptance 10: the watcher refuses to run without a terminal, which is
	// what happens inside Claude Code's Bash tool.
	watch := w.runtime("", "watch")
	if watch.code == 0 {
		t.Fatalf("the watcher started without a terminal:\n%s", watch.output)
	}
	if !strings.Contains(watch.output, "terminal") {
		t.Fatalf("unhelpful watcher message:\n%s", watch.output)
	}
}

// REL-01, acceptance 7 and 8: optional configuration is reversible.
func TestOptionalConfigurationRoundTrips(t *testing.T) {
	w := setup(t)
	w.install()

	settings := filepath.Join(w.config, "settings.json")
	if err := os.MkdirAll(w.config, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	const original = `{"theme":"dark","statusLine":{"type":"command","command":"mine.sh","padding":0}}`
	if err := os.WriteFile(settings, []byte(original), 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	if got := w.runtime("", "statusline", "install", "--yes"); got.code != 0 {
		t.Fatalf("statusline install: %s", got.output)
	}
	if got := w.runtime("", "statusline", "uninstall", "--yes"); got.code != 0 {
		t.Fatalf("statusline uninstall: %s", got.output)
	}

	restored, err := os.ReadFile(settings)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	if !strings.Contains(string(restored), "mine.sh") || !strings.Contains(string(restored), "dark") {
		t.Fatalf("settings were not restored:\n%s", restored)
	}

	// An existing /pray is never taken.
	alias := filepath.Join(w.config, "skills", "pray", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(alias), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	const theirs = "---\nname: pray\n---\n\nmine\n"
	if err := os.WriteFile(alias, []byte(theirs), 0o644); err != nil {
		t.Fatalf("write alias: %v", err)
	}

	if got := w.runtime("", "alias", "install", "--yes"); got.code != 3 {
		t.Fatalf("alias install exit = %d, want 3\n%s", got.code, got.output)
	}
	body, err := os.ReadFile(alias)
	if err != nil {
		t.Fatalf("read alias: %v", err)
	}
	if string(body) != theirs {
		t.Fatalf("their alias changed:\n%s", body)
	}
}

// REL-02: a plugin update that outruns the runtime.
func TestUpdateEndToEnd(t *testing.T) {
	w := setup(t)
	w.install()

	// A newer plugin ships a newer manifest. Copy the plugin root so the real
	// one is not edited.
	newRoot := t.TempDir()
	copyTree(t, w.pluginRoot, newRoot)
	writeManifestVersion(t, newRoot, "0.2.0")
	w.pluginRoot = newRoot

	// Acceptance 6: doctor reports the mismatch and says what to do.
	health := w.runtime("", "doctor")
	if !strings.Contains(health.output, "0.2.0") || !strings.Contains(health.output, "0.1.0") {
		t.Fatalf("doctor does not report the mismatch:\n%s", health.output)
	}
	if !strings.Contains(health.output, "/prayops:setup") {
		t.Fatalf("doctor does not say how to fix it:\n%s", health.output)
	}

	// Declining the update leaves the working runtime in place.
	plan := w.script("setup.sh")
	if plan.code != 10 {
		t.Fatalf("update plan exit = %d\n%s", plan.code, plan.output)
	}
	if !strings.Contains(plan.output, "Updating from v0.1.0 to v0.2.0") {
		t.Fatalf("the plan does not describe an update:\n%s", plan.output)
	}
	if got := w.runtime("", "version"); !strings.Contains(got.output, version) {
		t.Fatalf("the runtime changed after a declined update: %q", got.output)
	}

	// Acceptance 5: an update whose checksum does not match must not replace
	// the working runtime.
	w.releaseURL = w.serveCorrupt("0.2.0")
	if got := w.script("setup.sh", "--yes"); got.code != 13 {
		t.Fatalf("corrupt update exit = %d, want 13\n%s", got.code, got.output)
	}
	if got := w.runtime("", "version"); !strings.Contains(got.output, version) {
		t.Fatalf("a corrupt update replaced the runtime: %q", got.output)
	}

	// A good update succeeds and the version moves.
	w.releaseURL = w.serveRelease(buildRuntime(t, "0.2.0"))
	if got := w.script("setup.sh", "--yes"); got.code != 0 {
		t.Fatalf("update: exit %d\n%s", got.code, got.output)
	}
	if got := w.runtime("", "version"); !strings.Contains(got.output, "0.2.0") {
		t.Fatalf("version after update = %q", got.output)
	}

	// And running setup again is a no-op.
	if got := w.script("setup.sh"); got.code != 0 || !strings.Contains(got.output, "already installed") {
		t.Fatalf("repeat setup: exit %d\n%s", got.code, got.output)
	}
}

// serveCorrupt advertises a checksum that does not match the archive.
func (w *world) serveCorrupt(v string) string {
	w.t.Helper()

	name := assetName(v)
	mux := http.NewServeMux()
	mux.HandleFunc("/"+name, func(rw http.ResponseWriter, r *http.Request) {
		rw.Write([]byte("this is not the binary you are looking for"))
	})
	mux.HandleFunc("/checksums.txt", func(rw http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(rw, "%s  %s\n", strings.Repeat("0", 64), name)
	})

	server := httptest.NewServer(mux)
	w.t.Cleanup(server.Close)
	return server.URL
}

func copyTree(t *testing.T, from, to string) {
	t.Helper()

	err := filepath.Walk(from, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, rel)

		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, body, info.Mode())
	})
	if err != nil {
		t.Fatalf("copy plugin: %v", err)
	}
}

func writeManifestVersion(t *testing.T, root, v string) {
	t.Helper()

	path := filepath.Join(root, "runtime-manifest.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	var manifest map[string]any
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	manifest["runtimeVersion"] = v

	updated, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(path, updated, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}
