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
	"github.com/cruellaDev/claude-code-prayops/internal/layout"
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

	heartbeat     string
	lastHeartbeat time.Time
	quitting      bool
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

	return &Model{
		opts:   opts,
		events: spool.New(opts.StateDir),
		store:  session.NewStore(),
		smoke:  smoke.New(seed),
		layout: layout.Compute(80, 24),
	}
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

	state := m.State()
	burst := 0
	if len(state.PrayerQueue) > 0 {
		burst = 4
		// ponytail: a queued prayer only puffs smoke for now. Consume it here
		// so the burst fires once instead of every frame; PRY-03 replaces this
		// with the dissolve effect that actually owns the queue.
		m.consumePrayer(state)
	}

	m.smoke.Step(smoke.Options{
		Layout: m.layout,
		Phase:  state.Phase,
		Motion: m.opts.Motion,
		Burst:  burst,
	})
}

// consumePrayer drops the oldest queued prayer from the displayed session.
func (m *Model) consumePrayer(state contracts.SessionState) {
	state.PrayerQueue = state.PrayerQueue[1:]
	m.store.Put(state)
}

// Resize recomputes the layout. The smoke field is kept: re-seeding on resize
// would restart the scene every time a pane moves.
func (m *Model) Resize(width, height int) {
	m.layout = layout.Compute(width, height)
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
	return render.Frame(m.layout, m.State(), render.Options{Smoke: m.smoke.Render(m.layout)})
}

type tickMsg time.Time

func (m *Model) tick() tea.Cmd {
	return tea.Tick(m.Interval(), func(t time.Time) tea.Msg { return tickMsg(t) })
}

// Init starts the animation loop.
func (m *Model) Init() tea.Cmd {
	m.startHeartbeat()
	return m.tick()
}

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
