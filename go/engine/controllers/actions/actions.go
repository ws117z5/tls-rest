// Package actions is a registry of admin-triggerable units of work — each one
// registered once (typically from a package's Init) and then runnable on
// demand or on an interval from the Actions page, instead of every feature
// growing its own bespoke "run this" button and endpoint.
package actions

import (
	"errors"
	"sync"
	"time"

	"tls-rest/go/engine/controllers/field"
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
	// Env is extra "KEY=VALUE" entries appended after the subprocess's inherited environment.
	Env []string

	// Params declares this action's per-run configurable values, rendered on
	// the detail page the same way a module's own fieldset renders a create
	// form (same Field type, same JSON shape — see Snapshot.Params). Leave
	// nil for an action that takes no configuration.
	Params []field.Field
	// RunFunc/CommandFunc are the param-aware alternatives to Run/Command —
	// RunNow prefers these when Params is non-empty. params is the submitted
	// values keyed by Params[i].Name, already defaulted from LastParams for
	// anything the caller omitted (see RunNow).
	RunFunc     func(params map[string]interface{}) (string, error)
	CommandFunc func(params map[string]interface{}) []string

	proc       *process
	mu         sync.Mutex
	interval   time.Duration
	stop       chan struct{}
	running    bool
	lastRun    time.Time
	result     string
	lastErr    string
	log        []LogEntry
	lastParams map[string]interface{}
}

// Snapshot is a read-only, JSON-safe view of an action's config, last-run
// state, and run history (Action itself holds a mutex, so it isn't
// serialized directly).
type Snapshot struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	IntervalSec int                    `json:"interval_seconds"`
	Running     bool                   `json:"running"`
	Interactive bool                   `json:"interactive,omitempty"`
	LastRun     string                 `json:"last_run,omitempty"`
	LastResult  string                 `json:"last_result,omitempty"`
	LastError   string                 `json:"last_error,omitempty"`
	Log         []LogEntry             `json:"log,omitempty"`
	Params      []field.Field          `json:"params,omitempty"`
	LastParams  map[string]interface{} `json:"last_params,omitempty"`
}

var (
	regMu sync.RWMutex
	reg   = map[string]*Action{}
	// order records registration order — Go deliberately randomizes map
	// iteration, so ranging reg directly (the old All()) gave the Actions
	// page a different row order on every single page load.
	order []string
)

// Register adds a to the registry (or replaces an existing entry with the
// same ID) — call once, typically from the owning package's Init.
func Register(a *Action) {
	regMu.Lock()
	defer regMu.Unlock()
	if _, exists := reg[a.ID]; !exists {
		order = append(order, a.ID)
	}
	reg[a.ID] = a
}

// All returns every registered action, in registration order.
func All() []*Action {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]*Action, 0, len(order))
	for _, id := range order {
		if a, ok := reg[id]; ok {
			out = append(out, a)
		}
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

// RunNow executes the action synchronously (or starts it, for a Command/
// CommandFunc action) and records the outcome; refuses to start a second
// overlapping run of the same action. params is merged on top of the
// action's last-used values (so the caller only needs to send what changed,
// and the detail page's prefilled form matches what actually ran) and that
// merged set becomes the new LastParams regardless of outcome.
func (a *Action) RunNow(params map[string]interface{}) (string, error) {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return "", errors.New("already running")
	}
	merged := make(map[string]interface{}, len(a.lastParams)+len(params))
	for k, v := range a.lastParams {
		merged[k] = v
	}
	for k, v := range params {
		merged[k] = v
	}
	a.lastParams = merged
	a.running = true
	a.mu.Unlock()

	if a.CommandFunc != nil || len(a.Command) > 0 {
		if err := a.startProcess(merged); err != nil {
			a.record("", err)
			return "", err
		}
		return "started", nil
	}

	var result string
	var err error
	if a.RunFunc != nil {
		result, err = a.RunFunc(merged)
	} else {
		result, err = a.Run()
	}
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
				_, _ = a.RunNow(nil)
			case <-stop:
				return
			}
		}
	}()
}

// resolveParamOptions resolves each param's OptionsFunc (if set) into
// Options["options"] — mirrors module/fieldset_handler.go's resolveFieldOptions
// for the same WithOptions(func) case. field.Field tags OptionsFunc json:"-"
// (funcs can't serialize), and unlike a module's own fieldset endpoint, nothing
// else in this package ever called the func to populate the static list — so a
// SELECT param rendered with no options at all, every time.
func resolveParamOptions(params []field.Field) []field.Field {
	if len(params) == 0 {
		return params
	}
	out := make([]field.Field, len(params))
	for i, p := range params {
		if p.OptionsFunc != nil {
			newOpts := make(map[string]interface{}, len(p.Options)+1)
			for k, v := range p.Options {
				newOpts[k] = v
			}
			newOpts["options"] = p.OptionsFunc()
			p.Options = newOpts
		}
		out[i] = p
	}
	return out
}

// Snapshot returns a JSON-safe copy of a's current config and last-run state.
func (a *Action) Snapshot() Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := Snapshot{
		ID: a.ID, Name: a.Name, Description: a.Description,
		IntervalSec: int(a.interval.Seconds()), Running: a.running,
		Interactive: len(a.Command) > 0 || a.CommandFunc != nil,
		LastResult:  a.result, LastError: a.lastErr,
		Params: resolveParamOptions(a.Params), LastParams: a.lastParams,
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
