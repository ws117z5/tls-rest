// Package actions is a registry of admin-triggerable units of work — each one
// registered once (typically from a package's Init) and then runnable on
// demand or on an interval from the Actions page, instead of every feature
// growing its own bespoke "run this" button and endpoint.
package actions

import (
	"errors"
	"sync"
	"time"
)

// maxLogEntries bounds each action's kept history — old runs (manual or
// scheduled) fall off the front once exceeded, so a frequently-scheduled
// action can't grow this without limit.
const maxLogEntries = 20

// LogEntry is one past run of an action, newest first in Action.Log().
type LogEntry struct {
	Time   time.Time `json:"time"`
	Result string    `json:"result,omitempty"`
	Error  string    `json:"error,omitempty"`
}

// Action is one registrable unit of work. Run does the actual job and
// returns a short human-readable result; Interval/last-run bookkeeping below
// is unexported and managed by RunNow/SetSchedule.
type Action struct {
	ID          string
	Name        string
	Description string
	Run         func() (string, error)
	Command     []string // interactive alternative to Run: runs in a pty in Dir (see process.go)
	Dir         string

	proc     *process
	mu       sync.Mutex
	interval time.Duration
	stop     chan struct{}
	running  bool
	lastRun  time.Time
	result   string
	lastErr  string
	log      []LogEntry
}

// Snapshot is a read-only, JSON-safe view of an action's config, last-run
// state, and run history (Action itself holds a mutex, so it isn't
// serialized directly).
type Snapshot struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	IntervalSec int        `json:"interval_seconds"`
	Running     bool       `json:"running"`
	Interactive bool       `json:"interactive,omitempty"`
	LastRun     string     `json:"last_run,omitempty"`
	LastResult  string     `json:"last_result,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
	Log         []LogEntry `json:"log,omitempty"`
}

var (
	regMu sync.RWMutex
	reg   = map[string]*Action{}
)

// Register adds a to the registry (or replaces an existing entry with the
// same ID) — call once, typically from the owning package's Init.
func Register(a *Action) {
	regMu.Lock()
	defer regMu.Unlock()
	reg[a.ID] = a
}

// All returns every registered action.
func All() []*Action {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]*Action, 0, len(reg))
	for _, a := range reg {
		out = append(out, a)
	}
	return out
}

// Get looks up a registered action by ID.
func Get(id string) (*Action, bool) {
	regMu.RLock()
	defer regMu.RUnlock()
	a, ok := reg[id]
	return a, ok
}

// RunNow executes the action synchronously and records the outcome; refuses
// to start a second overlapping run of the same action.
func (a *Action) RunNow() (string, error) {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return "", errors.New("already running")
	}
	a.running = true
	a.mu.Unlock()

	if len(a.Command) > 0 {
		if err := a.startProcess(); err != nil {
			a.record("", err)
			return "", err
		}
		return "started", nil
	}

	result, err := a.Run()
	a.record(result, err)
	return result, err
}

// record stores one finished run's outcome in the action's state and log.
func (a *Action) record(result string, err error) {
	a.mu.Lock()
	a.running = false
	a.lastRun = time.Now()
	a.result = result
	entry := LogEntry{Time: a.lastRun, Result: result}
	if err != nil {
		a.lastErr = err.Error()
		entry.Error = a.lastErr
	} else {
		a.lastErr = ""
	}
	a.log = append(a.log, entry)
	if len(a.log) > maxLogEntries {
		a.log = a.log[len(a.log)-maxLogEntries:]
	}
	a.mu.Unlock()
}

// SetSchedule runs the action every interval from now on; interval <= 0
// cancels any existing schedule. In-memory only — lost on restart.
func (a *Action) SetSchedule(interval time.Duration) {
	a.mu.Lock()
	if a.stop != nil {
		close(a.stop)
		a.stop = nil
	}
	a.interval = interval
	if interval <= 0 {
		a.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	a.stop = stop
	a.mu.Unlock()

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_, _ = a.RunNow()
			case <-stop:
				return
			}
		}
	}()
}

// Snapshot returns a JSON-safe copy of a's current config and last-run state.
func (a *Action) Snapshot() Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := Snapshot{
		ID: a.ID, Name: a.Name, Description: a.Description,
		IntervalSec: int(a.interval.Seconds()), Running: a.running, Interactive: len(a.Command) > 0,
		LastResult: a.result, LastError: a.lastErr,
	}
	if !a.lastRun.IsZero() {
		s.LastRun = a.lastRun.UTC().Format(time.RFC3339)
	}
	s.Log = make([]LogEntry, len(a.log))
	for i, e := range a.log {
		s.Log[len(a.log)-1-i] = e // newest first
	}
	return s
}
