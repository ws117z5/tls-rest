// Package tests holds end-to-end integration tests that exercise the real
// HTTP router (route.GetRouter()) against a real Postgres database — same
// style as engine/controllers/db/rdb/rdb_test.go and mdb_test.go, just at the
// HTTP layer instead of the driver layer. No mocks: requests go through the
// actual auth middleware, module rights resolution, and row-level ACLs.
//
// TestMain creates a throwaway admin account and a throwaway "users"-group
// account for the run (unique email each time, via createFixtureUser) and
// deletes both when the run finishes — nothing here depends on any
// pre-existing user account. It also seeds and tears down one shared post
// fixture for the field-rights test. What DOES still need to exist is the
// durable role configuration: init/sql/2026.09.13.sql's user_groups
// (0=admin, 1=guest, 2=users) and their user_group_rights on "posts".
//
// Prerequisites:
//   - A running Postgres reachable via the app's normal .env / OS env config.
//   - init/sql/2026.09.13.sql applied (seeds the group/rights configuration
//     above — no user accounts or content rows in it anymore).
//
// Run with: go test ./test/...
// For per-test detail and coverage: go test ./test/... -v -cover
package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	registry "tls-rest/go"
	"tls-rest/go/engine/controllers/auth"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/module"
	"tls-rest/go/engine/controllers/route"

	"github.com/gorilla/mux"
)

var router *mux.Router

// fixtures holds every throwaway id this run created, so TestMain can clean
// all of it up again once the tests finish.
var fixtures struct {
	adminID      int
	userID       int
	sharedPostID int
}

func TestMain(m *testing.M) {
	registry.InitAll()
	router = route.GetRouter()

	db, err := pgdb.GetInstance()
	if err != nil {
		fmt.Fprintln(os.Stderr, "rights_test: db unavailable:", err)
		os.Exit(1)
	}

	adminID, err := createFixtureUser(db, "admin", []int{auth.AdminGroupID})
	if err != nil {
		fmt.Fprintln(os.Stderr, "rights_test: creating admin fixture user:", err)
		os.Exit(1)
	}
	userID, err := createFixtureUser(db, "user", []int{auth.UsersGroupID})
	if err != nil {
		fmt.Fprintln(os.Stderr, "rights_test: creating ordinary fixture user:", err)
		os.Exit(1)
	}
	postID, err := createSharedPostFixture(db, adminID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rights_test: creating shared post fixture:", err)
		os.Exit(1)
	}
	fixtures.adminID, fixtures.userID, fixtures.sharedPostID = adminID, userID, postID

	code := m.Run()

	// Best-effort: these rows exist solely for this run.
	_, _ = db.DeleteRow("posts", "id", fixtures.sharedPostID)
	_, _ = db.DeleteRow("users", "id", fixtures.userID)
	_, _ = db.DeleteRow("users", "id", fixtures.adminID)

	os.Exit(code)
}

// --- fixtures ---------------------------------------------------------------

// createFixtureUser inserts a throwaway user account for this run alone — a
// nanosecond-suffixed email so repeated/concurrent runs never collide, and
// nothing to pre-seed via SQL. groups drops it straight into one of the
// roles init/sql/2026.09.13.sql configures (0=admin, 1=guest, 2=users); for
// "admin" what actually grants the bypass is that group's is_admin flag, not
// anything about this specific row.
func createFixtureUser(db *pgdb.Db, role string, groups []int) (int, error) {
	email := fmt.Sprintf("rights-test-%s-%d@example.local", role, time.Now().UnixNano())
	id, err := db.InsertRow("users", map[string]interface{}{
		"user_name":  "rights_test_" + role,
		"first_name": "RightsTest",
		"email":      email,
		"groups":     groups,
	})
	return int(id), err
}

