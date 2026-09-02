package cache

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"tls-rest/go/engine/controllers/db/rdb"
)

var SessionCacheInstance *Cache[Session]

func init() {
	SessionCacheInstance = NewCache(func(key string) (Session, error) {
		// Simulate an expensive operation
		dummy := Session{}
		redisClient, err := rdb.GetInstance()

		if err != nil {
			return dummy, err
		}

		ctx := context.Background()

		data, err := redisClient.Get(ctx, key).Result()
		if err == nil && data != "" {
			ci, err := DecodeJson(data)
			if err != nil {
				return dummy, err
			}

			return ci, nil
		}

		return dummy, err
	}, func(key string, value Session) error {
		// Simulate an expensive operation
		redisClient, err := rdb.GetInstance()
		if err != nil {
			return err
		}

		ctx := context.Background()

		data, err := value.EncodeJson()
		if err != nil {
			return err
		}

		duration := time.Until(value.Expire)

		return redisClient.Set(ctx, key, data, duration).Err()
	})
}

type Session struct {
	UserAgent string
	IP        string
	UserID    int
	Username  string
	IsAdmin   bool

	// ModuleModes is the resolved per-module allowed-mode bitmask for this user
	// (module name -> auth.MODE_* bits), OR-ed across their groups. AccessLevel is
	// the highest access level among their groups; records at a higher level are
	// hidden. Both are filled by ManageSession and consumed by the middleware
	// (mode gating) and the fieldset engine (request-time row/field filtering).
	ModuleModes map[string]int
	AccessLevel int
	// FieldRights restricts, per module, which non-system fields the user may
	// see/write. A module ABSENT from the map is unrestricted (all fields). A
	// module PRESENT maps to the exact allowed field set (system fields like id
	// are always kept). Built by auth.ResolveModuleFieldRights.
	FieldRights map[string]map[string]int

	// Fieldset caches, per module, the hashsum of the fieldset last computed and
	// served to this session (authority-scoped). GetFieldset compares an incoming
	// ?hash= against this to answer 304 (unchanged) without recomputing.
	Fieldset map[string]string

	Expire     time.Time
	LastAccess time.Time

	// RightsEpoch is the value of auth's global rights epoch when this session's
	// rights (ModuleModes/FieldRights/AccessLevel/IsAdmin) were last resolved.
	// The per-request path re-resolves only when it no longer matches the current
	// epoch (i.e. a user/group/rights change bumped it), so rights are cached in
	// the session and recomputed on change instead of on every request.
	RightsEpoch int64
}

// ContextKey is the type used for request-context keys defined by this package.
type ContextKey string

// SessionKey is the context key under which the middleware stores the request's
// *Session. It lives here (rather than in auth) so packages that cannot import
// auth without creating an import cycle — notably the module engine — can still
// read the session via SessionFromContext. auth.SESSION_KEY aliases this.
const SessionKey ContextKey = "session"

// SessionFromContext returns the *Session stored on the context, or nil.
func SessionFromContext(ctx context.Context) *Session {
	if ctx == nil {
		return nil
	}
	if s, ok := ctx.Value(SessionKey).(*Session); ok {
		return s
	}
	return nil
}

type Cache[T any] struct {
	sync.Mutex
	data       map[string]T
	getter     func(string) (T, error)
	setter     func(string, T) error
	lastAccess map[string]time.Time
	expireOn   map[string]time.Time
	sizes      map[string]int64

	// Memory management. ttl == 0 disables in-memory caching (getter/setter
	// passthrough), preserving the original behaviour for callers that don't
	// opt in. maxEntries / maxBytes bound memory via LRU eviction.
	ttl        time.Duration
	maxEntries int
	maxBytes   int64
	curBytes   int64
	sizeOf     func(T) int64
}

