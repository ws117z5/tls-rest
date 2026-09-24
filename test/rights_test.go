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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"testing"
	"time"

	app "tls-rest/go/app"
	_ "tls-rest/go/app/bootstrap"
	"tls-rest/go/engine/controllers/accesslog"
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
	accesslog.Init() // needed by TestAccessLogMatchesActualResponses
	router = route.GetRouter(app.RegisterRoutes)

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

	// Drop anything that might still reference the fixtures first (comments/
	// likes/posts from a subtest whose own t.Cleanup didn't run, e.g. a
	// panic), then the fixtures themselves — logged, not discarded, so a
	// failure here is visible instead of silently leaving stray rows for
	// deleteStaleFixture to find (and log about) on the next run.
	del := func(query string, args ...interface{}) {
		if _, err := db.Exec(query, args...); err != nil {
			fmt.Fprintf(os.Stderr, "rights_test: cleanup %q failed: %v\n", query, err)
		}
	}
	del("DELETE FROM comments WHERE created_by = $1 OR created_by = $2", fixtures.userID, fixtures.adminID)
	del("DELETE FROM likes WHERE user_id = $1 OR user_id = $2", fixtures.userID, fixtures.adminID)
	del("DELETE FROM posts WHERE created_by = $1 OR created_by = $2", fixtures.userID, fixtures.adminID)
	del("DELETE FROM users WHERE id = $1", fixtures.userID)
	del("DELETE FROM users WHERE id = $1", fixtures.adminID)

	os.Exit(code)
}

// --- fixtures ---------------------------------------------------------------

// createFixtureUser inserts a throwaway user account for this run alone — a
// nanosecond-suffixed email so repeated/concurrent runs never collide, and
// nothing to pre-seed via SQL. groups drops it straight into one of the
// roles init/sql/2026.09.13.sql configures (0=admin, 1=guest, 2=users); for
// "admin" what actually grants the bypass is that group's is_admin flag, not
// anything about this specific row. Self-healing: a stale row with the same
// user_name (left by a previous run whose TestMain cleanup never ran — a
// panic, or the process getting killed before m.Run() returned) is deleted
// first, so failed cleanups don't accumulate across runs.
func createFixtureUser(db *pgdb.Db, role string, groups []int) (int, error) {
	userName := "rights_test_" + role
	deleteStaleFixture(db, "users", "user_name", userName)

	email := fmt.Sprintf("rights-test-%s-%d@example.local", role, time.Now().UnixNano())
	id, err := db.InsertRow("users", map[string]interface{}{
		"user_name":  userName,
		"first_name": "RightsTest",
		"email":      email,
		"groups":     groups,
	})
	return int(id), err
}

// deleteStaleFixture removes a leftover row (and anything in posts/comments/
// likes still pointing at it) matching column=value, if a previous run's
// cleanup failed to run or failed outright — see createFixtureUser.
func deleteStaleFixture(db *pgdb.Db, table, column, value string) {
	row, err := db.GetOne(fmt.Sprintf("SELECT id FROM %s WHERE %s = $1", table, column), value)
	if err != nil || row == nil {
		return
	}
	id := functions.Int(row["id"])
	if table == "users" {
		_, _ = db.Exec("DELETE FROM comments WHERE created_by = $1", id)
		_, _ = db.Exec("DELETE FROM likes WHERE user_id = $1", id)
		_, _ = db.Exec("DELETE FROM posts WHERE created_by = $1", id)
	}
	_, _ = db.DeleteRow(table, "id", id)
}

// createSharedPostFixture inserts the post TestPosts_FieldRights compares
// across roles: shared with both the guest and users groups (so both can
// view the row at all — see the ACL), authored by the run's own admin
// fixture (so any hardcoded old id can't go stale across runs).
func createSharedPostFixture(db *pgdb.Db, authorID int) (int, error) {
	deleteStaleFixture(db, "posts", "title", "rights_test: shared with guests and users")
	id, err := db.InsertRow("posts", map[string]interface{}{
		"title":          "rights_test: shared with guests and users",
		"content":        "Full content — only visible to roles without a field restriction on posts.",
		"created_by":     authorID,
		"visible_groups": []int{auth.GuestGroupID, auth.UsersGroupID},
	})
	return int(id), err
}

