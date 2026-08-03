// Package privacy holds the end-to-end privacy fixtures.
//
// The other packages each check their own boundary. This one drives the real
// commands with hostile payloads and then reads every byte PrayOps left on
// disk, because the promise is about what is stored, not about what any one
// function returns.
package privacy

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// secrets are strings planted in the payloads. None may appear anywhere under
// the plugin data directory afterwards.
var secrets = []string{
	// prompt
	"remove the audit log from the reconciliation job",
	// tool_input
	"psql -c 'select card_number from customers'",
	// tool_response
	"4111111111111111",
	// error_details
	"panic: nil pointer dereference at billing.go:42",
	// last_assistant_message
	"I have deleted the audit log writes as requested",
	// transcript_path
	"transcript-e7d1.jsonl",
	// the working directory itself
	"acme-confidential",
	// a raw tool name, which can name an internal system through MCP
	"internal-billing-ledger",
}

// payloads are the Claude Code hook events, each carrying every secret field
// the contract forbids storing.
func payloads() []string {
	return []string{
		`{"session_id":"s1","hook_event_name":"SessionStart","source":"startup",
		  "cwd":"/Users/someone/acme-confidential/payments",
		  "transcript_path":"/Users/someone/.claude/transcript-e7d1.jsonl"}`,

		`{"session_id":"s1","hook_event_name":"UserPromptSubmit",
		  "cwd":"/Users/someone/acme-confidential/payments",
		  "prompt":"remove the audit log from the reconciliation job",
		  "transcript_path":"/Users/someone/.claude/transcript-e7d1.jsonl"}`,

		`{"session_id":"s1","hook_event_name":"PreToolUse",
		  "cwd":"/Users/someone/acme-confidential/payments",
		  "tool_name":"mcp__internal-billing-ledger__query",
		  "tool_input":{"command":"psql -c 'select card_number from customers'"}}`,

		`{"session_id":"s1","hook_event_name":"PostToolUse",
		  "cwd":"/Users/someone/acme-confidential/payments",
		  "tool_name":"Bash",
		  "tool_input":{"command":"psql -c 'select card_number from customers'"},
		  "tool_response":{"stdout":"4111111111111111"}}`,

		`{"session_id":"s1","hook_event_name":"PostToolUseFailure",
		  "cwd":"/Users/someone/acme-confidential/payments","tool_name":"Bash",
		  "error_details":"panic: nil pointer dereference at billing.go:42"}`,

		`{"session_id":"s1","hook_event_name":"PermissionRequest",
		  "cwd":"/Users/someone/acme-confidential/payments","tool_name":"Write",
		  "tool_input":{"file_path":"/etc/passwd"},"permission_mode":"acceptEdits"}`,

		`{"session_id":"s1","hook_event_name":"Stop",
		  "cwd":"/Users/someone/acme-confidential/payments",
		  "last_assistant_message":"I have deleted the audit log writes as requested",
		  "stop_hook_active":true}`,

		`{"session_id":"s1","hook_event_name":"StopFailure",
		  "cwd":"/Users/someone/acme-confidential/payments",
		  "error_details":"panic: nil pointer dereference at billing.go:42"}`,

		`{"session_id":"s1","hook_event_name":"SessionEnd",
		  "cwd":"/Users/someone/acme-confidential/payments","reason":"clear"}`,
	}
}

// build compiles the real binary, so the fixtures exercise what ships.
func build(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "prayops")
	cmd := exec.Command("go", "build", "-o", binary, "./cmd/prayops")
	cmd.Dir = "../.."

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return binary
}

func run(t *testing.T, binary, data, stdin string, args ...string) {
	t.Helper()

	cmd := exec.Command(binary, args...)
	cmd.Env = append(os.Environ(), "CLAUDE_PLUGIN_DATA="+data)
	cmd.Stdin = strings.NewReader(stdin)

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out)
	}
}

