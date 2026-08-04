// Package prayer stores rendered prayer visuals for the watcher to pick up.
//
// The event that travels through the spool carries only a cache id, because
// the privacy allowlist has no room for a prayer's text or an image path. What
// is stored here is the downsampled cell grid, never the original file.
package prayer

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cruellaDev/claude-code-prayops/contracts"
)

// maxCacheBytes bounds one cached raster. A 24x12 grid is a few kilobytes.
const maxCacheBytes = 256 << 10

// Cache is the prayer cache under <state>/cache.
type Cache struct {
	dir string
	now func() time.Time
}

// New returns the cache for a state directory.
func New(stateDir string) *Cache {
	return &Cache{dir: filepath.Join(stateDir, "cache"), now: time.Now}
}

// SetClock replaces the clock. Tests use it.
func (c *Cache) SetClock(now func() time.Time) { c.now = now }

// NewID returns an identifier for one prayer.
func NewID() string {
	var buf [12]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("t%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf[:])
}

// safeName keeps an id from escaping the cache directory.
func safeName(id string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, id)
}

func (c *Cache) path(id string) string { return filepath.Join(c.dir, safeName(id)+".json") }

// Save stores a rendered prayer.
func (c *Cache) Save(id string, raster contracts.TerminalRaster) error {
	if id == "" {
		return fmt.Errorf("prayer: cache id is empty")
	}
	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		return fmt.Errorf("prayer: create cache: %w", err)
	}

	payload, err := json.Marshal(raster)
	if err != nil {
		return fmt.Errorf("prayer: marshal raster: %w", err)
	}
	if len(payload) > maxCacheBytes {
		return fmt.Errorf("prayer: raster is %d bytes, over the cache limit", len(payload))
	}

	tmp, err := os.CreateTemp(c.dir, "prayer-*.json")
	if err != nil {
		return fmt.Errorf("prayer: create temp: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("prayer: write raster: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("prayer: close raster: %w", err)
	}
	if err := os.Rename(tmpName, c.path(id)); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("prayer: publish raster: %w", err)
	}
	return nil
}

// Load returns a stored prayer.
func (c *Cache) Load(id string) (contracts.TerminalRaster, error) {
	var raster contracts.TerminalRaster
	if id == "" {
		return raster, fmt.Errorf("prayer: cache id is empty")
	}

	info, err := os.Stat(c.path(id))
	if err != nil {
		return raster, fmt.Errorf("prayer: %w", err)
	}
	if info.Size() > maxCacheBytes {
		return raster, fmt.Errorf("prayer: cached raster is %d bytes", info.Size())
	}

	payload, err := os.ReadFile(c.path(id))
	if err != nil {
		return raster, fmt.Errorf("prayer: read raster: %w", err)
	}
	if err := json.Unmarshal(payload, &raster); err != nil {
		return raster, fmt.Errorf("prayer: parse raster: %w", err)
	}
	return raster, nil
}

// Discard removes one prayer once it has been shown.
func (c *Cache) Discard(id string) {
	if id == "" {
		return
	}
	os.Remove(c.path(id))
}

// Cleanup removes prayers older than maxAge.
//
// A prayer sent with no watcher running is never collected by Discard, so
// without this the cache would keep every unseen prayer forever.
func (c *Cache) Cleanup(maxAge time.Duration) error {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("prayer: read cache: %w", err)
	}

	cutoff := c.now().Add(-maxAge)
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		os.Remove(filepath.Join(c.dir, entry.Name()))
	}
	return nil
}
