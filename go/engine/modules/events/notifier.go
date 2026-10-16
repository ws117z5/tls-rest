package events

import (
	"context"
	"fmt"
	"time"

	"tls-rest/go/engine/controllers/actions"
	"tls-rest/go/engine/controllers/db/pgdb"
	"tls-rest/go/engine/controllers/functions"
)

// notifyCheckInterval is how often checkDueNotifications runs — frequent
// enough that a reminder fires within a minute of its offset, cheap enough
// (one UPDATE ... RETURNING, usually zero rows) to poll that often.
const notifyCheckInterval = 30 * time.Second

// checkDueNotifications marks every event whose reminder time has arrived as
// notified (so it fires exactly once) and pings every recipient's SSE hub.
// Registered as an actions.Action so it runs on a schedule like any other
// background job in this app (see go/engine/pages/actions/jobs).
func checkDueNotifications() (string, error) {
	ctx := context.Background()
	db, err := pgdb.GetInstanceCtx(ctx)
	if err != nil {
		return "", err
	}

	// A repeating event re-arms instead of marking notified_at: starts_at
	// rolls forward to its next occurrence and notified_at stays NULL, so
	// this same row fires again next cycle instead of only once ever. A
	// non-repeating event behaves as before (notified_at = now(), fires once).
	due, err := db.GetAll(`
		UPDATE events SET
			notified_at = CASE WHEN repeat IS NULL OR repeat = '' OR repeat = 'none' THEN now() ELSE NULL END,
			starts_at = CASE repeat
				WHEN 'daily' THEN starts_at + interval '1 day'
				WHEN 'weekly' THEN starts_at + interval '7 days'
				WHEN 'monthly' THEN starts_at + interval '1 month'
				WHEN 'annual' THEN starts_at + interval '1 year'
				ELSE starts_at
			END
		WHERE notify = true AND notified_at IS NULL
		  AND starts_at - (notify_offset_minutes::int * interval '1 minute') <= now()
		RETURNING id, created_by, scope, target_group_id, target_user_id
	`)
	if err != nil {
		return "", err
	}

	notified := map[int]bool{}
	for _, row := range due {
		for _, uid := range recipientsOf(db, row) {
			if uid > 0 {
				hub.notify(uid)
				notified[uid] = true
			}
		}
	}

	return fmt.Sprintf("%d event(s) due, %d user(s) notified", len(due), len(notified)), nil
}

// recipientsOf resolves one due event's row into the user ids it should
// notify — created_by, plus whoever its scope shares it with.
func recipientsOf(db *pgdb.Db, row map[string]interface{}) []int {
	out := []int{functions.Int(row["created_by"])}

	switch functions.Coerce[string](row["scope"]) {
	case "user":
		if uid := functions.Int(row["target_user_id"]); uid > 0 {
			out = append(out, uid)
		}
	case "group":
		if gid := functions.Int(row["target_group_id"]); gid > 0 {
			rows, err := db.GetAll(`SELECT id FROM users WHERE groups @> to_jsonb($1::int)`, gid)
			if err == nil {
				for _, r := range rows {
					out = append(out, functions.Int(r["id"]))
				}
			}
		}
	case "global":
		rows, err := db.GetAll(`SELECT id FROM users`)
		if err == nil {
			for _, r := range rows {
				out = append(out, functions.Int(r["id"]))
			}
		}
	}
	return out
}

func init() {
	a := &actions.Action{
		ID:          "events_notify",
		Name:        "Event reminders",
		Description: "Pings users whose event reminder time has arrived.",
		Run:         checkDueNotifications,
	}
	actions.Register(a)
	a.SetSchedule(notifyCheckInterval)
}
