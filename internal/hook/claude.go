// Package hook turns Claude Code lifecycle hook payloads into ritual events.
//
// The adapter is an allowlist, not a filter: it names the handful of fields it
// copies and ignores everything else in the payload. Prompts, code, tool
// arguments, tool output, transcript paths, and error details are never read,
// so they cannot be forwarded by accident when Claude Code adds a field.
package hook

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/cruellaDev/claude-code-prayops/contracts"
	"github.com/cruellaDev/claude-code-prayops/internal/spool"
)

// MaxInput bounds how much of stdin is read. Real payloads are a few hundred
// bytes; anything near this is not an event worth persisting.
const MaxInput = 1 << 20

// payload is the allowlisted view of a Claude Code hook payload. Adding a
// field here is a privacy decision, so the struct is deliberately small.
type payload struct {
	SessionID string `json:"session_id"`
	EventName string `json:"hook_event_name"`
	CWD       string `json:"cwd"`
	ToolName  string `json:"tool_name"`
}

// eventTypes maps Claude Code hook events to ritual events.
var eventTypes = map[string]contracts.EventType{
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

// toolCategories coarsens tool names into the only tool information that is
// ever stored. The raw name is dropped, which also keeps MCP server names -
// which can identify internal systems - out of the event.
var toolCategories = map[string]string{
	"Read":         "read",
	"Glob":         "read",
	"Grep":         "read",
	"NotebookRead": "read",
	"Edit":         "edit",
	"Write":        "edit",
	"NotebookEdit": "edit",
	"Bash":         "execute",
	"BashOutput":   "execute",
	"KillShell":    "execute",
	"WebFetch":     "web",
	"WebSearch":    "web",
	"Task":         "agent",
	"Agent":        "agent",
	"TodoWrite":    "plan",
}

// ToolCategory returns the stored category for a tool name.
func ToolCategory(name string) string {
	if name == "" {
		return ""
	}
	if strings.HasPrefix(name, "mcp__") {
		return "mcp"
	}
	if category, ok := toolCategories[name]; ok {
		return category
	}
	return "other"
}

// ProjectKey is the stable identifier for a working directory. Only the hash
// is stored; the path itself never leaves the process.
func ProjectKey(cwd string) string {
	if cwd == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(filepath.Clean(cwd)))
	return hex.EncodeToString(sum[:])[:16]
}

// Adapter converts payloads into events.
type Adapter struct {
	Host  contracts.Host
	Now   func() time.Time
	NewID func() string
}

// NewAdapter returns an adapter for the given host.
func NewAdapter(host contracts.Host) *Adapter {
	return &Adapter{Host: host, Now: time.Now, NewID: randomID}
}

func randomID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// crypto/rand does not fail in practice; a timestamp still gives the
		// spool something unique enough to deduplicate on.
		return fmt.Sprintf("t%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf[:])
}

// Adapt reads one hook payload and returns the event to record.
func (a *Adapter) Adapt(r io.Reader) (contracts.RitualEvent, error) {
	var event contracts.RitualEvent

	raw, err := io.ReadAll(io.LimitReader(r, MaxInput))
	if err != nil {
		return event, fmt.Errorf("hook: read payload: %w", err)
	}
	if len(raw) == 0 {
		return event, fmt.Errorf("hook: empty payload")
	}

	var in payload
	if err := json.Unmarshal(raw, &in); err != nil {
		return event, fmt.Errorf("hook: parse payload: %w", err)
	}

	eventType, ok := eventTypes[in.EventName]
	if !ok {
		return event, fmt.Errorf("hook: unhandled event %q", in.EventName)
	}
	if in.SessionID == "" {
		return event, fmt.Errorf("hook: payload has no session_id")
	}

	event = contracts.RitualEvent{
		SchemaVersion: spool.SchemaVersion,
		ID:            a.NewID(),
		Host:          a.Host,
		SessionID:     in.SessionID,
		ProjectKey:    ProjectKey(in.CWD),
		Type:          eventType,
		OccurredAt:    a.Now().UTC(),
	}
	if in.CWD != "" {
		event.ProjectName = filepath.Base(filepath.Clean(in.CWD))
	}
	if category := ToolCategory(in.ToolName); category != "" {
		event.Attributes = map[string]string{"toolCategory": category}
	}

	return event, nil
}
