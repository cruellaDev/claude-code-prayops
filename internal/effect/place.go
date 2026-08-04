package effect

import (
	"sort"

	"github.com/cruellaDev/claude-code-prayops/contracts"
	"github.com/cruellaDev/claude-code-prayops/internal/layout"
)

// maxAttempts is how many jittered positions are tried before falling back to
// the unjittered anchor.
const maxAttempts = 16

// Place chooses where a prayer of the given size appears.
//
// The result is a pure function of the seed, the layout, and the size, so the
// same prayer lands in the same place every time it is replayed - and, more
// importantly, lands somewhere different for the next one.
//
// avoid lists rectangles the effect must not overlap, such as an effect that
// is still on screen.
func Place(seed uint64, l contracts.Layout, width, height int, avoid []contracts.Rect) (contracts.Placement, bool) {
	zones := layout.SafeZones(l, width, height)
	if len(zones) == 0 {
		return contracts.Placement{}, false
	}

	r := newRNG(seed)

	// Zones are tried in weighted order rather than picking one and jittering
	// inside it: jitter spans a few cells, so if the chosen zone is already
	// occupied no amount of retrying escapes it.
	order := zoneOrder(r, zones)

	anchorOf := func(area contracts.Rect) contracts.Rect {
		return contracts.Rect{
			X:      area.X + (area.Width-width)/2,
			Y:      area.Y + (area.Height-height)/2,
			Width:  width,
			Height: height,
		}
	}

	for _, zone := range order {
		area := zones[zone]
		anchor := anchorOf(area)

		for attempt := 0; attempt < maxAttempts; attempt++ {
			candidate := contracts.Rect{
				X:      anchor.X + r.intN(5) - 2,
				Y:      anchor.Y + r.intN(3) - 1,
				Width:  width,
				Height: height,
			}
			clampInto(&candidate, area)

			if !collides(candidate, avoid) {
				return placement(zone, candidate, area), true
			}
		}
	}

	// Deterministic fallback: the first zone's unjittered anchor. It may still
	// overlap, but a prayer that appears in a predictable place beats one that
	// does not appear at all.
	zone := order[0]
	area := zones[zone]
	anchor := anchorOf(area)
	clampInto(&anchor, area)
	return placement(zone, anchor, area), true
}

// zoneOrder returns every usable zone, the weighted pick first and the rest
// behind it in a stable order.
func zoneOrder(r *rng, zones map[contracts.Zone]contracts.Rect) []contracts.Zone {
	first, _ := pickZone(r, zones)

	order := []contracts.Zone{first}
	rest := make([]contracts.Zone, 0, len(zones))
	for zone := range zones {
		if zone != first {
			rest = append(rest, zone)
		}
	}
	sort.Slice(rest, func(i, j int) bool {
		// Heavier zones first, then by name so the order never depends on map
		// iteration.
		if contracts.ZoneWeights[rest[i]] != contracts.ZoneWeights[rest[j]] {
			return contracts.ZoneWeights[rest[i]] > contracts.ZoneWeights[rest[j]]
		}
		return rest[i] < rest[j]
	})
	return append(order, rest...)
}

// pickZone selects a zone by weight, renormalised over the zones that can
// actually hold the effect.
func pickZone(r *rng, zones map[contracts.Zone]contracts.Rect) (contracts.Zone, contracts.Rect) {
	// Map iteration order is random in Go, which would make placement
	// non-reproducible. Sorting first is what keeps the seed meaningful.
	names := make([]contracts.Zone, 0, len(zones))
	total := 0
	for zone := range zones {
		names = append(names, zone)
		total += contracts.ZoneWeights[zone]
	}
	sort.Slice(names, func(i, j int) bool { return names[i] < names[j] })

	if total <= 0 {
		zone := names[0]
		return zone, zones[zone]
	}

	roll := r.intN(total)
	for _, zone := range names {
		roll -= contracts.ZoneWeights[zone]
		if roll < 0 {
			return zone, zones[zone]
		}
	}
	zone := names[len(names)-1]
	return zone, zones[zone]
}

func clampInto(rect *contracts.Rect, area contracts.Rect) {
	if rect.X < area.X {
		rect.X = area.X
	}
	if rect.Y < area.Y {
		rect.Y = area.Y
	}
	if right := area.X + area.Width; rect.X+rect.Width > right {
		rect.X = right - rect.Width
	}
	if bottom := area.Y + area.Height; rect.Y+rect.Height > bottom {
		rect.Y = bottom - rect.Height
	}
}

func collides(rect contracts.Rect, avoid []contracts.Rect) bool {
	for _, other := range avoid {
		if rect.Intersects(other) {
			return true
		}
	}
	return false
}

// placement records the position as a normalised offset inside its zone, so a
// resize can restore the same relative spot without re-rolling the seed.
func placement(zone contracts.Zone, rect, area contracts.Rect) contracts.Placement {
	p := contracts.Placement{Zone: zone, Rect: rect}

	if span := area.Width - rect.Width; span > 0 {
		p.OffsetX = float64(rect.X-area.X) / float64(span)
	}
	if span := area.Height - rect.Height; span > 0 {
		p.OffsetY = float64(rect.Y-area.Y) / float64(span)
	}
	return p
}

// Reposition maps a placement onto a new layout after a resize.
//
// The zone and the normalised offset are kept, so the effect stays where the
// user saw it rather than jumping somewhere new. Nothing is re-rolled.
func Reposition(p contracts.Placement, l contracts.Layout, width, height int) (contracts.Placement, bool) {
	zones := layout.SafeZones(l, width, height)

	area, ok := zones[p.Zone]
	if !ok {
		// The zone no longer fits. Any remaining zone is better than dropping
		// the effect, and it is chosen deterministically rather than at random.
		names := make([]contracts.Zone, 0, len(zones))
		for zone := range zones {
			names = append(names, zone)
		}
		if len(names) == 0 {
			return contracts.Placement{}, false
		}
		sort.Slice(names, func(i, j int) bool { return names[i] < names[j] })
		p.Zone = names[0]
		area = zones[p.Zone]
	}

	rect := contracts.Rect{
		X:      area.X + int(p.OffsetX*float64(area.Width-width)+0.5),
		Y:      area.Y + int(p.OffsetY*float64(area.Height-height)+0.5),
		Width:  width,
		Height: height,
	}
	clampInto(&rect, area)

	p.Rect = rect
	return p, true
}
