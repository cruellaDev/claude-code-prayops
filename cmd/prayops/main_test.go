package main

import (
	"bytes"
	"encoding/json"
	"runtime"
	"strings"
	"testing"
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

// A hook runs on every Claude Code lifecycle event. It must consume its stdin,
// print nothing, and succeed - even though this build has no adapter yet.
func TestHookIsSilentAndSucceeds(t *testing.T) {
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