// readAll returns every file under a directory, keyed by path.
func readAll(t *testing.T, root string) map[string]string {
	t.Helper()

	files := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[strings.TrimPrefix(path, root)] = string(raw)
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return files
}

// FR-063 / HOK-03: after a full session of hostile payloads, nothing PrayOps
// wrote may contain a prompt, a command, tool output, an error message, a
// transcript path, an assistant message, or the working directory.
func TestNothingSensitiveReachesDisk(t *testing.T) {
	binary := build(t)
	data := t.TempDir()

	for _, payload := range payloads() {
		run(t, binary, data, payload, "hook", "claude")
	}
	// A prayer as well, since it is the one command that takes user text.
	run(t, binary, data, "", "pray",
		"--cwd", "/Users/someone/acme-confidential/payments",
		"--text", "무사배포")

	files := readAll(t, data)
	if len(files) == 0 {
		t.Fatal("the run wrote nothing at all, so this fixture proves nothing")
	}

	for path, body := range files {
		lowered := strings.ToLower(body)
		for _, secret := range secrets {
			if strings.Contains(lowered, strings.ToLower(secret)) {
				t.Errorf("%s contains %q:\n%s", path, secret, body)
			}
		}
	}
}

// The fixture above is only meaningful if the events were actually recorded.
func TestTheSessionWasRecorded(t *testing.T) {
	binary := build(t)
	data := t.TempDir()

	for _, payload := range payloads() {
		run(t, binary, data, payload, "hook", "claude")
	}

	files := readAll(t, data)

	var events, sessions int
	for path := range files {
		switch {
		case strings.Contains(path, "events/inbox"):
			events++
		case strings.Contains(path, "sessions/"):
			sessions++
		}
	}
	if events != len(payloads()) {
		t.Fatalf("%d events recorded, want %d", events, len(payloads()))
	}
	if sessions != 1 {
		t.Fatalf("%d session files, want 1", sessions)
	}

	// And the harmless facts must be there, or the adapter dropped everything.
	all := strings.Join(values(files), "\n")
	for _, want := range []string{"TOOL_STARTED", "SESSION_ENDED", "toolCategory", "payments"} {
		if !strings.Contains(all, want) {
			t.Fatalf("the recorded state is missing %q", want)
		}
	}
}

// The working directory is stored as a hash, and the same directory must
// always produce the same one - otherwise every session looks like a new
// project.
func TestProjectKeyIsAStableHash(t *testing.T) {
	binary := build(t)

	keys := map[string]bool{}
	for i := 0; i < 3; i++ {
		data := t.TempDir()
		run(t, binary, data, payloads()[0], "hook", "claude")

		for path, body := range readAll(t, data) {
			if !strings.Contains(path, "events/inbox") {
				continue
			}
			key := between(body, `"projectKey":"`, `"`)
			if key == "" {
				t.Fatalf("no project key in %s", body)
			}
			keys[key] = true
		}
	}
	if len(keys) != 1 {
		t.Fatalf("the same directory produced %d different keys: %v", len(keys), keys)
	}
}

// No network. The runtime only reaches out during installation, which is the
// shell script's job, so the compiled binary must not link a HTTP client at
// all.
func TestRuntimeDoesNotLinkAnHTTPClient(t *testing.T) {
	binary := build(t)

	cmd := exec.Command("go", "tool", "nm", binary)
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("go tool nm unavailable: %v", err)
	}

	for _, symbol := range []string{"net/http.(*Client).Do", "net/http.Get"} {
		if strings.Contains(string(out), symbol) {
			t.Fatalf("the runtime links %s; it should never make a request", symbol)
		}
	}
}

func values(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

func between(s, start, end string) string {
	i := strings.Index(s, start)
	if i < 0 {
		return ""
	}
	rest := s[i+len(start):]
	j := strings.Index(rest, end)
	if j < 0 {
		return ""
	}
	return rest[:j]
}
