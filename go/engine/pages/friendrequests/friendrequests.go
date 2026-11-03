// Package friendrequests is a pure menu entry — a signed-in-only page with no
// data/routes of its own; its frontend (js/src/engine/pages/friendrequests)
// reads straight from the already-existing GET /api/chats (filtered to
// type=="direct" && myStatus=="pending") and responds through the already-
// existing /api/chats/{id}/accept, /api/blocks/{userId}, and
// /api/users/{userId}/report (see go/engine/modules/chats).
package friendrequests

import (
	"tls-rest/go/app"
	"tls-rest/go/engine/controllers/module"
)

var Page = &module.PageAbstract{
	ID:           "friendrequests",
	Name:         "Friend requests",
	Icon:         "messages",
	RequiresAuth: true,
}

func init() { app.RegisterPage(Page) }
