// Package bootstrap tests the plugin's runtime installer end to end against a
// local release server. Nothing here reaches the network.
package bootstrap

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
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

const (
	pluginRoot     = "../../plugins/prayops"
	runtimeVersion = "0.1.0" // must match plugins/prayops/runtime-manifest.json
)

// fakeRuntime is a stand-in for the Go binary. The installer only executes the
// two commands it needs to trust a download, so a script is enough.
const fakeRuntime = `#!/bin/sh
case "$1 $2" in
  "version --json") echo '{"version":"%s","os":"x","arch":"y"}' ;;
  "doctor --bootstrap-smoke") echo ok ;;
  *) exit 1 ;;
esac
`

type entry struct {
	name     string
	body     string
	mode     int64
	typeFlag byte
	linkname string
}

func tarGz(t *testing.T, entries []entry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)

	for _, e := range entries {
		typeFlag := e.typeFlag
		if typeFlag == 0 {
			typeFlag = tar.TypeReg
		}
		mode := e.mode
		if mode == 0 {
			mode = 0o755
		}
		header := &tar.Header{
			Name:     e.name,
			Mode:     mode,
			Size:     int64(len(e.body)),
			Typeflag: typeFlag,
			Linkname: e.linkname,
		}
		if typeFlag != tar.TypeReg {
			header.Size = 0
		}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatalf("write header %s: %v", e.name, err)
		}
		if header.Size > 0 {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatalf("write body %s: %v", e.name, err)
			}
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

func assetName() string {
	return fmt.Sprintf("prayops_%s_%s_%s.tar.gz", runtimeVersion, runtime.GOOS, runtime.GOARCH)
}

// release serves one archive plus a checksums.txt. When corruptChecksum is set
// the advertised hash does not match the archive.
func release(t *testing.T, archive []byte, corruptChecksum bool) string {
	t.Helper()

	sum := sha256.Sum256(archive)
	digest := hex.EncodeToString(sum[:])
	if corruptChecksum {
		digest = strings.Repeat("0", 64)
	}
	checksums := fmt.Sprintf("%s  %s\n", digest, assetName())

	mux := http.NewServeMux()
	mux.HandleFunc("/"+assetName(), func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	})
	mux.HandleFunc("/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(checksums))
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server.URL
}

func goodArchive(t *testing.T) []byte {
	t.Helper()
	return tarGz(t, []entry{{name: "prayops", body: fmt.Sprintf(fakeRuntime, runtimeVersion)}})
}

type result struct {
	code   int
	output string
	data   string // CLAUDE_PLUGIN_DATA
}

func setup(t *testing.T, baseURL, data string, args ...string) result {
	t.Helper()

	root, err := filepath.Abs(pluginRoot)
	if err != nil {
		t.Fatalf("resolve plugin root: %v", err)
	}

	cmd := exec.Command(filepath.Join(root, "scripts", "setup.sh"), args...)
	cmd.Env = append(os.Environ(),
		"CLAUDE_PLUGIN_ROOT="+root,
		"CLAUDE_PLUGIN_DATA="+data,
		"PRAYOPS_RELEASE_BASE_URL="+baseURL,
	)

	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("run setup.sh: %v (%s)", err, out)
		}
		code = exitErr.ExitCode()
	}
	return result{code: code, output: string(out), data: data}
}

func (r result) installed() bool {
	info, err := os.Stat(filepath.Join(r.data, "bin", "prayops"))
	return err == nil && info.Mode().Perm()&0o111 != 0
}

func (r result) binaryContents(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(r.data, "bin", "prayops"))
	if err != nil {
		return ""
	}
	return string(raw)
}

// plantRuntime writes a sentinel runtime so failure paths can be checked for
// having left it alone.
func plantRuntime(t *testing.T, data string) string {
	t.Helper()
	const sentinel = "#!/bin/sh\necho existing\n"

	if err := os.MkdirAll(filepath.Join(data, "bin"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(data, "bin", "prayops"), []byte(sentinel), 0o755); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}
	meta := `{"runtimeVersion": "0.0.9", "pluginVersion": "0.0.9"}`
	if err := os.WriteFile(filepath.Join(data, "runtime.json"), []byte(meta), 0o644); err != nil {
		t.Fatalf("write runtime.json: %v", err)
	}
	return sentinel
}

// FR-021: without consent the installer only describes what it would do.
func TestPlanModeInstallsNothing(t *testing.T) {
	got := setup(t, release(t, goodArchive(t), false), t.TempDir())

	if got.code != 10 {
		t.Fatalf("exit = %d, want 10\n%s", got.code, got.output)
	}
	if got.installed() {
		t.Fatal("plan mode installed a runtime")
	}
	for _, needed := range []string{runtimeVersion, runtime.GOOS + "/" + runtime.GOARCH, "SHA-256", assetName(), "--yes"} {
		if !strings.Contains(got.output, needed) {
			t.Fatalf("plan does not mention %q:\n%s", needed, got.output)
		}
	}
}

