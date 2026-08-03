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

// Doctor reports a table and exits non-zero when something needs attention,
// so a caller can tell a healthy install from a broken one without parsing.
func TestDoctorReportsProblems(t *testing.T) {
	code, stdout, _ := exec(t, "", "doctor", "--plugin-root", t.TempDir(), "--plugin-data", t.TempDir())
	if code != 1 {
		t.Fatalf("exit = %d, want 1 for a missing runtime\n%s", code, stdout)
	}
	for _, want := range []string{"Plugin", "Runtime", "not installed", "/prayops:setup", "Terminal"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("report does not mention %q:\n%s", want, stdout)
		}
	}
}

func TestDoctorJSON(t *testing.T) {
	_, stdout, _ := exec(t, "", "doctor", "--json", "--plugin-root", t.TempDir(), "--plugin-data", t.TempDir())

	var report struct {
		Checks []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("stdout is not JSON (%v): %q", err, stdout)
	}
	if len(report.Checks) < 5 {
		t.Fatalf("only %d checks", len(report.Checks))
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

// The status line runs on Claude Code's refresh interval, so it reads one
// session file and nothing else.
func TestStatuslineRendersItsOwnSession(t *testing.T) {
	data := t.TempDir()
	t.Setenv("CLAUDE_PLUGIN_DATA", data)
	t.Setenv("NO_COLOR", "1")

	// Two sessions are live; the status line must show the one that asked.
	for _, payload := range []string{
		`{"session_id":"mine","hook_event_name":"UserPromptSubmit"}`,
		`{"session_id":"theirs","hook_event_name":"SessionEnd"}`,
	} {
		if code, _, _ := exec(t, payload, "hook", "claude"); code != 0 {
			t.Fatalf("hook exit = %d", code)
		}
	}

	code, stdout, stderr := exec(t, `{"session_id":"mine","model":{"display_name":"Opus"}}`, "statusline", "claude")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "THINKING") {
		t.Fatalf("status line = %q, want the prompt-submitted phase", stdout)
	}
	if strings.Contains(stdout, "ENDED") {
		t.Fatalf("status line showed another session: %q", stdout)
	}
	if strings.Count(stdout, "\n") != 1 {
		t.Fatalf("status line is not exactly one line: %q", stdout)
	}
}

// Before the first hook fires there is no session file, and the status line
// still has to print something harmless rather than an error.
func TestStatuslineWithoutStateIsQuiet(t *testing.T) {
	t.Setenv("CLAUDE_PLUGIN_DATA", t.TempDir())
	t.Setenv("NO_COLOR", "1")

	code, stdout, stderr := exec(t, `{"session_id":"unknown"}`, "statusline", "claude")
	if code != 0 || stderr != "" {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "IDLE") {
		t.Fatalf("status line = %q", stdout)
	}
}

func TestStatuslineWithoutPluginDataPrintsNothing(t *testing.T) {
	t.Setenv("CLAUDE_PLUGIN_DATA", "")

	code, stdout, stderr := exec(t, `{"session_id":"mine"}`, "statusline", "claude")
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
}

// FR-040: the alias is optional, so a name someone else already uses is a
// refusal rather than a takeover.
func TestAliasRefusesToTakeAnExistingPray(t *testing.T) {
	base := t.TempDir()
	t.Setenv("CLAUDE_PLUGIN_DATA", filepath.Join(base, "data"))
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(base, "config"))

	path := filepath.Join(base, "config", "skills", "pray", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	const theirs = "---\nname: pray\n---\n\nmine\n"
	if err := os.WriteFile(path, []byte(theirs), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	code, _, stderr := exec(t, "", "alias", "install", "--yes")
	if code != 3 {
		t.Fatalf("exit = %d, want 3", code)
	}
	if !strings.Contains(stderr, "left untouched") {
		t.Fatalf("stderr = %q", stderr)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(body) != theirs {
		t.Fatalf("their file changed:\n%s", body)
	}
}

func TestAliasInstallNeedsConsent(t *testing.T) {
	base := t.TempDir()
	t.Setenv("CLAUDE_PLUGIN_DATA", filepath.Join(base, "data"))
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(base, "config"))

	code, stdout, _ := exec(t, "", "alias", "install")
	if code != 10 {
		t.Fatalf("exit = %d, want 10\n%s", code, stdout)
	}
	if _, err := os.Stat(filepath.Join(base, "config", "skills", "pray", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("the alias was installed without consent")
	}
}

func TestUnimplementedAndUnknownCommandsFail(t *testing.T) {
	for _, args := range [][]string{
		{},
		{"watch"},
		{"pray"},
		{"setup"},
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
