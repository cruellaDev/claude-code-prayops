package contracts

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestRectIntersects(t *testing.T) {
	base := Rect{X: 10, Y: 10, Width: 4, Height: 3}

	cases := []struct {
		name  string
		other Rect
		want  bool
	}{
		{"overlap", Rect{X: 12, Y: 11, Width: 4, Height: 3}, true},
		{"contained", Rect{X: 11, Y: 11, Width: 1, Height: 1}, true},
		{"touch right edge", Rect{X: 14, Y: 10, Width: 2, Height: 3}, false},
		{"touch bottom edge", Rect{X: 10, Y: 13, Width: 4, Height: 2}, false},
		{"disjoint", Rect{X: 40, Y: 40, Width: 2, Height: 2}, false},
		{"zero width", Rect{X: 11, Y: 11, Width: 0, Height: 3}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := base.Intersects(tc.other); got != tc.want {
				t.Fatalf("Intersects(%+v) = %v, want %v", tc.other, got, tc.want)
			}
			if got := tc.other.Intersects(base); got != tc.want {
				t.Fatalf("reverse Intersects(%+v) = %v, want %v", tc.other, got, tc.want)
			}
		})
	}
}

// forbiddenFields are the payloads that must never reach a serialized event.
var forbiddenFields = []string{
	"prompt",
	"command",
	"tool_input",
	"toolInput",
	"tool_response",
	"toolResponse",
	"transcript_path",
	"transcriptPath",
	"error_details",
	"errorDetails",
	"last_assistant_message",
	"lastAssistantMessage",
	"cwd",
}

func TestRitualEventRoundTripCarriesNoPayload(t *testing.T) {
	in := RitualEvent{
		SchemaVersion: 1,
		ID:            "01J000000000000000000000",
		Host:          HostClaude,
		SessionID:     "session-1",
		ProjectKey:    "3f786850e387550fdab836ed7e6dc881de23001b",
		ProjectName:   "payment-api",
		Type:          EventToolStarted,
		OccurredAt:    time.Unix(1700000000, 0).UTC(),
		Attributes:    map[string]string{"toolCategory": "edit"},
	}

	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	lowered := strings.ToLower(string(raw))
	for _, field := range forbiddenFields {
		if strings.Contains(lowered, strings.ToLower(field)) {
			t.Fatalf("serialized event contains forbidden field %q: %s", field, raw)
		}
	}

	var out RitualEvent
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.ID != in.ID {
		t.Fatalf("round trip lost ID: got %q want %q", out.ID, in.ID)
	}
	if !out.OccurredAt.Equal(in.OccurredAt) {
		t.Fatalf("round trip lost OccurredAt: got %v want %v", out.OccurredAt, in.OccurredAt)
	}
	if out.Attributes["toolCategory"] != "edit" {
		t.Fatalf("round trip lost attributes: %+v", out.Attributes)
	}
}

func TestAllowedEventAttributeKeysExcludePayload(t *testing.T) {
	for key := range AllowedEventAttributeKeys {
		lowered := strings.ToLower(key)
		for _, field := range forbiddenFields {
			if strings.Contains(lowered, strings.ToLower(field)) {
				t.Fatalf("allowlist key %q matches forbidden field %q", key, field)
			}
		}
	}
}

func TestSessionStateCarriesNoPayload(t *testing.T) {
	raw, err := json.Marshal(SessionState{
		SchemaVersion: 1,
		Host:          HostClaude,
		SessionID:     "session-1",
		Phase:         PhaseWorking,
		PrayerQueue:   []PrayerRequest{{ID: "p1", Source: PrayerSource{Kind: PrayerSourceText, Text: "무사배포"}}},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	lowered := strings.ToLower(string(raw))
	for _, field := range forbiddenFields {
		if strings.Contains(lowered, strings.ToLower(field)) {
			t.Fatalf("serialized session contains forbidden field %q: %s", field, raw)
		}
	}
}

func TestZoneWeightsCoverEverySafeZone(t *testing.T) {
	zones := []Zone{ZoneLeft, ZoneRight, ZoneUpperLeft, ZoneUpperRight}

	total := 0
	for _, zone := range zones {
		weight, ok := ZoneWeights[zone]
		if !ok {
			t.Fatalf("zone %q has no weight", zone)
		}
		if weight <= 0 {
			t.Fatalf("zone %q weight must be positive, got %d", zone, weight)
		}
		total += weight
	}

	if total != 100 {
		t.Fatalf("zone weights total = %d, want 100", total)
	}
	if len(ZoneWeights) != len(zones) {
		t.Fatalf("ZoneWeights has %d entries, want %d", len(ZoneWeights), len(zones))
	}
}
