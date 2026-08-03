package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cruellaDev/claude-code-prayops/contracts"
	"github.com/cruellaDev/claude-code-prayops/internal/spool"
)

func exec(t *testing.T, stdin string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = run(args, strings.NewReader(stdin), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestVersionJSON(t *testing.T) {
	code, stdout, stderr := exec(t, "", "version", "--json")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}

	var got struct {
		Version string `json:"version"`
		OS      string `json:"os"`
		Arch    string `json:"arch"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not JSON (%v): %q", err, stdout)
	}
	if got.Version != version {
		t.Fatalf("version = %q, want %q", got.Version, version)
	}
	if got.OS != runtime.GOOS || got.Arch != runtime.GOARCH {
		t.Fatalf("platform = %s/%s, want %s/%s", got.OS, got.Arch, runtime.GOOS, runtime.GOARCH)
	}
}

func TestVersionPlain(t *testing.T) {
	code, stdout, stderr := exec(t, "", "version")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if strings.TrimSpace(stdout) != "prayops "+version {
		t.Fatalf("stdout = %q", stdout)
	}
}

func TestDoctorBootstrapSmoke(t *testing.T) {
	code, stdout, stderr := exec(t, "", "doctor", "--bootstrap-smoke")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if strings.TrimSpace(stdout) != "ok" {
		t.Fatalf("stdout = %q, want \"ok\"", stdout)
	}
}

func TestDoctorAcceptsBootstrapPaths(t *testing.T) {
	code, _, stderr := exec(t, "", "doctor", "--plugin-root", "/tmp/root", "--plugin-data", "/tmp/data")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
}

// A hook runs on every Claude Code lifecycle event. With no plugin data
// directory there is nowhere to record anything, and it still must print
// nothing and succeed rather than interrupt the session.
func TestHookIsSilentAndSucceeds(t *testing.T) {
	t.Setenv("CLAUDE_PLUGIN_DATA", "")
	payload := `{"session_id":"s1","hook_event_name":"PreToolUse","tool_input":{"command":"rm -rf /"}}`

	code, stdout, stderr := exec(t, payload, "hook", "claude")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if stdout != "" {
		t.Fatalf("hook wrote to stdout: %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("hook wrote to stderr: %q", stderr)
	}
}

func TestHookRecordsEvent(t *testing.T) {
	data := t.TempDir()
	t.Setenv("CLAUDE_PLUGIN_DATA", data)

	payload := `{"session_id":"s1","hook_event_name":"PreToolUse","cwd":"/Users/someone/payment-api",
	  "tool_name":"Bash","tool_input":{"command":"psql -c 'select * from customers'"},
	  "transcript_path":"/Users/someone/.claude/transcript.jsonl"}`

	code, stdout, stderr := exec(t, payload, "hook", "claude")
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}

	events, err := spool.New(filepath.Join(data, "state")).Read(0)
	if err != nil {
		t.Fatalf("read spool: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("spool holds %d events, want 1", len(events))
	}
	if events[0].Type != contracts.EventToolStarted || events[0].SessionID != "s1" {
		t.Fatalf("unexpected event: %+v", events[0])
	}
	if events[0].Attributes["toolCategory"] != "execute" {
		t.Fatalf("tool category = %q", events[0].Attributes["toolCategory"])
	}
}

// Whatever goes wrong, the hook stays silent and successful; the reason is
// left where doctor can find it.
func TestHookFailsQuietly(t *testing.T) {
	data := t.TempDir()
	t.Setenv("CLAUDE_PLUGIN_DATA", data)

	code, stdout, stderr := exec(t, `{"hook_event_name":"Notification"}`, "hook", "claude")
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}

	recorded, err := os.ReadFile(filepath.Join(data, "state", "hook-last-error.txt"))
	if err != nil {
		t.Fatalf("no failure was recorded: %v", err)
	}
	if !strings.Contains(string(recorded), "Notification") {
		t.Fatalf("unhelpful record: %q", recorded)
	}

	events, err := spool.New(filepath.Join(data, "state")).Read(0)
	if err != nil {
		t.Fatalf("read spool: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("an unhandled event was still recorded: %+v", events)
	}
}

func TestUnimplementedAndUnknownCommandsFail(t *testing.T) {
	for _, args := range [][]string{
		{},
		{"watch"},
		{"pray"},
		{"statusline", "claude"},
		{"setup"},
		{"alias"},
		{"nonsense"},
	} {
		name := "none"
		if len(args) > 0 {
			name = args[0]
		}
		t.Run(name, func(t *testing.T) {
			code, stdout, stderr := exec(t, "", args...)
			if code != 2 {
				t.Fatalf("exit = %d, want 2", code)
			}
			if stdout != "" {
				t.Fatalf("wrote to stdout: %q", stdout)
			}
			if stderr == "" {
				t.Fatal("expected an explanation on stderr")
			}
		})
	}
}