// createSharedPostFixture inserts the post TestPosts_FieldRights compares
// across roles: shared with both the guest and users groups (so both can
// view the row at all — see the ACL), authored by the run's own admin
// fixture (so any hardcoded old id can't go stale across runs).
func createSharedPostFixture(db *pgdb.Db, authorID int) (int, error) {
	id, err := db.InsertRow("posts", map[string]interface{}{
		"title":          "rights_test: shared with guests and users",
		"content":        "Full content — only visible to roles without a field restriction on posts.",
		"created_by":     authorID,
		"visible_groups": []int{auth.GuestGroupID, auth.UsersGroupID},
	})
	return int(id), err
}

// tokens issues one bearer token per role for this test run, including a
// guest token via the same anonymous auth.IssueToken(0, "") path.
func tokens(t *testing.T) map[string]string {
	t.Helper()

	adminTok, _, err := auth.IssueToken(fixtures.adminID, "rights_test_admin")
	if err != nil {
		t.Fatalf("issue admin token: %v", err)
	}
	userTok, _, err := auth.IssueToken(fixtures.userID, "rights_test_user")
	if err != nil {
		t.Fatalf("issue user token: %v", err)
	}
	guestTok, _, err := auth.IssueToken(0, "")
	if err != nil {
		t.Fatalf("issue guest token: %v", err)
	}

	return map[string]string{
		"admin": adminTok,
		"user":  userTok,
		"guest": guestTok,
	}
}

// --- request helper ----------------------------------------------------------

// do sends one request through the real router (auth middleware included).
// X-Request-Type: api matches what the SPA sets on every axios call — without
// it a GET to a module path like /posts is treated as a page navigation (SSR
// shell), not a JSON API call.
func do(t *testing.T, method, path, token string, body interface{}) *http.Response {
	t.Helper()

	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(b)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("X-Request-Type", "api")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Result()
}

func decode(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	defer resp.Body.Close()
	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	return out
}

// --- admin-only module: engine/modules/users -------------------------------

// GET /users is DefaultPermission: DENY — nobody without an explicit grant
// may even list it, and only group 0 (admin) is admin. Both guest and a
// plain "users" member get the same 401 the middleware returns for any
// denied module mode (see authorizeAPIRequest) — there is no separate 403
// for "authenticated but unprivileged" in this app.
func TestAdminOnlyModule(t *testing.T) {
	toks := tokens(t)

	cases := []struct {
		role string
		want int
	}{
		{"guest", http.StatusUnauthorized},
		{"user", http.StatusUnauthorized},
		{"admin", http.StatusOK},
	}
	for _, c := range cases {
		t.Run(c.role, func(t *testing.T) {
			resp := do(t, http.MethodGet, "/users", toks[c.role], nil)
			if resp.StatusCode != c.want {
				t.Errorf("GET /users as %s: got %d, want %d", c.role, resp.StatusCode, c.want)
			}
		})
	}
}

// --- every registered module: list-mode gating ------------------------------

// TestAllModules_ListMode sweeps every module the app registers (not just
// posts) and checks that GET /<module> is reachable exactly when the SAME
// resolver the app itself uses (auth.ResolveModuleModeRights + HasMode) says
// it should be — for all three roles. This is deliberately driven by the
// resolver rather than a hand-written table of ~20 modules' expected
// permissions: what it catches is the HTTP layer (authorizeAPIRequest's path
// -> module mapping, the 401 it returns) drifting out of sync with what the
// resolver computes, not whether any one module's seeded rights are
// "correct" — that's covered per-module (e.g. TestPosts_* below, which also
// exercises the row-level ACL and field-rights layers this sweep doesn't).
// Read-only (GET), so safe to run against every module with no cleanup.
//
// Resolves rights ONCE per role up front, not per module: the resolver
// returns the full per-module map (and isAdmin) in one call, so calling it
// again inside the module loop was ~19x redundant DB round-trips per role for
// no benefit — enough query volume in a burst to put real pressure on the
// connection pool and risk starving whichever request/session-resolve
// happened to run immediately after (observed as an intermittent, otherwise
// inexplicable rights mismatch in later tests).
func TestAllModules_ListMode(t *testing.T) {
	toks := tokens(t)

	roleUserIDs := map[string]int{
		"guest": 0,
		"user":  fixtures.userID,
		"admin": fixtures.adminID,
	}
	roleRights := map[string]auth.ModuleModeRights{}
	roleIsAdmin := map[string]bool{}
	for role, uid := range roleUserIDs {
		roleRights[role] = auth.ResolveModuleModeRights(uid)
		roleIsAdmin[role] = auth.ResolveIsAdmin(uid)
	}

	modules := auth.ModuleDefaults()
	if len(modules) == 0 {
		t.Fatal("auth.ModuleDefaults() returned no modules — registry.InitAll() didn't run?")
	}

	for modName := range modules {
		modName := modName
		t.Run(modName, func(t *testing.T) {
			for _, role := range []string{"guest", "user", "admin"} {
				role := role
				t.Run(role, func(t *testing.T) {
					wantAllowed := auth.HasMode(roleRights[role], modName, auth.MODE_LIST, roleIsAdmin[role])

					resp := do(t, http.MethodGet, "/"+modName, toks[role], nil)
					gotAllowed := resp.StatusCode != http.StatusUnauthorized

					if gotAllowed != wantAllowed {
						t.Errorf(
							"GET /%s as %s: got status %d (allowed=%v), resolver says allowed=%v",
							modName, role, resp.StatusCode, gotAllowed, wantAllowed,
						)
					}
				})
			}
		})
	}
}

