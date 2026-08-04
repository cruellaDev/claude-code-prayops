// Package watch is the full PrayOps altar.
//
// The Bubble Tea model here is a thin shell: every decision it makes lives in
// Advance, which needs no terminal and is tested directly.
package watch

import (
	"os"
	"path/filepath"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/cruellaDev/claude-code-prayops/contracts"
	"github.com/cruellaDev/claude-code-prayops/internal/effect"
	"github.com/cruellaDev/claude-code-prayops/internal/layout"
	"github.com/cruellaDev/claude-code-prayops/internal/prayer"
	"github.com/cruellaDev/claude-code-prayops/internal/render"
	"github.com/cruellaDev/claude-code-prayops/internal/session"
	"github.com/cruellaDev/claude-code-prayops/internal/smoke"
	"github.com/cruellaDev/claude-code-prayops/internal/spool"
)

// Frame rates. A busy session animates; an idle one should not spin a CPU.
const (
	ActiveFPS = 12
	IdleFPS   = 4
)

// heartbeatEvery is how often the watcher touches its heartbeat file. Doctor
// treats a heartbeat older than a minute as "not running".
const heartbeatEvery = 15 * time.Second

// Options configure a watcher.
type Options struct {
	// StateDir is <plugin data>/state.
	StateDir string
	// Host selects which sessions to display.
	Host contracts.Host
	// Motion off disables drift for reduced-motion users.
	Motion bool
	// Color emits ANSI colour for prayer images.
	Color bool
	// Seed makes the smoke reproducible; zero picks one from the clock.
	Seed uint64
	// Now is injectable for tests.
	Now func() time.Time
}

// Model is the Bubble Tea model for the altar.
type Model struct {
	opts   Options
	events *spool.Spool
	store  *session.Store
	smoke  *smoke.Field
	layout contracts.Layout

	cache  *prayer.Cache
	active *activeEffect

	heartbeat     string
	lastHeartbeat time.Time
	quitting      bool
}

// activeEffect is the one prayer on screen. There is never more than one; the
// rest wait in the session's queue.
type activeEffect struct {
	cacheID   string
	raster    contracts.TerminalRaster
	placement contracts.Placement
	mask      effect.Mask
	timeline  effect.Timeline
	started   time.Time
}

// New returns a watcher model.
func New(opts Options) *Model {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Host == "" {
		opts.Host = contracts.HostClaude
	}
	seed := opts.Seed
	if seed == 0 {
		seed = uint64(opts.Now().UnixNano())
	}

	events := spool.New(opts.StateDir)
	events.SetClock(opts.Now)

	return &Model{
		opts:   opts,
		events: events,
		store:  session.NewStore(),
		cache:  newCache(opts),
		smoke:  smoke.New(seed),
		layout: layout.Compute(80, 24),
	}
}

func newCache(opts Options) *prayer.Cache {
	cache := prayer.New(opts.StateDir)
	cache.SetClock(opts.Now)
	return cache
}

// State returns the session currently being displayed.
func (m *Model) State() contracts.SessionState {
	state, _ := m.store.Active()
	return state
}

// Advance consumes pending events and steps the animation by one frame.
//
// This is the whole watcher loop. Bubble Tea only decides when to call it.
func (m *Model) Advance() {
	events, err := m.events.Read(spool.DefaultBatch)
	if err == nil {
		for _, event := range events {
			if event.Host == m.opts.Host {
				m.store.Apply(event)
			}
		}
	}

	burst := m.stepEffect()

	m.smoke.Step(smoke.Options{
		Layout:  m.layout,
		Phase:   m.State().Phase,
		Motion:  m.opts.Motion,
		Burst:   burst,
		BurstAt: m.burstOrigin(),
	})
}

// stepEffect retires a finished prayer, starts the next one, and reports how
// much smoke this frame should emit for it.
func (m *Model) stepEffect() int {
	now := m.opts.Now()

	if m.active != nil {
		phase, _ := m.active.timeline.At(now.Sub(m.active.started))
		switch phase {
		case effect.PhaseDone:
			m.cache.Discard(m.active.cacheID)
			m.active = nil
		case effect.PhaseTrail:
			// The image is gone; only smoke marks where it was.
			return 2
		default:
			return 0
		}
	}

	state := m.State()
	if len(state.PrayerQueue) == 0 {
		return 0
	}

	// PRY-04: one effect is active at a time, so the rest stay queued.
	request := state.PrayerQueue[0]
	state.PrayerQueue = state.PrayerQueue[1:]
	m.store.Put(state)

	art, err := m.cache.Load(request.Source.CacheID)
	if err != nil || art.Width == 0 || art.Height == 0 {
		// The prayer was never cached or has been collected. Losing an
		// ornament is not worth reporting; the queue simply moves on.
		return 0
	}

	placement, ok := effect.Place(request.Seed, m.layout, art.Width, art.Height, nil)
	if !ok {
		// Nothing fits on this terminal. Keep the smoke so something happens.
		m.cache.Discard(request.Source.CacheID)
		return 4
	}

	timeline := effect.NewTimeline(request.Duration)
	m.active = &activeEffect{
		cacheID:   request.Source.CacheID,
		raster:    art,
		placement: placement,
		mask:      effect.NewMask(request.Seed, timeline),
		timeline:  timeline,
		started:   now,
	}
	return 4
}

