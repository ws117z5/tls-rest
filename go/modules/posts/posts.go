package posts

import (
	"context"
	"net/http"

	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/modules/html"

	. "tls-rest/go/engine/controllers/field"
	. "tls-rest/go/engine/controllers/module"

	"github.com/gorilla/mux"
)

// shareableUsers lists every user as a {value, name} option for the
// visible_users sharing table. Simple and unfiltered — sharing your own post
// with someone doesn't need the authority scoping a group-grant does.
func shareableUsers(ctx context.Context, viewer map[string]interface{}) []map[string]interface{} {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return nil
	}
	rows, err := db.GetAll(`SELECT id AS value, user_name AS name FROM users ORDER BY user_name`)
	if err != nil {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]interface{}{"value": r["value"], "name": functions.Coerce[string](r["name"])})
	}
	return out
}

// shareableGroups lists every user group as a {value, name} option for the
// visible_groups sharing table.
func shareableGroups(ctx context.Context, viewer map[string]interface{}) []map[string]interface{} {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return nil
	}
	rows, err := db.GetAll(`SELECT id AS value, name FROM user_groups ORDER BY name`)
	if err != nil {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]interface{}{"value": r["value"], "name": functions.Coerce[string](r["name"])})
	}
	return out
}

// existingTags suggests tags already used on other posts, for the tags field's autocomplete.
func existingTags(ctx context.Context, input string, _ map[string]interface{}) []AutoOption {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return nil
	}
	rows, err := db.GetAll(`
		SELECT DISTINCT tag FROM (
			SELECT jsonb_array_elements_text(tags) AS tag FROM posts
		) t WHERE tag ILIKE $1 ORDER BY tag LIMIT 20`, "%"+input+"%")
	if err != nil {
		return nil
	}
	out := make([]AutoOption, 0, len(rows))
	for _, r := range rows {
		tag := functions.Coerce[string](r["tag"])
		out = append(out, AutoOption{Value: tag, Label: tag})
	}
	return out
}

