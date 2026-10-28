// Package msgusers is an admin-only, read-only module listing msg-app users
// with live online status, via DataFetch — no table of its own.
package msgusers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/db/pgdb"
	. "tls-rest/go/engine/controllers/field"
	"tls-rest/go/engine/controllers/functions"
	. "tls-rest/go/engine/controllers/module"
	"tls-rest/go/engine/controllers/request"
	"tls-rest/go/engine/modules/p2prendezvous"
)

type MsgUsers struct {
	*ModuleAbstract[interface{}]
}

func (m *MsgUsers) fieldset() []Field {
	return []Field{
		NewField("user_name", TYPE_STRING, false).WithLabel("Username").AsReadOnly(),
		NewField("first_name", TYPE_STRING, false).WithLabel("First name").AsReadOnly(),
		NewField("last_name", TYPE_STRING, false).WithLabel("Last name").AsReadOnly(),
		NewField("email", TYPE_STRING, false).WithLabel("Email").AsReadOnly(),
		NewField("last_seen", TYPE_DATE_TIME, false).WithLabel("Last seen").AsReadOnly(),
		NewField("online", TYPE_CHECKBOX, false).WithLabel("Online").AsReadOnly(),
	}
}

// filters are display-only: fetchRows reads these params itself, bypassing the declarative SQL builder.
func (m *MsgUsers) filters() *Filedset {
	return NewFieldset(
		NewFilter("name", TYPE_STRING).WithLabel("Name").Contains(),
		NewFilter("email", TYPE_STRING).WithLabel("Email").Contains(),
		NewFilter("friends", TYPE_STRING).WithLabel("Friends (usernames, comma-separated)").Equals(),
		NewFilter("online", TYPE_CHECKBOX).WithLabel("Online only").Equals(),
	)
}

func readOnly(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "msgusers is read-only", http.StatusMethodNotAllowed)
}

func splitTrimmed(csv string) []string {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// fetchRows is the DataFetch hook: friends/online can't be one SQL WHERE clause.
func fetchRows(ctx context.Context, r *http.Request, mode int) ([]map[string]interface{}, error) {
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return nil, err
	}

	rq := request.From(r)
	var conds []string
	var args []interface{}
	argIndex := 1

	if mode == MODE_VIEW {
		if id := rq.String("id"); id != "" {
			conds = append(conds, fmt.Sprintf("u.id = $%d", argIndex))
			args = append(args, id)
			argIndex++
		}
	}
	if name := strings.TrimSpace(rq.String("name")); name != "" {
		conds = append(conds, fmt.Sprintf("(first_name ILIKE $%d OR last_name ILIKE $%d)", argIndex, argIndex))
		args = append(args, "%"+name+"%")
		argIndex++
	}
	if email := strings.TrimSpace(rq.String("email")); email != "" {
		conds = append(conds, fmt.Sprintf("email ILIKE $%d", argIndex))
		args = append(args, "%"+email+"%")
		argIndex++
	}
	if friends := splitTrimmed(rq.String("friends")); len(friends) > 0 {
		conds = append(conds, fmt.Sprintf(
			`EXISTS (SELECT 1 FROM friends f JOIN users fu ON fu.id = CASE WHEN f.user_a = u.id THEN f.user_b ELSE f.user_a END `+
				`WHERE (f.user_a = u.id OR f.user_b = u.id) AND fu.user_name = ANY($%d))`,
			argIndex,
		))
		args = append(args, friends)
		argIndex++
	}

	query := "SELECT id, user_name, first_name, last_name, email, last_seen FROM users u"
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY user_name"

	rows, err := db.GetAll(query, args...)
	if err != nil {
		return nil, err
	}

	onlineOnly := rq.String("online") == "true"
	out := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		online := p2prendezvous.IsOnline(functions.Coerce[int](row["id"]))
		if onlineOnly && !online {
			continue
		}
		row["online"] = online
		out = append(out, row)
	}
	return out, nil
}

func NewMsgUsers() *MsgUsers {
	m := &MsgUsers{
		ModuleAbstract: &ModuleAbstract[interface{}]{
			ID:                   "msgusers",
			Name:                 "Active Users",
			Icon:                 "statistics",
			Submenu:              "External",
			ReadOnly:             true,
			Rights:               make(map[int]int),
			DefaultPermission:    PERMISSION_DENY,
			DefaultPermissionSet: true,
			OmitSystemFields:     SystemFieldsExceptID(),
			Overrides: HandlerOverrides{
				Create: readOnly,
				Edit:   readOnly,
				Delete: readOnly,
			},
			DataFetch: fetchRows,
			NoTable:   true,
		},
	}
	m.ModuleAbstract.Fields = m.fieldset()
	m.ModuleAbstract.Filters = m.filters()
	return m
}

var Module *MsgUsers

func init() {
	app.RegisterModule(NewMsgUsers(), "users")
}
