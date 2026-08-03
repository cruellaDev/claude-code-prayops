package contracts

import "time"

type PrayerSourceKind string

const (
	PrayerSourceImage  PrayerSourceKind = "IMAGE"
	PrayerSourceText   PrayerSourceKind = "TEXT"
	PrayerSourcePreset PrayerSourceKind = "PRESET"
)

type PrayerSource struct {
	Kind    PrayerSourceKind `json:"kind"`
	CacheID string           `json:"cacheId,omitempty"`
	Text    string           `json:"text,omitempty"`
	Preset  string           `json:"preset,omitempty"`
}

type PrayerRequest struct {
	ID        string        `json:"id"`
	SessionID string        `json:"sessionId"`
	Source    PrayerSource  `json:"source"`
	Seed      uint64        `json:"seed"`
	Duration  time.Duration `json:"duration"`
}

type Zone string

const (
	ZoneLeft       Zone = "LEFT"
	ZoneRight      Zone = "RIGHT"
	ZoneUpperLeft  Zone = "UPPER_LEFT"
	ZoneUpperRight Zone = "UPPER_RIGHT"
)

type Rect struct {
	X      int
	Y      int
	Width  int
	Height int
}

// Intersects reports whether two rectangles share at least one cell. An empty
// rectangle covers no cells and therefore never intersects.
func (r Rect) Intersects(other Rect) bool {
	if r.Width <= 0 || r.Height <= 0 || other.Width <= 0 || other.Height <= 0 {
		return false
	}
	return r.X < other.X+other.Width &&
		r.X+r.Width > other.X &&
		r.Y < other.Y+other.Height &&
		r.Y+r.Height > other.Y
}