// fieldset defines the module's fields. It is the field-set counterpart to
// filters() in filters.go: both build a set that the fieldset engine consumes.
func (p *Posts) fieldset() []Field {
	return []Field{

		NewField("title", TYPE_STRING, true).
			WithLabel("Title").
			WithDescription("Post title").
			WithValidation("minLength", 3).
			WithValidation("maxLength", 200).
			WithOption("width", "600px"),

		NewField("author", TYPE_STRING, true).
			WithLabel("Author").
			WithValidation("minLength", 3).
			WithValidation("maxLength", 200).
			WithOption("width", "600px").
			WithSQL("(SELECT CONCAT(first_name, ' ', last_name) FROM users WHERE users.id = posts.created_by LIMIT 1)").
			AsVirtual(),

		// Optional, unique; when set the post is also reachable at /posts/<slug> (see viewByRef in routes.go).
		NewField("slug", TYPE_STRING, false).
			WithLabel("Slug").
			WithDescription("Optional; lets the post be opened at /posts/<slug> instead of /posts/<id>").
			WithValidation("pattern", "^[a-z0-9]+(-[a-z0-9]+)*$").
			WithValidation("patternMessage", "lowercase letters, numbers and hyphens only").
			WithOption("width", "300px"),

		NewField("tags", TYPE_LABEL, false).
			WithLabel("Tags").
			WithDescription("Type a tag and press Enter to add it").
			WithAutocomplete(map[string]interface{}{"function": existingTags}).
			NonSortable().
			NonSearchable(),

		NewField("images", TYPE_IMAGE, false).
			WithLabel("Images").
			WithDescription("Post images").
			WithOption("multiple", true).
			WithOption("folderTemplate", "posts/{title}").
			NonSortable().
			NonSearchable().
			WithResize(ResizeOptions{Width: 1000}),

		// List-view counters, computed at read time — no stored column.
		NewField("likes_count", TYPE_INT, false).
			WithLabel("Likes").
			WithSQL("(SELECT COUNT(*) FROM likes WHERE likes.module_id = 'posts' AND likes.row_id = posts.id AND likes.value = 1)").
			AsVirtual().
			AsReadOnly().
			InModes(MODE_LIST | MODE_VIEW),

		// Recursive: a reply's module_id/row_id point at its parent comment, not
		// the post, so a plain WHERE would miss every reply. Matches the total
		// CommentsThread shows on the post's own page (comments module's countAll).
		NewField("comments_count", TYPE_INT, false).
			WithLabel("Comments").
			WithSQL(`(WITH RECURSIVE thread AS (
				SELECT c.id FROM comments c WHERE c.module_id = 'posts' AND c.row_id = posts.id
			  UNION ALL
				SELECT c.id FROM comments c JOIN thread t ON c.module_id = 'comments' AND c.row_id = t.id
			) SELECT COUNT(*) FROM thread)`).
			AsVirtual().
			AsReadOnly().
			InModes(MODE_LIST | MODE_VIEW),

		NewField("content", TYPE_MARKDOWN, true).
			WithLabel("Content").
			WithDescription("Post content").
			WithValidation("minLength", 10).
			WithOption("width", "600px").
			WithOption("height", "300px").
			NonSortable(),

		// content's markdown is compiled server-side on save (AfterFieldset below)
		// into the shared html module (html_id); view mode renders compiled_html.
		NewHTMLField("compiled_html").
			WithLabel("Content (HTML)").
			InModes(MODE_VIEW),

		// Sharing lists: who besides you and admins may see this post. Empty
		// (the default) means private to you and admins — see VisibilityUsersField
		// / VisibilityGroupsField below, which is what actually enforces this.
		// Not shown at create time — a post is shared after it exists.
		NewField("visible_users", TYPE_TABLE, false).
			WithLabel("Visible To (Users)").
			WithDescription("Specific users who may also view this post, besides you and admins").
			WithOption("width", "500px").
			InModes(MODE_VIEW | MODE_EDIT | MODE_SUBMIT).
			TableFieldset([]Field{
				NewField("user", TYPE_INT, true).
					WithLabel("User").
					WithOption("widget", "select").
					WithOption("width", "300px").
					WithOptionsCtx(shareableUsers),
			}).
			TableRowsAddable("user").
			TableData(func(ctx context.Context, data map[string]interface{}) []map[string]interface{} {
				pid := functions.Int(data["id"])
				if pid <= 0 {
					return nil
				}
				db, err := pgdb.GetInstanceCtx(ctx)
				if err != nil {
					return nil
				}
				rows, err := db.GetAll(
					`SELECT jsonb_array_elements_text(visible_users)::int AS "user" FROM posts WHERE id = $1`,
					pid)
				if err != nil {
					return nil
				}
				return rows
			}).
			TableOnSubmit(func(rows []map[string]interface{}) interface{} {
				ids := []interface{}{}
				for _, r := range rows {
					if id := functions.Int(r["user"]); id != -1 {
						ids = append(ids, id)
					}
				}
				return ids
			}),

		NewField("visible_groups", TYPE_TABLE, false).
			WithLabel("Visible To (Groups)").
			WithDescription("User groups who may also view this post, besides you and admins").
			WithOption("width", "500px").
			InModes(MODE_VIEW | MODE_EDIT | MODE_SUBMIT).
			TableFieldset([]Field{
				NewField("group", TYPE_INT, true).
					WithLabel("Group").
					WithOption("widget", "select").
					WithOption("width", "300px").
					WithOptionsCtx(shareableGroups),
			}).
			TableRowsAddable("group").
			TableData(func(ctx context.Context, data map[string]interface{}) []map[string]interface{} {
				pid := functions.Int(data["id"])
				if pid <= 0 {
					return nil
				}
				db, err := pgdb.GetInstanceCtx(ctx)
				if err != nil {
					return nil
				}
				rows, err := db.GetAll(
					`SELECT jsonb_array_elements_text(visible_groups)::int AS "group" FROM posts WHERE id = $1`,
					pid)
				if err != nil {
					return nil
				}
				return rows
			}).
			TableOnSubmit(func(rows []map[string]interface{}) interface{} {
				ids := []interface{}{}
				for _, r := range rows {
					if id := functions.Int(r["group"]); id != -1 {
						ids = append(ids, id)
					}
				}
				return ids
			}),

		// Additional visibility toggles, on top of the sharing lists above —
		// see VisibilityAnyoneField/VisibilityFriendsField/
		// VisibilityBlockedOverrideField's own doc (module.go) for exactly how
		// each is combined.
		NewField("visible_anyone", TYPE_CHECKBOX, false).
			WithLabel("Visible To Anyone").
			WithDescription("Make this post public, besides the specific sharing above"),
		NewField("visible_friends", TYPE_CHECKBOX, false).
			WithLabel("Visible To Friends").
			WithDescription("Let your friends see this post, besides the specific sharing above"),
		NewField("visible_to_blocked", TYPE_CHECKBOX, false).
			WithLabel("Visible To Blocked Users").
			WithDescription("Let users you've blocked see this post anyway (by default they're excluded even if otherwise visible to them)"),

		// Hide list: the inverse of visible_users — specific users this post
		// is hidden from no matter what (even "Visible To Anyone"). Same
		// TableData/TableOnSubmit shape as visible_users above, just reading
		// from/writing to hide_users instead.
		NewField("hide_users", TYPE_TABLE, false).
			WithLabel("Hide From (Users)").
			WithDescription("Specific users who may never view this post, even if it's otherwise visible to them").
			WithOption("width", "500px").
			InModes(MODE_VIEW | MODE_EDIT | MODE_SUBMIT).
			TableFieldset([]Field{
				NewField("user", TYPE_INT, true).
					WithLabel("User").
					WithOption("widget", "select").
					WithOption("width", "300px").
					WithOptionsCtx(shareableUsers),
			}).
			TableRowsAddable("user").
			TableData(func(ctx context.Context, data map[string]interface{}) []map[string]interface{} {
				pid := functions.Int(data["id"])
				if pid <= 0 {
					return nil
				}
				db, err := pgdb.GetInstanceCtx(ctx)
				if err != nil {
					return nil
				}
				rows, err := db.GetAll(
					`SELECT jsonb_array_elements_text(hide_users)::int AS "user" FROM posts WHERE id = $1`,
					pid)
				if err != nil {
					return nil
				}
				return rows
			}).
			TableOnSubmit(func(rows []map[string]interface{}) interface{} {
				ids := []interface{}{}
				for _, r := range rows {
					if id := functions.Int(r["user"]); id != -1 {
						ids = append(ids, id)
					}
				}
				return ids
			}),
	}
}