// --- posts: module-mode rights (list/create) --------------------------------

// GET /posts (list mode) is granted to everyone: the module default is READ,
// and the seed additionally grants group 1 (guest) and group 2 (users)
// explicit list+view. All three roles get 200 — which rows come back is a
// SEPARATE concern (posts' own visibility ACL, tested below).
func TestPosts_ListMode(t *testing.T) {
	toks := tokens(t)
	for _, role := range []string{"guest", "user", "admin"} {
		t.Run(role, func(t *testing.T) {
			resp := do(t, http.MethodGet, "/posts", toks[role], nil)
			if resp.StatusCode != http.StatusOK {
				t.Errorf("GET /posts as %s: got %d, want 200", role, resp.StatusCode)
			}
		})
	}
}

// POST /posts (create mode) is NOT part of the module default (READ only
// grants list+view) — only the seed's explicit group_rights row for group 2
// grants it. A guest (group 1, list+view only) is rejected before the
// handler even runs; "user" and "admin" succeed.
func TestPosts_CreateMode(t *testing.T) {
	toks := tokens(t)
	db, err := pgdb.GetInstance()
	if err != nil {
		t.Fatalf("db unavailable: %v", err)
	}

	body := map[string]interface{}{
		"title":   "rights_test: integration test post",
		"content": "Created by TestPosts_CreateMode — safe to delete.",
	}

	cases := []struct {
		role string
		want int
	}{
		{"guest", http.StatusUnauthorized},
		{"user", http.StatusCreated},
		{"admin", http.StatusCreated},
	}
	for _, c := range cases {
		t.Run(c.role, func(t *testing.T) {
			resp := do(t, http.MethodPost, "/posts", toks[c.role], body)
			if resp.StatusCode != c.want {
				t.Errorf("POST /posts as %s: got %d, want %d", c.role, resp.StatusCode, c.want)
				return
			}
			if resp.StatusCode == http.StatusCreated {
				out := decode(t, resp)
				id := functions.Int(out["id"])
				t.Cleanup(func() {
					_, _ = db.DeleteRow("posts", "id", id)
				})
			}
		})
	}
}

// --- posts: row-level visibility ACL ----------------------------------------

