// Package sessions is an admin-only module listing active server sessions
// straight out of Redis — no table of its own, since cache.SessionCacheInstance
// already owns that store (see go/engine/controllers/db/cache/cache.go).
package sessions

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/gorilla/mux"

	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/rdb"
	. "tls-rest/go/engine/controllers/field"
	. "tls-rest/go/engine/controllers/module"
	"tls-rest/go/engine/controllers/request"
)

type Sessions struct {
	*ModuleAbstract[any]
}

func (m *Sessions) fieldset() []Field {
	return []Field{
		NewField("kind", TYPE_STRING, false).WithLabel("Kind").AsReadOnly(),
		NewField("username", TYPE_STRING, false).WithLabel("Username").AsReadOnly(),
		NewField("user_id", TYPE_INT, false).WithLabel("User ID").AsReadOnly(),
		NewField("is_admin", TYPE_CHECKBOX, false).WithLabel("Admin").AsReadOnly(),
		NewField("ip", TYPE_STRING, false).WithLabel("IP").AsReadOnly(),
		NewField("user_agent", TYPE_STRING, false).WithLabel("User agent").AsReadOnly(),
		NewField("last_access", TYPE_DATE_TIME, false).WithLabel("Last access").AsReadOnly(),
		NewField("expire", TYPE_DATE_TIME, false).WithLabel("Expires").AsReadOnly(),
	}
}

func (m *Sessions) filters() *Filedset {
	return NewFieldset(
		NewFilter("username", TYPE_STRING).WithLabel("Username").Contains(),
		NewFilter("authenticated_only", TYPE_CHECKBOX).WithLabel("Authenticated only").Equals(),
	)
}

func readOnly(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "sessions can't be created or edited — delete one to revoke it", http.StatusMethodNotAllowed)
}

// The Redis key is the natural id, but it's raw standard base64 (can contain
// "/"), which would break a path-based /{id} route — re-encoded URL-safe for
// the id field, decoded back wherever a row is looked up by it.
func encodeID(key string) string { return base64.RawURLEncoding.EncodeToString([]byte(key)) }
func decodeID(id string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(id)
	return string(b), err
}

// isSessionShape mirrors this project's own earlier investigation into this
// exact Redis instance: a session key is base64 of 16 (web, GetRandomHash(16))
// or 32 (mobile/bearer, GetRandomHash(32)) random bytes — nothing else in a
// keyspace shared with unrelated data matches both that encoding and length.
func isSessionShape(key string) bool {
	return len(key) == 24 || len(key) == 44
}

// maxRows caps a single listing so a Redis instance with tens of thousands of
// keys (this one) doesn't decode all of them on every page load — sorted by
// LastAccess first, so a capped view still favors the sessions anyone's
// actually likely to be looking for.
const maxRows = 500

func fetchRows(ctx context.Context, r *http.Request, mode int) ([]map[string]any, error) {
	redisClient, err := rdb.GetInstance()
	if err != nil {
		return nil, err
	}

	rq := request.From(r)

	// Single-record view already knows the exact key — a direct GET, not a
	// full-keyspace SCAN just to find the one row.
	if mode == MODE_VIEW {
		if id := rq.String("id"); id != "" {
			key, derr := decodeID(id)
			if derr != nil {
				return nil, nil
			}
			data, gerr := redisClient.Get(ctx, key).Result()
			if gerr != nil {
				return nil, nil
			}
			s, derr := cache.DecodeJson(data)
			if derr != nil {
				return nil, nil
			}
			return []map[string]any{rowFor(key, s)}, nil
		}
	}

	usernameFilter := strings.ToLower(strings.TrimSpace(rq.String("username")))
	authOnly := rq.String("authenticated_only") == "true"

	type match struct {
		key string
		s   cache.Session
	}
	var matches []match

	var cursor uint64
	for {
		keys, next, serr := redisClient.Scan(ctx, cursor, "", 1000).Result()
		if serr != nil {
			return nil, serr
		}
		cursor = next
		for _, key := range keys {
			if !isSessionShape(key) {
				continue
			}
			data, gerr := redisClient.Get(ctx, key).Result()
			if gerr != nil {
				continue // vanished between SCAN and GET (expired) — skip, not an error
			}
			s, derr := cache.DecodeJson(data)
			if derr != nil {
				continue
			}
			if authOnly && s.UserID <= 0 {
				continue
			}
			if usernameFilter != "" && !strings.Contains(strings.ToLower(s.Username), usernameFilter) {
				continue
			}
			matches = append(matches, match{key: key, s: s})
		}
		if cursor == 0 {
			break
		}
	}

	sort.Slice(matches, func(i, j int) bool { return matches[i].s.LastAccess.After(matches[j].s.LastAccess) })
	if len(matches) > maxRows {
		matches = matches[:maxRows]
	}

	out := make([]map[string]any, 0, len(matches))
	for _, m := range matches {
		out = append(out, rowFor(m.key, m.s))
	}
	return out, nil
}

func rowFor(key string, s cache.Session) map[string]any {
	kind := "Web"
	if len(key) == 44 {
		kind = "Mobile/Bearer"
	}
	username := s.Username
	if username == "" {
		username = "(anonymous)"
	}
	return map[string]any{
		"id":          encodeID(key),
		"kind":        kind,
		"username":    username,
		"user_id":     s.UserID,
		"is_admin":    s.IsAdmin,
		"ip":          s.IP,
		"user_agent":  s.UserAgent,
		"last_access": s.LastAccess,
		"expire":      s.Expire,
	}
}

// revoke deletes the session outright — not auth.RevokeToken's leave-an-
// already-expired-placeholder trick (meant for a client invalidating its own
// token mid-request), since an admin explicitly removing a row here should
// actually remove it, not add a second zombie key.
func revoke(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	key, err := decodeID(id)
	if err != nil || key == "" {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	redisClient, err := rdb.GetInstance()
	if err != nil {
		http.Error(w, "redis unavailable", http.StatusInternalServerError)
		return
	}
	if err := redisClient.Del(r.Context(), key).Err(); err != nil {
		http.Error(w, "failed to delete session", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"id": id, "message": "Session revoked"})
}

func NewSessions() *Sessions {
	m := &Sessions{
		ModuleAbstract: &ModuleAbstract[any]{
			ID:                   "sessions",
			Name:                 "Sessions",
			Icon:                 "shield",
			Submenu:              "users",
			Description:          "Active web + mobile/bearer sessions, read straight out of Redis (capped to the 500 most recently active). Delete a row to revoke that session immediately.",
			Rights:               make(map[int]int),
			DefaultPermission:    PERMISSION_DENY,
			DefaultPermissionSet: true,
			OmitSystemFields:     SystemFieldsExceptID(),
			Overrides: HandlerOverrides{
				Create: readOnly,
				Edit:   readOnly,
				Delete: revoke,
			},
			DataFetch: fetchRows,
			NoTable:   true,
		},
	}
	m.ModuleAbstract.Fields = m.fieldset()
	m.ModuleAbstract.Filters = m.filters()
	return m
}

var Module *Sessions

func init() {
	app.RegisterModule(NewSessions(), "sessions")
}