func (item *Session) EncodeJson() (string, error) {
	bytes, err := json.Marshal(item)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func DecodeJson(data string) (Session, error) {
	ci := Session{}
	err := json.Unmarshal([]byte(data), &ci)

	if err != nil {
		return Session{}, err
	}
	return ci, nil
}

func NewCache[T any](getter func(string) (T, error), setter func(string, T) error) *Cache[T] {
	return &Cache[T]{
		data:       make(map[string]T),
		getter:     getter,
		setter:     setter,
		lastAccess: make(map[string]time.Time),
		expireOn:   make(map[string]time.Time),
		sizes:      make(map[string]int64),
		// ttl defaults to 0 => in-memory caching disabled; enable with WithTTL.
	}
}

// WithTTL enables in-memory caching and sets how long an entry stays fresh
// before it must be reloaded via the getter. Returns the cache for chaining.
func (c *Cache[T]) WithTTL(d time.Duration) *Cache[T] {
	c.Lock()
	c.ttl = d
	c.Unlock()
	return c
}

// WithMaxEntries caps the number of cached entries; the least-recently-used
// entries are evicted once the cap is exceeded. 0 == unlimited.
func (c *Cache[T]) WithMaxEntries(n int) *Cache[T] {
	c.Lock()
	c.maxEntries = n
	c.Unlock()
	return c
}

// WithMaxBytes caps the total memory used by cached values, measured by sizeOf
// (e.g. len(data) for byte blobs). LRU entries are evicted once the cap is
// exceeded. maxBytes 0 == unbounded.
func (c *Cache[T]) WithMaxBytes(maxBytes int64, sizeOf func(T) int64) *Cache[T] {
	c.Lock()
	c.maxBytes = maxBytes
	c.sizeOf = sizeOf
	c.Unlock()
	return c
}

func (c *Cache[T]) Set(key string, value T) {
	c.Lock()
	defer c.Unlock()

	if c.setter != nil {
		go c.setter(key, value)
	}
	c.store(key, value)
}

// Get retrieves a value by key.
func (c *Cache[T]) Get(key string) (*T, error) {
	c.Lock()
	defer c.Unlock()

	if value, ok := c.data[key]; ok {
		if time.Now().Before(c.expireOn[key]) {
			c.lastAccess[key] = time.Now()
			return &value, nil
		}
		// Expired: drop it and reload below.
		c.remove(key)
	}

	value, err := c.getter(key)
	if err != nil {
		return new(T), err
	}

	c.store(key, value)
	return &value, nil
}

// store inserts/updates an entry and enforces the memory limits. It must be
// called with the lock held. With ttl == 0 the value is not retained (the
// cache acts as a pure getter/setter passthrough).
func (c *Cache[T]) store(key string, value T) {
	if c.ttl <= 0 {
		return
	}

	// Account for a replaced entry's old size.
	if old, ok := c.sizes[key]; ok {
		c.curBytes -= old
	}

	now := time.Now()
	c.data[key] = value
	c.lastAccess[key] = now
	c.expireOn[key] = now.Add(c.ttl)

	if c.sizeOf != nil {
		sz := c.sizeOf(value)
		c.sizes[key] = sz
		c.curBytes += sz
	}

	c.evict()
}

// remove deletes an entry and its bookkeeping. Must hold the lock.
func (c *Cache[T]) remove(key string) {
	delete(c.data, key)
	delete(c.lastAccess, key)
	delete(c.expireOn, key)
	if sz, ok := c.sizes[key]; ok {
		c.curBytes -= sz
		delete(c.sizes, key)
	}
}

// evict drops expired entries, then removes least-recently-used entries until
// the entry-count and byte limits are satisfied. Must hold the lock.
func (c *Cache[T]) evict() {
	now := time.Now()
	for k, exp := range c.expireOn {
		if now.After(exp) {
			c.remove(k)
		}
	}

	overEntries := func() bool { return c.maxEntries > 0 && len(c.data) > c.maxEntries }
	overBytes := func() bool { return c.maxBytes > 0 && c.curBytes > c.maxBytes }

	for overEntries() || overBytes() {
		var lruKey string
		var lruTime time.Time
		first := true
		for k, t := range c.lastAccess {
			if first || t.Before(lruTime) {
				lruKey, lruTime, first = k, t, false
			}
		}
		if first { // nothing left to evict
			break
		}
		c.remove(lruKey)
	}
}

// Keys returns all cached keys (admin/console introspection).
func (c *Cache[T]) Keys() []string {
	c.Lock()
	defer c.Unlock()
	keys := make([]string, 0, len(c.data))
	for k := range c.data {
		keys = append(keys, k)
	}
	return keys
}

// Len returns the number of cached entries.
func (c *Cache[T]) Len() int {
	c.Lock()
	defer c.Unlock()
	return len(c.data)
}

// Delete removes a single entry.
func (c *Cache[T]) Delete(key string) {
	c.Lock()
	defer c.Unlock()
	if _, ok := c.data[key]; ok {
		c.curBytes -= c.sizes[key]
		delete(c.data, key)
		delete(c.lastAccess, key)
		delete(c.expireOn, key)
		delete(c.sizes, key)
	}
}

// Clear removes every entry.
func (c *Cache[T]) Clear() {
	c.Lock()
	defer c.Unlock()
	c.data = make(map[string]T)
	c.lastAccess = make(map[string]time.Time)
	c.expireOn = make(map[string]time.Time)
	c.sizes = make(map[string]int64)
	c.curBytes = 0
}