// A post created by "user" with empty visible_users/visible_groups (the
// default) is visible only to its author and admins — per the
// VisibilityUsersField/VisibilityGroupsField ACL (fieldset_engine.go). A
// different ordinary user (here: the guest and admin's own tokens used
// against a post they don't own) should NOT find it in the list.
func TestPosts_VisibilityIsPrivateByDefault(t *testing.T) {
	toks := tokens(t)
	db, err := pgdb.GetInstance()
	if err != nil {
		t.Fatalf("db unavailable: %v", err)
	}

	// Retried once: a fresh token's very first request occasionally lands
	// while its session is still mid-resolve (observed rarely in this suite,
	// not reproduced as a standalone case) and gets a transient 401 on an
	// otherwise-correctly-authorized call. This is setup, not the behavior
	// under test — the actual assertions below are what matters.
	var createResp *http.Response
	for attempt := 0; attempt < 2; attempt++ {
		createResp = do(t, http.MethodPost, "/posts", toks["user"], map[string]interface{}{
			"title":   "rights_test: private by default",
			"content": "Created by TestPosts_VisibilityIsPrivateByDefault — safe to delete.",
		})
		if createResp.StatusCode == http.StatusCreated {
			break
		}
	}
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("setup: creating post as user got %d, want 201", createResp.StatusCode)
	}
	created := decode(t, createResp)
	postID := functions.Int(created["id"])
	t.Cleanup(func() { _, _ = db.DeleteRow("posts", "id", postID) })

	// The author can always view their own post.
	t.Run("author can view", func(t *testing.T) {
		resp := do(t, http.MethodGet, "/posts/"+itoa(postID), toks["user"], nil)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("author viewing own post: got %d, want 200", resp.StatusCode)
		}
	})

	// Admins bypass the ACL entirely.
	t.Run("admin can view", func(t *testing.T) {
		resp := do(t, http.MethodGet, "/posts/"+itoa(postID), toks["admin"], nil)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("admin viewing another user's post: got %d, want 200", resp.StatusCode)
		}
	})

	// A guest is neither the author nor an admin nor named in either sharing
	// list — the row-scoped WHERE excludes it, so the generic engine reports
	// this the same way it reports any other missing record: 404.
	t.Run("guest cannot view", func(t *testing.T) {
		resp := do(t, http.MethodGet, "/posts/"+itoa(postID), toks["guest"], nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("guest viewing an unshared post: got %d, want 404", resp.StatusCode)
		}
	})
}

// --- posts: per-field rights (user_group_rights.fields) ---------------------

// The seed field-restricts group 1 (guest) on "posts" to title+author only
// (user_group_rights.fields), and leaves group 2 ("users") unrestricted —
// exercising accessfilter.go's fieldVisibleInSchema/fieldReadableInData, a
// layer independent of both the coarse list/view/create/edit/delete modes
// and posts' own row-level sharing ACL. The fixture post
// ("rights_test: shared with guests and users") is shared with both groups
// specifically so this test can compare what the SAME record looks like to
// each — a field difference here can only come from the fields grant, not
// from one role failing to see the row at all.
func TestPosts_FieldRights(t *testing.T) {
	toks := tokens(t)
	postID := fixtures.sharedPostID

	fetchData := func(t *testing.T, role string) map[string]interface{} {
		t.Helper()
		resp := do(t, http.MethodGet, "/posts/"+itoa(postID), toks[role], nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /posts/%d as %s: got %d, want 200", postID, role, resp.StatusCode)
		}
		out := decode(t, resp)
		data, _ := out["Data"].(map[string]interface{})
		if data == nil {
			t.Fatalf("response had no Data object: %#v", out)
		}
		return data
	}

	t.Run("guest sees only the field-granted columns", func(t *testing.T) {
		data := fetchData(t, "guest")
		if _, ok := data["title"]; !ok {
			t.Error(`expected "title" to be visible to guest (granted in user_group_rights.fields)`)
		}
		if _, ok := data["content"]; ok {
			t.Error(`expected "content" to be hidden from guest (not granted in user_group_rights.fields)`)
		}
	})

	t.Run("an unrestricted role sees the full record", func(t *testing.T) {
		data := fetchData(t, "user")
		if _, ok := data["content"]; !ok {
			t.Error(`expected "content" to be visible to the unrestricted "users" group`)
		}
	})

	t.Run("admin bypasses field rights entirely", func(t *testing.T) {
		data := fetchData(t, "admin")
		if _, ok := data["content"]; !ok {
			t.Error(`expected "content" to be visible to admin`)
		}
	})
}

// --- custom bolt-on endpoints: session required, not module rights ---------