// FR-024 / FR-025: the happy path verifies, smoke tests, then installs.
func TestInstallsVerifiedRuntime(t *testing.T) {
	data := t.TempDir()
	got := setup(t, release(t, goodArchive(t), false), data, "--yes")

	if got.code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", got.code, got.output)
	}
	if !got.installed() {
		t.Fatalf("no executable runtime was installed\n%s", got.output)
	}
	if !strings.Contains(got.output, "SHA-256 verified") || !strings.Contains(got.output, "Smoke test passed") {
		t.Fatalf("installer did not report verification:\n%s", got.output)
	}

	meta, err := os.ReadFile(filepath.Join(data, "runtime.json"))
	if err != nil {
		t.Fatalf("read runtime.json: %v", err)
	}
	for _, needed := range []string{`"runtimeVersion": "` + runtimeVersion + `"`, `"asset": "` + assetName() + `"`, `"sha256"`, `"installedAt"`} {
		if !strings.Contains(string(meta), needed) {
			t.Fatalf("runtime.json is missing %s:\n%s", needed, meta)
		}
	}

	// The event spool directories the runtime writes into must exist.
	for _, dir := range []string{"state/events/tmp", "state/events/inbox", "state/events/rejected", "state/sessions", "state/watchers"} {
		if _, err := os.Stat(filepath.Join(data, dir)); err != nil {
			t.Fatalf("missing %s: %v", dir, err)
		}
	}
}

func TestSecondRunIsANoOp(t *testing.T) {
	data := t.TempDir()
	url := release(t, goodArchive(t), false)

	if got := setup(t, url, data, "--yes"); got.code != 0 {
		t.Fatalf("first run exit = %d\n%s", got.code, got.output)
	}

	got := setup(t, url, data) // no --yes: an up-to-date runtime needs no consent
	if got.code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", got.code, got.output)
	}
	if !strings.Contains(got.output, "already installed") {
		t.Fatalf("expected an up-to-date message:\n%s", got.output)
	}
}

// FR-024 / acceptance test 5: a bad checksum must not touch the existing
// runtime.
func TestChecksumMismatchPreservesExistingRuntime(t *testing.T) {
	data := t.TempDir()
	sentinel := plantRuntime(t, data)

	got := setup(t, release(t, goodArchive(t), true), data, "--yes")

	if got.code != 13 {
		t.Fatalf("exit = %d, want 13\n%s", got.code, got.output)
	}
	if got.binaryContents(t) != sentinel {
		t.Fatal("the existing runtime was replaced despite a checksum mismatch")
	}
	if !strings.Contains(got.output, "checksum mismatch") {
		t.Fatalf("no checksum mismatch reported:\n%s", got.output)
	}
}

// NFR-003: archive entries are validated before extraction.
func TestMaliciousArchivesAreRejected(t *testing.T) {
	cases := map[string][]entry{
		"parent traversal": {
			{name: "prayops", body: "#!/bin/sh\n"},
			{name: "../escape", body: "owned"},
		},
		"absolute path": {
			{name: "prayops", body: "#!/bin/sh\n"},
			{name: "/etc/cron.d/owned", body: "owned"},
		},
		"symlink": {
			{name: "prayops", body: "#!/bin/sh\n"},
			{name: "link", typeFlag: tar.TypeSymlink, linkname: "/etc/passwd"},
		},
		"unexpected extra file": {
			{name: "prayops", body: "#!/bin/sh\n"},
			{name: "install.sh", body: "owned"},
		},
	}

	for name, entries := range cases {
		t.Run(name, func(t *testing.T) {
			data := t.TempDir()
			sentinel := plantRuntime(t, data)

			got := setup(t, release(t, tarGz(t, entries), false), data, "--yes")

			if got.code != 14 {
				t.Fatalf("exit = %d, want 14\n%s", got.code, got.output)
			}
			if got.binaryContents(t) != sentinel {
				t.Fatal("a rejected archive still replaced the runtime")
			}
			if _, err := os.Stat(filepath.Join(data, "escape")); err == nil {
				t.Fatal("archive entry escaped the work directory")
			}
		})
	}
}

