package contracts

// LayoutMode is chosen from the terminal size available to the watcher.
type LayoutMode string

const (
	LayoutFull    LayoutMode = "FULL"
	LayoutMedium  LayoutMode = "MEDIUM"
	LayoutCompact LayoutMode = "COMPACT"
)

// Layout is the resolved altar geometry for one frame size. Prayer placement
// anchors on Burner and Incense and must stay inside Content.
type Layout struct {
	Mode    LayoutMode `json:"mode"`
	Width   int        `json:"width"`
	Height  int        `json:"height"`
	Content Rect       `json:"content"`
	Altar   Rect       `json:"altar"`
	Burner  Rect       `json:"burner"`
	Incense Rect       `json:"incense"`
	Plates  Rect       `json:"plates"`
	Status  Rect       `json:"status"`
}

// Placement is the resolved position of one prayer effect. Zone and Offset are
// retained across resize so the effect is never re-randomized.
type Placement struct {
	Zone    Zone    `json:"zone"`
	Rect    Rect    `json:"rect"`
	OffsetX float64 `json:"offsetX"`
	OffsetY float64 `json:"offsetY"`
}

// ZoneWeights is the selection weight per safe zone. Only valid zones are kept
// and the remaining weights are renormalized.
var ZoneWeights = map[Zone]int{
	ZoneRight:      35,
	ZoneLeft:       30,
	ZoneUpperRight: 20,
	ZoneUpperLeft:  15,
}