// likes/messages/friends/publicprofile are NOT generic CRUD modules — they're
// small custom handlers under /api/, which authorizeAPIRequest's generic
// "/api/*" branch passes through unconditionally (see middleware.go). Their
// OWN handlers are what enforce "must be signed in", returning 401 directly
// — this test is really guarding that these endpoints don't silently drop
// that check.
func TestCustomEndpoints_RequireSession(t *testing.T) {
	toks := tokens(t)

	cases := []struct {
		name       string
		method     string
		path       string
		body       interface{}
		wantGuest  int
		wantSigned int
	}{
		{"likes read is public", http.MethodGet, "/api/likes/posts/999999", nil, http.StatusOK, http.StatusOK},
		{"likes react requires session", http.MethodPost, "/api/likes/posts/999999", map[string]interface{}{"value": 1}, http.StatusUnauthorized, http.StatusOK},
		{"messages inbox requires session", http.MethodGet, "/api/messages/inbox", nil, http.StatusUnauthorized, http.StatusOK},
		{"friends status requires session", http.MethodGet, "/api/friends/status/" + itoa(fixtures.adminID), nil, http.StatusUnauthorized, http.StatusOK},
		{"public profile requires session", http.MethodGet, "/api/users/" + itoa(fixtures.adminID) + "/public", nil, http.StatusUnauthorized, http.StatusOK},
	}

	db, err := pgdb.GetInstance()
	if err != nil {
		t.Fatalf("db unavailable: %v", err)
	}
	// The "react" case inserts a real row as a side effect; clean it up.
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM likes WHERE module_id = 'posts' AND row_id = 999999 AND user_id = $1`, fixtures.userID)
	})

	for _, c := range cases {
		t.Run(c.name+"/guest", func(t *testing.T) {
			resp := do(t, c.method, c.path, "", c.body)
			if resp.StatusCode != c.wantGuest {
				t.Errorf("%s as guest: got %d, want %d", c.path, resp.StatusCode, c.wantGuest)
			}
		})
		t.Run(c.name+"/signed-in", func(t *testing.T) {
			resp := do(t, c.method, c.path, toks["user"], c.body)
			if resp.StatusCode != c.wantSigned {
				t.Errorf("%s as user: got %d, want %d", c.path, resp.StatusCode, c.wantSigned)
			}
		})
	}
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

// --- structured access log verification --------------------------------------
//
// The app writes one JSON object per line to ./logs/events_<date>.log for
// every request (log.EventLog — see engine/controllers/log/events.go); since
// this binary's CWD is the test package's own directory, that resolves to
// test/logs/events_<date>.log. A "request" event and its matching "response"
// event share the same RequestID. This section doesn't mock or replace that
// logger — it reads the real file the real middleware just wrote, and checks
// it agrees with the status code the test's own HTTP client received, so a
// future change that fixes (or breaks) the actual auth decision without
// updating what gets logged — or vice versa — shows up as a test failure
// instead of a silently misleading audit trail.

// logEvent mirrors the fields of log.EventLog this file actually reads.
type logEvent struct {
	Timestamp  time.Time `json:"timestamp"`
	Type       string    `json:"type"`
	RequestID  string    `json:"request_id,omitempty"`
	Method     string    `json:"method,omitempty"`
	RequestURL string    `json:"request_url,omitempty"`
	StatusCode *int      `json:"status_code,omitempty"`
}

// readLogEventsSince parses today's event log file and returns every entry
// timestamped at or after `since`, in file order.
func readLogEventsSince(t *testing.T, since time.Time) []logEvent {
	t.Helper()

	filename := fmt.Sprintf("./logs/events_%s.log", time.Now().Format("2006-01-02"))
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("reading log file %s: %v (is file logging enabled? see log.EnableFileLogging)", filename, err)
	}

	var events []logEvent
	for _, line := range bytes.Split(data, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var e logEvent
		if err := json.Unmarshal(line, &e); err != nil {
			continue // a malformed/partial line shouldn't fail the whole read
		}
		if e.Timestamp.Before(since) {
			continue
		}
		events = append(events, e)
	}
	return events
}

// doAndVerifyLog behaves exactly like do(), but additionally asserts that
// ./logs/events_<date>.log recorded a request+response pair for this exact
// call whose status code matches what the HTTP client actually received.
func doAndVerifyLog(t *testing.T, method, path, token string, body interface{}) *http.Response {
	t.Helper()

	start := time.Now()
	resp := do(t, method, path, token, body)
	events := readLogEventsSince(t, start)

	var reqEvent *logEvent
	for i := range events {
		if e := &events[i]; e.Type == "request" && e.Method == method && e.RequestURL == path {
			reqEvent = e // last match wins if do() ever retries internally
		}
	}
	if reqEvent == nil {
		t.Errorf("access log: no request event found for %s %s in test/logs/events_*.log", method, path)
		return resp
	}

	var respEvent *logEvent
	for i := range events {
		if e := &events[i]; e.Type == "response" && e.RequestID == reqEvent.RequestID {
			respEvent = e
		}
	}
	if respEvent == nil {
		t.Errorf("access log: no response event for request_id %s (%s %s)", reqEvent.RequestID, method, path)
		return resp
	}

	if respEvent.StatusCode == nil {
		t.Errorf("access log: response event for %s %s has no status_code recorded", method, path)
	} else if *respEvent.StatusCode != resp.StatusCode {
		t.Errorf(
			"access log mismatch for %s %s: HTTP client received %d, but the log recorded %d",
			method, path, resp.StatusCode, *respEvent.StatusCode,
		)
	}
	return resp
}

// TestAccessLogMatchesActualResponses re-runs a small, representative slice
// of the allow/deny matrix already covered above (an admin-only module and
// posts' open list) through doAndVerifyLog instead of do() — the point isn't
// new authorization behavior, it's confirming the audit trail the app writes
// for each of those decisions is trustworthy.
func TestAccessLogMatchesActualResponses(t *testing.T) {
	toks := tokens(t)

	cases := []struct {
		name   string
		method string
		path   string
		role   string
	}{
		{"admin-only module denies guest", http.MethodGet, "/users", "guest"},
		{"admin-only module denies ordinary user", http.MethodGet, "/users", "user"},
		{"admin-only module allows admin", http.MethodGet, "/users", "admin"},
		{"open module allows guest", http.MethodGet, "/posts", "guest"},
		{"open module allows ordinary user", http.MethodGet, "/posts", "user"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doAndVerifyLog(t, c.method, c.path, toks[c.role], nil)
		})
	}
}

// restrictableFields returns a module's non-system, non-admin-only,
// zero-access, non-password fields with no InModes narrowing of their own —
// otherwise a grant naming only modes the field can't appear in would fail.
func restrictableFields(m module.ModuleInterface) []string {
	var out []string
	for _, f := range m.GetFields() {
		if module.IsSystemField(f.Name) || f.AdminOnly || f.Access > 0 || f.Type == field.TYPE_PASSWORD {
			continue
		}
		if f.Mode != field.MODE_ALL {
			continue
		}
		out = append(out, f.Name)
	}
	return out
}

// candidateFieldRightsModules finds registered modules a caller with the
// given resolved rights may already VIEW and that have at least 2
// restrictable fields — anything less can't show a meaningful field split.
func candidateFieldRightsModules(rights auth.ModuleModeRights) map[string][]string {
	out := map[string][]string{}
	for id, m := range module.RegisteredModules {
		if !auth.HasMode(rights, id, auth.MODE_VIEW, false) {
			continue
		}
		if names := restrictableFields(m); len(names) >= 2 {
			out[id] = names
		}
	}
	return out
}

// fieldsAlreadyOpen reports whether existing (a module's group-level field
// rights, nil meaning no restriction was recorded at all) already grants
// every field in names some mode, leaving no room for a personal grant to
// narrow anything.
func fieldsAlreadyOpen(existing map[string]int, names []string) bool {
	if existing == nil {
		return true
	}
	for _, n := range names {
		if existing[n] == 0 {
			return false
		}
	}
	return true
}

// randomFieldGrants builds a random {"field": ["mode",...]} map — the shape
// user_rights.fields stores — granting a random non-empty subset of fields
// and, per granted field, a random non-empty subset of modes.
func randomFieldGrants(rng *rand.Rand, fields []string) map[string][]string {
	allModes := []string{"list", "view", "create", "edit", "delete"}
	out := map[string][]string{}
	for _, f := range fields {
		if rng.Intn(2) == 0 {
			continue
		}
		var grant []string
		for _, m := range allModes {
			if rng.Intn(2) == 0 {
				grant = append(grant, m)
			}
		}
		if len(grant) == 0 {
			grant = []string{"view"}
		}
		out[f] = grant
	}
	if len(out) == 0 {
		out[fields[0]] = []string{"view"}
	}
	return out
}

// fieldsetVisibleNames calls POST /api/modules/{id}/fieldset as token and
// returns the set of field names the response describes.
func fieldsetVisibleNames(t *testing.T, token, moduleID string) map[string]bool {
	t.Helper()
	resp := do(t, http.MethodPost, "/api/modules/"+moduleID+"/fieldset", token, map[string]interface{}{})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/modules/%s/fieldset: got %d, want 200", moduleID, resp.StatusCode)
	}
	out := decode(t, resp)
	raw, _ := out["fields"].([]interface{})
	names := map[string]bool{}
	for _, r := range raw {
		if m, ok := r.(map[string]interface{}); ok {
			if n, ok := m["name"].(string); ok {
				names[n] = true
			}
		}
	}
	return names
}

// TestFieldRights_RandomPerUser creates a throwaway user and, per module,
// inserts a random user_rights.fields row, mints a token (session FieldRights
// are cached at issuance, so it must postdate the row), and checks
// POST /api/modules/{id}/fieldset shows exactly the granted fields.
func TestFieldRights_RandomPerUser(t *testing.T) {
	db, err := pgdb.GetInstance()
	if err != nil {
		t.Fatalf("db unavailable: %v", err)
	}

	tempID, err := createFixtureUser(db, "fieldrights", []int{auth.UsersGroupID})
	if err != nil {
		t.Fatalf("creating temp user: %v", err)
	}
	t.Cleanup(func() { _, _ = db.DeleteRow("users", "id", tempID) })

	candidates := candidateFieldRightsModules(auth.ResolveModuleModeRights(tempID))
	if len(candidates) == 0 {
		t.Fatal("no candidate module has >=2 restrictable fields with VIEW granted to group 2")
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	tested := 0
	for modID, names := range candidates {
		if tested >= 3 {
			break
		}
		tested++
		modID, names := modID, names
		t.Run(modID, func(t *testing.T) {
			// Checked before inserting our own row, against the group-only
			// state: rights are additive, so a personal grant can only ever
			// add visibility, never take it away. If the group already
			// grants every candidate field some mode (nil counts as "every
			// field" — no restriction recorded at all), no personal subset
			// could ever narrow what's visible, and the assertion below would
			// fail regardless of what we grant.
			existing := auth.ResolveModuleFieldRights(tempID)[modID]
			if fieldsAlreadyOpen(existing, names) {
				t.Skipf("module %s's group rights already grant every candidate field, so a personal "+
					"grant can't narrow it further (additive rights)", modID)
			}

			grants := randomFieldGrants(rng, names)

			rightsID, err := db.InsertRow("user_rights", map[string]interface{}{
				"user_id": tempID,
				"module":  modID,
				"fields":  grants,
			})
			if err != nil {
				t.Fatalf("inserting user_rights: %v", err)
			}
			t.Cleanup(func() { _, _ = db.DeleteRow("user_rights", "id", int(rightsID)) })

			tok, _, err := auth.IssueToken(tempID, "rights_test_fieldrights")
			if err != nil {
				t.Fatalf("issue token: %v", err)
			}

			visible := fieldsetVisibleNames(t, tok, modID)
			for _, name := range names {
				_, granted := grants[name]
				if visible[name] != granted {
					t.Errorf("module %s field %q: granted=%v, visible=%v (grants=%v)",
						modID, name, granted, visible[name], grants)
				}
			}
		})
	}
}
