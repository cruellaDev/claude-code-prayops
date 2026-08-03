package contracts

import "time"

// SessionPhase is the reduced lifecycle state of one host session.
type SessionPhase string

const (
	PhaseIdle             SessionPhase = "IDLE"
	PhaseThinking         SessionPhase = "THINKING"
	PhaseWorking          SessionPhase = "WORKING"
	PhaseApprovalRequired SessionPhase = "APPROVAL_REQUIRED"
	PhaseTurnCompleted    SessionPhase = "TURN_COMPLETED"
	PhaseTurnFailed       SessionPhase = "TURN_FAILED"
	PhaseSessionEnded     SessionPhase = "SESSION_ENDED"
)

// SessionState is the reducer output for one session. It carries no prompt,
// code, tool payload, or transcript data.
type SessionState struct {
	SchemaVersion int          `json:"schemaVersion"`
	Host          Host         `json:"host"`
	SessionID     string       `json:"sessionId"`
	ProjectKey    string       `json:"projectKey"`
	ProjectName   string       `json:"projectName,omitempty"`
	Phase         SessionPhase `json:"phase"`
	LastEventID   string       `json:"lastEventId"`
	UpdatedAt     time.Time    `json:"updatedAt"`

	// TurnStartedAt is when the current turn began. The incense burns down
	// from here, so it must not move on every event within the turn.
	TurnStartedAt time.Time `json:"turnStartedAt,omitempty"`

	// ToolErrored marks that the most recent tool call failed. The reason is
	// never stored, only the fact.
	ToolErrored bool `json:"toolErrored,omitempty"`

	// PrayerQueue holds pending prayer requests. Capacity is enforced by the
	// reducer, not by this type.
	PrayerQueue []PrayerRequest `json:"prayerQueue,omitempty"`
}

// MaxPendingPrayers is the pending queue capacity. One effect is active at a
// time; older pending entries are dropped when the queue is full.
const MaxPendingPrayers = 5