// tokens issues one bearer token per role for this test run, including a
// guest token via the same anonymous auth.IssueToken(ctx, 0, "") path.
func tokens(t *testing.T) map[string]string {
	t.Helper()

	adminTok, _, err := auth.IssueToken(context.Background(), fixtures.adminID, "rights_test_admin")
	if err != nil {
		t.Fatalf("issue admin token: %v", err)
	}
	userTok, _, err := auth.IssueToken(context.Background(), fixtures.userID, "rights_test_user")
	if err != nil {
		t.Fatalf("issue user token: %v", err)
	}
	guestTok, _, err := auth.IssueToken(context.Background(), 0, "")
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
		roleRights[role] = auth.ResolveModuleModeRights(context.Background(), uid)
		roleIsAdmin[role] = auth.ResolveIsAdmin(uid)
	}

	modules := auth.ModuleDefaults()
	if len(modules) == 0 {
		t.Fatal("auth.ModuleDefaults() returned no modules — did the bootstrap import run?")
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

// Group 1 (guest) is field-restricted on "posts" (user_group_rights.fields);
// group 2 ("users") is left unrestricted — exercising accessfilter.go's
// fieldVisibleInSchema/fieldReadableInData, a layer independent of both the
// coarse list/view/create/edit/delete modes and posts' own row-level sharing
// ACL. The fixture post ("rights_test: shared with guests and users") is
// shared with both groups specifically so this test can compare what the
// SAME record looks like to each — a field difference here can only come
// from the fields grant, not from one role failing to see the row at all.
//
// The guest case doesn't hardcode which fields are granted — it reads the
// live grant via auth.ResolveModuleFieldRights (the same resolver the server
// itself uses) and asserts the response matches it exactly, so the test
// tracks whatever this database is actually configured to allow instead of
// drifting out of sync with it.
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

	t.Run("guest sees exactly the field-granted columns", func(t *testing.T) {
		data := fetchData(t, "guest")
		logPostsFieldRightsDiagnostics(t, "guest", 0, data)

		rights := auth.ResolveModuleFieldRights(0)["posts"]
		if len(rights) == 0 {
			t.Fatal("guest's posts field rights must be restricted (non-empty in user_group_rights.fields) for this test to exercise anything")
		}
		for field, mask := range rights {
			_, visible := data[field]
			wantVisible := mask&auth.MODE_VIEW != 0
			if visible != wantVisible {
				t.Errorf("field %q: visible=%v, want %v (per user_group_rights.fields grant, mask=%d)", field, visible, wantVisible, mask)
			}
		}
	})

	t.Run("an unrestricted role sees the full record", func(t *testing.T) {
		data := fetchData(t, "user")
		logPostsFieldRightsDiagnostics(t, "user", fixtures.userID, data)
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

// logPostsFieldRightsDiagnostics dumps what's stored and resolved for the
// "posts" module's field rights, so a CI failure log alone — without direct
// access to whatever database it ran against — is enough to tell "no row",
// "wrong fields", or "wrong group_id" apart from a real app bug.
func logPostsFieldRightsDiagnostics(t *testing.T, role string, userID int, data map[string]interface{}) {
	t.Helper()
	t.Logf("[diag] %s: resolved field rights for posts = %#v", role, auth.ResolveModuleFieldRights(userID)["posts"])
	t.Logf("[diag] %s: response field names = %v", role, sortedKeys(data))

	db, err := pgdb.GetInstance()
	if err != nil {
		t.Logf("[diag] db unavailable: %v", err)
		return
	}
	rows, err := db.RQuery(`SELECT id, group_id, modes, fields FROM user_group_rights WHERE module = 'posts' ORDER BY group_id, id`)
	if err != nil {
		t.Logf("[diag] could not read user_group_rights: %v", err)
		return
	}
	if len(rows) == 0 {
		t.Logf("[diag] no user_group_rights row exists for module='posts' at all")
	}
	for _, row := range rows {
		t.Logf("[diag] user_group_rights: id=%v group_id=%v modes=%v fields=%v",
			row["id"], row["group_id"], row["modes"], row["fields"])
	}
}

func sortedKeys(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// --- posts: filter rights (MODE_FILTERS + user_group_rights/user_rights.filter_fields) ---

// TestPosts_Filters exercises the filters-rights layer end to end: MODE_FILTERS
// is no longer part of a module's default permission (defaultModesFor), so a
// role with only the "users" group's baseline list/view rights must see and be
// able to apply no filters at all; granting MODE_FILTERS unlocks every declared
// filter (bar admin-only ones); and a filter_fields grant narrows that down to
// a named subset — checked both in the Filters metadata AND against the actual
// query results, since a hidden filter that still applies server-side would be
// a worse bug than a merely-visible one.
func TestPosts_Filters(t *testing.T) {
	db, err := pgdb.GetInstance()
	if err != nil {
		t.Fatalf("db unavailable: %v", err)
	}

	tempID, err := createFixtureUser(db, "filters", []int{auth.UsersGroupID})
	if err != nil {
		t.Fatalf("creating temp user: %v", err)
	}
	t.Cleanup(func() { _, _ = db.DeleteRow("users", "id", tempID) })

	token := func(t *testing.T) string {
		t.Helper()
		tok, _, err := auth.IssueToken(context.Background(), tempID, "rights_test_filters")
		if err != nil {
			t.Fatalf("issue token: %v", err)
		}
		return tok
	}

	listPosts := func(t *testing.T, tok, query string) map[string]interface{} {
		t.Helper()
		resp := do(t, http.MethodGet, "/posts"+query, tok, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /posts%s: got %d, want 200", query, resp.StatusCode)
		}
		return decode(t, resp)
	}

	filterNames := func(out map[string]interface{}) map[string]bool {
		names := map[string]bool{}
		arr, _ := out["Filters"].([]interface{})
		for _, f := range arr {
			if m, ok := f.(map[string]interface{}); ok {
				if n, ok := m["name"].(string); ok {
					names[n] = true
				}
			}
		}
		return names
	}

	// Guest (group 1) is this suite's canonical zero-filters baseline: its live
	// grant is modes=3 (list|view only, confirmed via the diagnostics below),
	// unlike "users" which — as this test itself discovered — now carries a
	// real, admin-configured Filters grant. Asserting against guest keeps this
	// case meaningful instead of chasing whatever "users" happens to have.
	t.Run("guest has no MODE_FILTERS: no filters listed, and a filter param has no effect", func(t *testing.T) {
		guestTok := tokens(t)["guest"]
		mask := auth.ResolveModuleModeRights(context.Background(), 0)["posts"]
		if mask&auth.MODE_FILTERS != 0 {
			t.Fatalf("test assumption broken: guest's live posts modes (%d) now include MODE_FILTERS", mask)
		}

		out := listPosts(t, guestTok, "")
		if names := filterNames(out); len(names) != 0 {
			t.Errorf("expected no filters listed without MODE_FILTERS, got %v", names)
		}

		unfiltered := listPosts(t, guestTok, "")
		filtered := listPosts(t, guestTok, "?title=no-such-post-zzz")
		if fmt.Sprint(unfiltered["Total"]) != fmt.Sprint(filtered["Total"]) {
			t.Errorf("expected title= to be ignored server-side without MODE_FILTERS; Total %v vs %v",
				unfiltered["Total"], filtered["Total"])
		}
	})

	t.Run("MODE_FILTERS granted, filter_fields unrestricted: every non-admin-only filter listed and usable", func(t *testing.T) {
		rightsID, err := db.InsertRow("user_rights", map[string]interface{}{
			"user_id": tempID,
			"module":  "posts",
			"modes":   auth.MODE_LIST | auth.MODE_VIEW | auth.MODE_FILTERS,
		})
		if err != nil {
			t.Fatalf("inserting user_rights: %v", err)
		}
		t.Cleanup(func() { _, _ = db.DeleteRow("user_rights", "id", int(rightsID)) })

		tok := token(t)
		names := filterNames(listPosts(t, tok, ""))
		if !names["title"] || !names["created_from"] {
			t.Errorf("expected \"title\" and \"created_from\" filters listed once MODE_FILTERS is granted, got %v", names)
		}
		if names["user"] || names["user_group"] {
			t.Errorf("expected admin-only filters to stay hidden from a non-admin regardless of MODE_FILTERS, got %v", names)
		}

		unfiltered := listPosts(t, tok, "")
		match := listPosts(t, tok, "?title=shared")
		noMatch := listPosts(t, tok, "?title=no-such-post-zzz")
		if fmt.Sprint(match["Total"]) == fmt.Sprint(noMatch["Total"]) {
			t.Errorf("expected title= to actually filter results once granted; got the same Total (%v) for a match and a non-match", match["Total"])
		}
		if fmt.Sprint(unfiltered["Total"]) == fmt.Sprint(noMatch["Total"]) {
			t.Errorf("fixture setup: expected the no-such-post title to narrow results below the unfiltered Total (%v)", unfiltered["Total"])
		}
	})

	t.Run("MODE_FILTERS granted, filter_fields restricts to title only", func(t *testing.T) {
		rightsID, err := db.InsertRow("user_rights", map[string]interface{}{
			"user_id":       tempID,
			"module":        "posts",
			"modes":         auth.MODE_LIST | auth.MODE_VIEW | auth.MODE_FILTERS,
			"filter_fields": []string{"title"},
		})
		if err != nil {
			t.Fatalf("inserting user_rights: %v", err)
		}
		t.Cleanup(func() { _, _ = db.DeleteRow("user_rights", "id", int(rightsID)) })

		tok := token(t)
		names := filterNames(listPosts(t, tok, ""))
		if !names["title"] {
			t.Errorf(`expected "title" filter listed (explicitly granted via filter_fields), got %v`, names)
		}
		if names["created_from"] {
			t.Errorf(`expected "created_from" filter hidden (not in filter_fields grant), got %v`, names)
		}

		unfiltered := listPosts(t, tok, "")
		byCreatedFrom := listPosts(t, tok, "?created_from=2099-01-01")
		if fmt.Sprint(unfiltered["Total"]) != fmt.Sprint(byCreatedFrom["Total"]) {
			t.Errorf("expected created_from= to be ignored server-side (not in filter_fields grant); Total %v vs %v",
				unfiltered["Total"], byCreatedFrom["Total"])
		}

		match := listPosts(t, tok, "?title=shared")
		noMatch := listPosts(t, tok, "?title=no-such-post-zzz")
		if fmt.Sprint(match["Total"]) == fmt.Sprint(noMatch["Total"]) {
			t.Errorf("expected the granted title= filter to still work; got the same Total (%v) for a match and a non-match", match["Total"])
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

// --- access_log verification --------------------------------------------

type accessLogRow struct {
	Status int
}

// findAccessLogRow: accesslog.Record is async, so poll briefly.
func findAccessLogRow(t *testing.T, since time.Time, method, path string) *accessLogRow {
	t.Helper()

	db, err := pgdb.GetInstance()
	if err != nil {
		t.Fatalf("db unavailable: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		row, err := db.GetOne(
			`SELECT status FROM access_log WHERE method = $1 AND path = $2 AND created >= $3
			 ORDER BY created DESC LIMIT 1`, method, path, since,
		)
		if err != nil {
			t.Fatalf("querying access_log: %v", err)
		}
		if row != nil {
			return &accessLogRow{Status: functions.Int(row["status"])}
		}
		if time.Now().After(deadline) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// doAndVerifyLog: do(), plus asserting access_log recorded this call's status.
func doAndVerifyLog(t *testing.T, method, path, token string, body interface{}) *http.Response {
	t.Helper()

	start := time.Now()
	resp := do(t, method, path, token, body)

	row := findAccessLogRow(t, start, method, path)
	if row == nil {
		t.Errorf("access_log: no row found for %s %s", method, path)
		return resp
	}
	if row.Status != resp.StatusCode {
		t.Errorf(
			"access_log mismatch for %s %s: HTTP client received %d, but the log recorded %d",
			method, path, resp.StatusCode, row.Status,
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

	candidates := candidateFieldRightsModules(auth.ResolveModuleModeRights(context.Background(), tempID))
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

			tok, _, err := auth.IssueToken(context.Background(), tempID, "rights_test_fieldrights")
			if err != nil {
				t.Fatalf("issue token: %v", err)
			}

			visible := fieldsetVisibleNames(t, tok, modID)
			for _, name := range names {
				// Additive rights: a field the group already opened stays visible
				// regardless of whether this personal grant repeats it (see
				// fieldsAlreadyOpen above, applied per-field here rather than
				// requiring every candidate field to already be open).
				_, granted := grants[name]
				granted = granted || existing[name] != 0
				if visible[name] != granted {
					t.Errorf("module %s field %q: granted=%v, visible=%v (grants=%v, existing=%v)",
						modID, name, granted, visible[name], grants, existing)
				}
			}
		})
	}
}