// NewPosts creates a new Posts module instance
func NewPosts() *Posts {
	m := &Posts{
		ModuleAbstract: &ModuleAbstract[interface{}]{
			ID:     "posts",
			Name:   "Posts",
			Icon:   "posts",
			Rights: make(map[int]int),
			// Public module: everyone may read (list/view); writes require rights.
			DefaultPermission:    1, // PERMISSION_READ
			DefaultPermissionSet: true,
			// Sharing-list visibility (fieldset.go's visible_users/visible_groups):
			// a non-admin sees a post only if they wrote it, or are named in one
			// of the two sharing lists. A post shared with nobody is private to
			// its author and admins — this replaces the generic access-level gate
			// for this module entirely (see buildVisibilityCondition).
			VisibilityUsersField:  "visible_users",
			VisibilityGroupsField: "visible_groups",
			// See module.go's own doc on each of these for exactly how they
			// combine with the sharing lists above.
			VisibilityAnyoneField:          "visible_anyone",
			VisibilityFriendsField:         "visible_friends",
			VisibilityBlockedOverrideField: "visible_to_blocked",
			HideUsersField:                 "hide_users",
			// GET /posts/<ref>: a numeric ref is the id, same as always; anything else is looked up as a slug
			// and rewritten to its id (see viewByRef in routes.go) before the normal, rights-checked View runs.
			// Registered ahead of the engine's own GET /{id}, so it replaces that route entirely for posts.
			CustomRoutes: []CustomRoute{
				{Path: "/{ref}", Methods: []string{"GET"}, Handler: viewByRef},
			},
		},
	}

	// Build the field and filter sets. fieldset() defines the columns/form
	// fields; filters() (in filters.go) defines the list-mode filters. Default
	// system fields are added automatically by Initialize().
	m.ModuleAbstract.Fields = m.fieldset()
	m.ModuleAbstract.Filters = m.filters()

	// The post view renders a comments thread (engine/modules/comments) via its
	// custom layout, talking to the /api/comments REST API directly.

	// content is a normal stored field, so filterValidFields keeps it in data —
	// compile it server-side and upsert the result into the shared html module,
	// setting html_id so view mode renders compiled_html, not client-side markdown.
	m.AfterFieldset = func(r *http.Request, data map[string]interface{}) (map[string]interface{}, error) {
		markdown, ok := data["content"].(string)
		if !ok {
			return data, nil
		}

		db, err := pgdb.GetInstanceCtx(r.Context())
		if err != nil {
			return nil, err
		}

		var existingID int64
		if id := mux.Vars(r)["id"]; id != "" {
			if row, e := db.GetOne("SELECT html_id FROM posts WHERE id = $1", id); e == nil && row != nil {
				existingID = functions.Coerce[int64](row["html_id"])
			}
		}

		compiled := html.RenderMarkdown(markdown)
		if existingID > 0 {
			n, err := db.UpdateRow("html", map[string]interface{}{"compiled_html": compiled}, "id", existingID)
			if err != nil {
				return nil, err
			}
			if n > 0 {
				data["html_id"] = existingID
				return data, nil
			}
		}

		id, err := db.InsertRow("html", map[string]interface{}{"compiled_html": compiled})
		if err != nil {
			return nil, err
		}
		data["html_id"] = id
		return data, nil
	}

	return m
}

// Global module instance (initialized at startup)
var Module *Posts

// Package initialization (similar to PHP module registration)
// Routes are automatically registered when Initialize() is called
// No manual route registration needed - the module system handles:
// GET    /posts     -> List()
// POST   /posts     -> Create()
// GET    /posts/{id} -> View()
// PUT    /posts/{id} -> Edit()
// DELETE /posts/{id} -> Delete()
func init() {
	Module = NewPosts()
	app.RegisterModule(Module, "posts")
}
