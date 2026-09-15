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

// Action is one registrable unit of work. Run does the actual job and
// returns a short human-readable result; Interval/last-run bookkeeping below
// is unexported and managed by RunNow/SetSchedule.
type Action struct {
	ID          string
	Name        string
	Description string
	Run         func() (string, error)

	mu       sync.Mutex
	interval time.Duration
	stop     chan struct{}
	running  bool
	lastRun  time.Time
	result   string
	lastErr  string
}

// Snapshot is a read-only, JSON-safe view of an action's config and last-run
// state (Action itself holds a mutex, so it isn't serialized directly).
type Snapshot struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IntervalSec int    `json:"interval_seconds"`
	Running     bool   `json:"running"`
	LastRun     string `json:"last_run,omitempty"`
	LastResult  string `json:"last_result,omitempty"`
	LastError   string `json:"last_error,omitempty"`
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

	result, err := a.Run()

	a.mu.Lock()
	a.running = false
	a.lastRun = time.Now()
	a.result = result
	if err != nil {
		a.lastErr = err.Error()
	} else {
		a.lastErr = ""
	}
	a.mu.Unlock()

	return result, err
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
		IntervalSec: int(a.interval.Seconds()), Running: a.running,
		LastResult: a.result, LastError: a.lastErr,
	}
	if !a.lastRun.IsZero() {
		s.LastRun = a.lastRun.UTC().Format(time.RFC3339)
	}
	return s
}
