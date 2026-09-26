package paymentplatforms

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"

	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"
)

// Instance is one configured payment platform row (a provider plus its credentials).
type Instance struct {
	ID       int64
	Name     string
	Provider string
	Sandbox  bool
	APIKey   string
	Config   map[string]interface{}
}

// Charge is what a provider is asked to collect.
type Charge struct {
	PaymentID   int64
	Amount      float64
	Currency    string
	Description string
}

// Result is a provider's answer: an external reference, a status, and optionally where to send the payer.
type Result struct {
	ExternalID  string `json:"external_id"`
	Status      string `json:"status"`
	RedirectURL string `json:"redirect_url,omitempty"`
	Message     string `json:"message,omitempty"`
}

// Provider is one payment backend; register implementations with RegisterProvider from init().
type Provider interface {
	ID() string
	Name() string
	Charge(ctx context.Context, inst Instance, c Charge) (Result, error)
}

var (
	mu        sync.RWMutex
	providers = map[string]Provider{}
)

func RegisterProvider(p Provider) {
	mu.Lock()
	defer mu.Unlock()
	providers[p.ID()] = p
}

// ProviderOptions lists registered providers as {value, name} select options.
func ProviderOptions() []map[string]interface{} {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]map[string]interface{}, 0, len(providers))
	for id, p := range providers {
		out = append(out, map[string]interface{}{"value": id, "name": p.Name()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["value"].(string) < out[j]["value"].(string) })
	return out
}

// Load reads an enabled platform instance by id.
func Load(ctx context.Context, id int64) (Instance, error) {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return Instance{}, err
	}
	row, err := db.GetOne("SELECT id, name, provider, sandbox, api_key, config FROM payment_platforms WHERE id = $1 AND enabled = true", id)
	if err != nil {
		return Instance{}, err
	}
	if row == nil {
		return Instance{}, errors.New("payment platform not found or disabled")
	}
	inst := Instance{
		ID:       functions.Coerce[int64](row["id"]),
		Name:     functions.Coerce[string](row["name"]),
		Provider: functions.Coerce[string](row["provider"]),
		Sandbox:  functions.Truthy(row["sandbox"]),
		APIKey:   functions.Coerce[string](row["api_key"]),
	}
	switch c := row["config"].(type) {
	case map[string]interface{}:
		inst.Config = c
	case string:
		_ = json.Unmarshal([]byte(c), &inst.Config)
	}
	return inst, nil
}

// Run charges through the instance's provider.
func Run(ctx context.Context, inst Instance, c Charge) (Result, error) {
	mu.RLock()
	p, ok := providers[inst.Provider]
	mu.RUnlock()
	if !ok {
		return Result{}, errors.New("unknown payment provider: " + inst.Provider)
	}
	return p.Charge(ctx, inst, c)
}