// FR-025: a binary that fails its smoke test never becomes the runtime.
func TestSmokeFailurePreservesExistingRuntime(t *testing.T) {
	data := t.TempDir()
	sentinel := plantRuntime(t, data)

	wrongVersion := tarGz(t, []entry{{name: "prayops", body: fmt.Sprintf(fakeRuntime, "9.9.9")}})
	got := setup(t, release(t, wrongVersion, false), data, "--yes")

	if got.code != 15 {
		t.Fatalf("exit = %d, want 15\n%s", got.code, got.output)
	}
	if got.binaryContents(t) != sentinel {
		t.Fatal("a binary that failed its smoke test replaced the runtime")
	}
}

func TestConcurrentSetupIsRefused(t *testing.T) {
	data := t.TempDir()
	if err := os.MkdirAll(filepath.Join(data, "setup.lock"), 0o755); err != nil {
		t.Fatalf("mkdir lock: %v", err)
	}

	got := setup(t, release(t, goodArchive(t), false), data, "--yes")

	if got.code != 17 {
		t.Fatalf("exit = %d, want 17\n%s", got.code, got.output)
	}
	if got.installed() {
		t.Fatal("a locked setup still installed a runtime")
	}
}

// FR-023: an unsupported platform is refused before anything is downloaded.
func TestUnsupportedPlatformIsRefused(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, name := range []string{"scripts/setup.sh", "runtime-manifest.json"} {
		raw, err := os.ReadFile(filepath.Join(pluginRoot, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if name == "runtime-manifest.json" {
			// Keep the shape, drop every target this host could match.
			raw = []byte(strings.ReplaceAll(string(raw), `"`+runtime.GOARCH+`"`, `"sparc"`))
		}
		if err := os.WriteFile(filepath.Join(root, name), raw, 0o755); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	data := t.TempDir()
	cmd := exec.Command(filepath.Join(root, "scripts", "setup.sh"), "--yes")
	cmd.Env = append(os.Environ(),
		"CLAUDE_PLUGIN_ROOT="+root,
		"CLAUDE_PLUGIN_DATA="+data,
		"PRAYOPS_RELEASE_BASE_URL=http://127.0.0.1:1", // must never be contacted
	)

	out, err := cmd.CombinedOutput()
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected a failure, got %v (%s)", err, out)
	}
	if exitErr.ExitCode() != 11 {
		t.Fatalf("exit = %d, want 11\n%s", exitErr.ExitCode(), out)
	}
	if !strings.Contains(string(out), "does not ship a runtime") {
		t.Fatalf("unhelpful message:\n%s", out)
	}
}

// FR-020: the plugin refuses to run outside the Claude Code plugin environment
// rather than guessing where to install.
func TestRefusesWithoutPluginData(t *testing.T) {
	root, err := filepath.Abs(pluginRoot)
	if err != nil {
		t.Fatalf("resolve plugin root: %v", err)
	}

	cmd := exec.Command(filepath.Join(root, "scripts", "setup.sh"), "--yes")
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "CLAUDE_PLUGIN_ROOT=" + root}

	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("setup succeeded without CLAUDE_PLUGIN_DATA:\n%s", out)
	}
	if !strings.Contains(string(out), "CLAUDE_PLUGIN_DATA") {
		t.Fatalf("unhelpful message:\n%s", out)
	}
}

// JSON key order is not guaranteed. Matching the whole {"os":..,"arch":..}
// object as one string made a re-serialised manifest - which puts "arch"
// first - look like an unsupported platform.
func TestSupportedPlatformIgnoresKeyOrder(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	script, err := os.ReadFile(filepath.Join(pluginRoot, "scripts", "setup.sh"))
	if err != nil {
		t.Fatalf("read script: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "setup.sh"), script, 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	// Same manifest, keys the other way round.
	manifest := fmt.Sprintf(`{
  "schemaVersion": 1,
  "runtimeVersion": %q,
  "repository": "cruellaDev/claude-code-prayops",
  "assetTemplate": "prayops_{version}_{os}_{arch}.tar.gz",
  "checksumAsset": "checksums.txt",
  "supported": [
    {"arch": %q, "os": %q}
  ]
}`, runtimeVersion, runtime.GOARCH, runtime.GOOS)
	if err := os.WriteFile(filepath.Join(root, "runtime-manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	data := t.TempDir()
	cmd := exec.Command(filepath.Join(root, "scripts", "setup.sh"), "--yes")
	cmd.Env = append(os.Environ(),
		"CLAUDE_PLUGIN_ROOT="+root,
		"CLAUDE_PLUGIN_DATA="+data,
		"PRAYOPS_RELEASE_BASE_URL="+release(t, goodArchive(t), false),
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install refused a supported platform: %v\n%s", err, out)
	}
	if _, statErr := os.Stat(filepath.Join(data, "bin", "prayops")); statErr != nil {
		t.Fatalf("nothing was installed: %v\n%s", statErr, out)
	}
}
