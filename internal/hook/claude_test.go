package hook

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/cruellaDev/claude-code-prayops/contracts"
)

func testAdapter() *Adapter {
	return &Adapter{
		Host:  contracts.HostClaude,
		Now:   func() time.Time { return time.Unix(1700000000, 0).UTC() },
		NewID: func() string { return "fixed-id" },
	}
}

func adapt(t *testing.T, raw string) contracts.RitualEvent {
	t.Helper()
	event, err := testAdapter().Adapt(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("adapt: %v", err)
	}
	return event
}

func TestLifecycleMapping(t *testing.T) {
	cases := map[string]contracts.EventType{
		"SessionStart":       contracts.EventSessionStarted,
		"UserPromptSubmit":   contracts.EventPromptSubmitted,
		"PreToolUse":         contracts.EventToolStarted,
		"PostToolUse":        contracts.EventToolFinished,
		"PostToolUseFailure": contracts.EventToolFailed,
		"PermissionRequest":  contracts.EventApprovalRequired,
		"Stop":               contracts.EventTurnCompleted,
		"StopFailure":        contracts.EventTurnFailed,
		"SessionEnd":         contracts.EventSessionEnded,
	}

	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			got := adapt(t, `{"session_id":"s1","hook_event_name":"`+name+`"}`)
			if got.Type != want {
				t.Fatalf("type = %q, want %q", got.Type, want)
			}
			if got.Host != contracts.HostClaude || got.SessionID != "s1" {
				t.Fatalf("unexpected event: %+v", got)
			}
		})
	}
}

func TestUnknownEventIsRefused(t *testing.T) {
	_, err := testAdapter().Adapt(strings.NewReader(`{"session_id":"s1","hook_event_name":"Notification"}`))
	if err == nil {
		t.Fatal("adapter accepted an unhandled event")
	}
}

func TestCwdBecomesHashAndBasename(t *testing.T) {
	got := adapt(t, `{"session_id":"s1","hook_event_name":"Stop","cwd":"/Users/someone/secret-clients/payment-api"}`)

	if got.ProjectName != "payment-api" {
		t.Fatalf("project name = %q", got.ProjectName)
	}
	if got.ProjectKey == "" || len(got.ProjectKey) != 16 {
		t.Fatalf("project key = %q, want a 16 character hash", got.ProjectKey)
	}
	if strings.Contains(got.ProjectKey, "secret-clients") {
		t.Fatal("the working directory leaked into the project key")
	}

	// The hash has to be stable, or every session would look like a new
	// project to the watcher.
	again := adapt(t, `{"session_id":"s2","hook_event_name":"Stop","cwd":"/Users/someone/secret-clients/payment-api/"}`)
	if again.ProjectKey != got.ProjectKey {
		t.Fatalf("project key is not stable: %q vs %q", again.ProjectKey, got.ProjectKey)
	}
}

func TestToolNameIsReducedToACategory(t *testing.T) {
	cases := map[string]string{
		"Read":                          "read",
		"Grep":                          "read",
		"Edit":                          "edit",
		"Write":                         "edit",
		"Bash":                          "execute",
		"WebFetch":                      "web",
		"Task":                          "agent",
		"TodoWrite":                     "plan",
		"mcp__internal-billing__charge": "mcp",
		"SomeFutureTool":                "other",
	}

	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			raw, err := json.Marshal(map[string]string{
				"session_id":      "s1",
				"hook_event_name": "PreToolUse",
				"tool_name":       name,
			})
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			got := adapt(t, string(raw))
			if got.Attributes["toolCategory"] != want {
				t.Fatalf("category = %q, want %q", got.Attributes["toolCategory"], want)
			}
			for _, value := range got.Attributes {
				if strings.Contains(value, name) && want != "other" {
					t.Fatalf("the raw tool name %q was stored", name)
				}
			}
		})
	}
}

// FR-063 / HOK-03: the payload fields Claude Code sends that must never be
// read, using the field names from the real hook contract.
func TestSensitivePayloadFieldsAreNeverStored(t *testing.T) {
	raw := `{
	  "session_id": "s1",
	  "hook_event_name": "PreToolUse",
	  "cwd": "/Users/someone/payment-api",
	  "transcript_path": "/Users/someone/.claude/projects/payment-api/transcript.jsonl",
	  "prompt": "rewrite the billing reconciliation to skip the audit log",
	  "tool_name": "Bash",
	  "tool_input": {"command": "psql -c 'select * from customers'"},
	  "tool_response": {"stdout": "4111111111111111"},
	  "error_details": "panic: nil pointer at billing.go:42",
	  "last_assistant_message": "I have removed the audit log write.",
	  "permission_mode": "acceptEdits",
	  "stop_hook_active": true
	}`

	got := adapt(t, raw)

	serialized, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	for _, secret := range []string{
		"rewrite the billing", "psql", "4111111111111111", "nil pointer",
		"audit log", "transcript", ".jsonl", "acceptEdits",
		"/Users/someone/payment-api",
	} {
		if strings.Contains(string(serialized), secret) {
			t.Fatalf("event leaked %q:\n%s", secret, serialized)
		}
	}

	// What it does keep.
	if got.SessionID != "s1" || got.Type != contracts.EventToolStarted ||
		got.Attributes["toolCategory"] != "execute" || got.ProjectName != "payment-api" {
		t.Fatalf("adapter dropped something it should keep: %+v", got)
	}
}

func TestPayloadWithoutSessionIsRefused(t *testing.T) {
	if _, err := testAdapter().Adapt(strings.NewReader(`{"hook_event_name":"Stop"}`)); err == nil {
		t.Fatal("adapter accepted a payload with no session_id")
	}
}

func TestMalformedPayloadIsRefused(t *testing.T) {
	for name, raw := range map[string]string{
		"empty":       ``,
		"not json":    `{"session_id"`,
		"json scalar": `"just a string"`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := testAdapter().Adapt(strings.NewReader(raw)); err == nil {
				t.Fatal("adapter accepted a malformed payload")
			}
		})
	}
}

// A payload larger than MaxInput must not be buffered whole.
func TestOversizedPayloadIsBounded(t *testing.T) {
	huge := `{"session_id":"s1","hook_event_name":"Stop","cwd":"` + strings.Repeat("a", 2*MaxInput) + `"}`

	if _, err := testAdapter().Adapt(strings.NewReader(huge)); err == nil {
		t.Fatal("adapter accepted a payload past the size limit")
	}
}

func TestEventIDsAreUnique(t *testing.T) {
	adapter := NewAdapter(contracts.HostClaude)
	seen := map[string]bool{}

	for i := 0; i < 100; i++ {
		got, err := adapter.Adapt(strings.NewReader(`{"session_id":"s1","hook_event_name":"Stop"}`))
		if err != nil {
			t.Fatalf("adapt: %v", err)
		}
		if seen[got.ID] {
			t.Fatalf("duplicate event ID %q", got.ID)
		}
		seen[got.ID] = true
	}
}

// NFR-001 budgets the whole hook at 100ms.
func BenchmarkAdapt(b *testing.B) {
	adapter := NewAdapter(contracts.HostClaude)
	raw := `{"session_id":"s1","hook_event_name":"PreToolUse","cwd":"/Users/someone/payment-api","tool_name":"Bash","tool_input":{"command":"go test ./..."}}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := adapter.Adapt(strings.NewReader(raw)); err != nil {
			b.Fatalf("adapt: %v", err)
		}
	}
}
