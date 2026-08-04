// Package spool is the local event transport between Claude Code hooks and the
// PrayOps watcher. There is no daemon, socket, or database: a hook writes one
// small JSON file atomically, and the watcher consumes it.
package spool

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cruellaDev/claude-code-prayops/contracts"
)

const (
	// SchemaVersion is the event schema this build reads and writes.
	SchemaVersion = 1

	// DefaultBatch is how many events one read consumes.
	DefaultBatch = 64

	// maxInbox bounds the spool when no watcher is running. Hooks fire on
	// every lifecycle event, so an unbounded inbox would grow for as long as
	// Claude Code is used. Oldest events are dropped first.
	maxInbox = 512

	// recentIDs is how many event IDs are remembered for deduplication.
	recentIDs = 256

	// maxEventBytes rejects anything far larger than a real event.
	maxEventBytes = 64 << 10
)

// ErrNoPluginData is returned when the plugin data directory is unknown.
var ErrNoPluginData = errors.New("spool: CLAUDE_PLUGIN_DATA is not set")

// Spool is a directory triple under the plugin data directory. It is safe to
// use from independent processes: writes are atomic renames.
type Spool struct {
	root     string
	now      func() time.Time
	seen     map[string]struct{}
	seenRing []string
}

// New returns a spool rooted at <stateDir>/events.
func New(stateDir string) *Spool {
	return &Spool{
		root: filepath.Join(stateDir, "events"),
		now:  time.Now,
		seen: make(map[string]struct{}, recentIDs),
	}
}

// SetClock replaces the clock. Tests use it; production does not.
func (s *Spool) SetClock(now func() time.Time) { s.now = now }

func (s *Spool) dir(name string) string { return filepath.Join(s.root, name) }

func (s *Spool) ensureDirs() error {
	for _, name := range []string{"tmp", "inbox", "rejected"} {
		if err := os.MkdirAll(s.dir(name), 0o700); err != nil {
			return fmt.Errorf("spool: create %s: %w", name, err)
		}
	}
	return nil
}

// Validate enforces the event contract, including the privacy allowlist. It
// runs on write so a faulty adapter cannot persist a payload field, and again
// on read so a file written by an older or tampered build cannot smuggle one
// in.
func Validate(event contracts.RitualEvent) error {
	switch {
	case event.SchemaVersion != SchemaVersion:
		return fmt.Errorf("spool: unsupported schema version %d", event.SchemaVersion)
	case event.ID == "":
		return errors.New("spool: event has no ID")
	case event.SessionID == "":
		return errors.New("spool: event has no session ID")
	case event.Host == "":
		return errors.New("spool: event has no host")
	case event.Type == "":
		return errors.New("spool: event has no type")
	case event.OccurredAt.IsZero():
		return errors.New("spool: event has no timestamp")
	}

	for key := range event.Attributes {
		if _, ok := contracts.AllowedEventAttributeKeys[key]; !ok {
			return fmt.Errorf("spool: attribute %q is not on the privacy allowlist", key)
		}
	}
	return nil
}

// Write stores one event. It succeeds whether or not a watcher is running.
func (s *Spool) Write(event contracts.RitualEvent) error {
	if err := Validate(event); err != nil {
		return err
	}
	if err := s.ensureDirs(); err != nil {
		return err
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("spool: marshal event: %w", err)
	}

	// Write to tmp/ first so a reader never observes a partial file, then
	// rename into inbox/. rename(2) within one directory tree is atomic.
	tmp, err := os.CreateTemp(s.dir("tmp"), "event-*.json")
	if err != nil {
		return fmt.Errorf("spool: create temp event: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("spool: write event: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("spool: close event: %w", err)
	}

	if err := os.Rename(tmpName, filepath.Join(s.dir("inbox"), filename(event))); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("spool: publish event: %w", err)
	}

	return s.trim()
}

// filename orders events by occurrence, so a lexical directory sort is FIFO.
// The event ID keeps two events in the same nanosecond apart.
func filename(event contracts.RitualEvent) string {
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, event.ID)
	return fmt.Sprintf("%020d-%s.json", event.OccurredAt.UTC().UnixNano(), safe)
}

// trim drops the oldest events once the inbox exceeds maxInbox, which is what
// happens when hooks run for a long time with no watcher attached.
func (s *Spool) trim() error {
	names, err := inboxNames(s.dir("inbox"))
	if err != nil {
		return err
	}
	if len(names) <= maxInbox {
		return nil
	}
	for _, name := range names[:len(names)-maxInbox] {
		os.Remove(filepath.Join(s.dir("inbox"), name))
	}
	return nil
}

func inboxNames(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("spool: read inbox: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names, nil
}

// Read consumes up to limit events in FIFO order.
//
// Consumed files are deleted, so an event is delivered at most once. A crash
// mid-batch loses that batch; for a decorative status display that is a better
// trade than an acknowledgement protocol.
//
// Unreadable or invalid events are moved to rejected/ instead of being deleted
// or retried, so one bad file can never wedge the reader.
func (s *Spool) Read(limit int) ([]contracts.RitualEvent, error) {
	if limit <= 0 {
		limit = DefaultBatch
	}

	names, err := inboxNames(s.dir("inbox"))
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, nil
	}
	if len(names) > limit {
		names = names[:limit]
	}

	events := make([]contracts.RitualEvent, 0, len(names))
	for _, name := range names {
		path := filepath.Join(s.dir("inbox"), name)

		event, err := readEvent(path)
		if err != nil {
			s.reject(path, name)
			continue
		}
		if _, duplicate := s.seen[event.ID]; duplicate {
			os.Remove(path)
			continue
		}

		s.remember(event.ID)
		events = append(events, event)
		os.Remove(path)
	}

	return events, nil
}

func readEvent(path string) (contracts.RitualEvent, error) {
	var event contracts.RitualEvent

	info, err := os.Stat(path)
	if err != nil {
		return event, err
	}
	if info.Size() > maxEventBytes {
		return event, fmt.Errorf("spool: event file is %d bytes", info.Size())
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return event, err
	}
	if err := json.Unmarshal(raw, &event); err != nil {
		return event, err
	}
	return event, Validate(event)
}

func (s *Spool) reject(path, name string) {
	if err := os.MkdirAll(s.dir("rejected"), 0o700); err != nil {
		os.Remove(path)
		return
	}
	if err := os.Rename(path, filepath.Join(s.dir("rejected"), name)); err != nil {
		os.Remove(path)
	}
}

func (s *Spool) remember(id string) {
	s.seen[id] = struct{}{}
	s.seenRing = append(s.seenRing, id)
	if len(s.seenRing) > recentIDs {
		delete(s.seen, s.seenRing[0])
		s.seenRing = s.seenRing[1:]
	}
}

// Cleanup removes rejected events and abandoned temp files older than maxAge.
// A crashed hook can leave a temp file behind; nothing else collects them.
func (s *Spool) Cleanup(maxAge time.Duration) error {
	cutoff := s.now().Add(-maxAge)

	for _, name := range []string{"tmp", "rejected"} {
		entries, err := os.ReadDir(s.dir(name))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("spool: read %s: %w", name, err)
		}
		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil || info.ModTime().After(cutoff) {
				continue
			}
			os.Remove(filepath.Join(s.dir(name), entry.Name()))
		}
	}
	return nil
}
