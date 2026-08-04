package contracts

import "time"

type Host string

const (
	HostClaude Host = "claude"
	HostCodex  Host = "codex"
)

type EventType string

const (
	EventSessionStarted   EventType = "SESSION_STARTED"
	EventPromptSubmitted  EventType = "PROMPT_SUBMITTED"
	EventToolStarted      EventType = "TOOL_STARTED"
	EventToolFinished     EventType = "TOOL_FINISHED"
	EventToolFailed       EventType = "TOOL_FAILED"
	EventApprovalRequired EventType = "APPROVAL_REQUIRED"
	EventTurnCompleted    EventType = "TURN_COMPLETED"
	EventTurnFailed       EventType = "TURN_FAILED"
	EventSessionEnded     EventType = "SESSION_ENDED"
	EventPrayerRequested  EventType = "PRAYER_REQUESTED"
)

type RitualEvent struct {
	SchemaVersion int               `json:"schemaVersion"`
	ID            string            `json:"id"`
	Host          Host              `json:"host"`
	SessionID     string            `json:"sessionId"`
	ProjectKey    string            `json:"projectKey"`
	ProjectName   string            `json:"projectName,omitempty"`
	Type          EventType         `json:"type"`
	OccurredAt    time.Time         `json:"occurredAt"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}
