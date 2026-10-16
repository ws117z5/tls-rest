// Calendar page (js/src/pages/calendar) — a month/week/day visualization over
// the events module's own data. It has no routes of its own: it reads and
// writes through events' ordinary CRUD API (GET /api/modules/events with
// from/to list filters, POST /api/modules/events to create). Google Calendar
// sync is requested from the Profile page instead (go/engine/pages/profile) —
// one-way push only, triggered per-event by its own sync_with_google field.
package calendar

import (
	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/module"
)

var Page = &module.PageAbstract{
	ID:   "calendar",
	Name: "Calendar",
	Icon: "calendar",
	// No Submenu: top-level, like the messages page — Menu.tsx special-cases
	// this same key to render it with the due-reminders badge instead of the
	// generic head-item rendering.
	RequiresAuth: true,
}

func init() { app.RegisterPage(Page) }