// burstOrigin is where prayer smoke comes from: the effect if one is on
// screen, otherwise the incense.
func (m *Model) burstOrigin() contracts.Rect {
	if m.active == nil {
		return contracts.Rect{}
	}
	return m.active.placement.Rect
}

// Resize recomputes the layout. The smoke field is kept: re-seeding on resize
// would restart the scene every time a pane moves.
func (m *Model) Resize(width, height int) {
	m.layout = layout.Compute(width, height)

	// D-028: an effect keeps its zone and relative offset across a resize
	// rather than being re-rolled, so it stays where the user saw it.
	if m.active != nil {
		placement, ok := effect.Reposition(m.active.placement, m.layout,
			m.active.raster.Width, m.active.raster.Height)
		if !ok {
			m.cache.Discard(m.active.cacheID)
			m.active = nil
			return
		}
		m.active.placement = placement
	}
}

// Interval is how long to wait before the next frame.
func (m *Model) Interval() time.Duration {
	fps := IdleFPS
	switch m.State().Phase {
	case contracts.PhaseThinking, contracts.PhaseWorking, contracts.PhaseApprovalRequired:
		fps = ActiveFPS
	}
	return time.Second / time.Duration(fps)
}

// Frame renders the current scene.
func (m *Model) Frame() string {
	opts := render.Options{Smoke: m.smoke.Render(m.layout), Color: m.opts.Color}

	if m.active != nil {
		elapsed := m.opts.Now().Sub(m.active.started)
		mask := m.active.mask
		opts.Prayer = &render.Prayer{
			Raster:  m.active.raster,
			Rect:    m.active.placement.Rect,
			Visible: func(x, y int) bool { return mask.Visible(x, y, elapsed) },
			Rise:    func(x, y int) int { return mask.Rise(x, y, elapsed) },
		}
	}
	return render.Frame(m.layout, m.State(), opts)
}

type tickMsg time.Time

func (m *Model) tick() tea.Cmd {
	return tea.Tick(m.Interval(), func(t time.Time) tea.Msg { return tickMsg(t) })
}

// Init starts the animation loop.
func (m *Model) Init() tea.Cmd {
	m.startHeartbeat()
	m.collect()
	return m.tick()
}

// collect discards quarantined events and files left by crashed hooks.
//
// Nothing else removes them, so without this a single corrupt event would
// leave doctor reporting the same warning for the life of the installation.
// The watcher does it because it is the only long-running process, and it does
// it once at startup so a frame never pays for it.
func (m *Model) collect() {
	_ = m.events.Cleanup(StaleAfter)
	_ = m.cache.Cleanup(StaleAfter)
}

// StaleAfter is how long a quarantined event or an unshown prayer is kept.
const StaleAfter = 24 * time.Hour

// Update handles resize, keys, and the frame tick.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Resize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.quitting = true
			m.stopHeartbeat()
			return m, tea.Quit
		}
		return m, nil

	case tickMsg:
		if m.quitting {
			return m, nil
		}
		m.Advance()
		m.beat()
		return m, m.tick()
	}

	return m, nil
}

// View renders one frame into the alternate screen.
func (m *Model) View() tea.View {
	view := tea.NewView(m.Frame())
	// The alternate screen is what restores the user's terminal on exit,
	// including after a panic, because Bubble Tea leaves it on the way out.
	view.AltScreen = true
	return view
}

// startHeartbeat publishes this watcher so doctor can see it.
func (m *Model) startHeartbeat() {
	if m.opts.StateDir == "" {
		return
	}
	dir := filepath.Join(m.opts.StateDir, "watchers")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	m.heartbeat = filepath.Join(dir, strconv.Itoa(os.Getpid()))
	m.beat()
}

func (m *Model) beat() {
	if m.heartbeat == "" {
		return
	}
	now := m.opts.Now()
	if now.Sub(m.lastHeartbeat) < heartbeatEvery {
		return
	}
	if err := os.WriteFile(m.heartbeat, []byte(now.UTC().Format(time.RFC3339)), 0o600); err == nil {
		m.lastHeartbeat = now
	}
}

func (m *Model) stopHeartbeat() {
	if m.heartbeat == "" {
		return
	}
	os.Remove(m.heartbeat)
	m.heartbeat = ""
}

// Run starts the watcher and blocks until the user quits.
func Run(opts Options) error {
	model := New(opts)
	program := tea.NewProgram(model)

	// The heartbeat is removed on the way out however the program ends, so a
	// crashed watcher does not look like a running one to doctor.
	defer model.stopHeartbeat()

	_, err := program.Run()
	return err
}
