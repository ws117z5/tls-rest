package registry

import (
	auth "tls-rest/go/engine/controllers/auth"
	cfg "tls-rest/go/engine/controllers/config"
	module "tls-rest/go/engine/controllers/module"

	// Engine modules
	accesslog "tls-rest/go/engine/modules/accesslog"
	accessrules "tls-rest/go/engine/modules/accessrules"
	comments "tls-rest/go/engine/modules/comments"
	contact "tls-rest/go/engine/modules/contact"
	deletion "tls-rest/go/engine/modules/deletion"
	friends "tls-rest/go/engine/modules/friends"
	images "tls-rest/go/engine/modules/images"
	likes "tls-rest/go/engine/modules/likes"
	logs "tls-rest/go/engine/modules/logs"
	messages "tls-rest/go/engine/modules/messages"
	modulerights "tls-rest/go/engine/modules/modulerights"
	publicprofile "tls-rest/go/engine/modules/publicprofile"
	translations "tls-rest/go/engine/modules/translations"
	usergroups "tls-rest/go/engine/modules/usergroups"
	users "tls-rest/go/engine/modules/users"

	// App modules
	posts "tls-rest/go/modules/posts"
	words "tls-rest/go/modules/words"

	// Engine pages
	actionspage "tls-rest/go/engine/pages/actions"
	configmod "tls-rest/go/engine/modules/config"
	console "tls-rest/go/engine/pages/console"
	login "tls-rest/go/engine/pages/login"
	profile "tls-rest/go/engine/pages/profile"
	statistics "tls-rest/go/engine/pages/statistics"

	// App pages
	myip "tls-rest/go/pages/myip"
	netmapper "tls-rest/go/pages/netmapper"

	// Features that own arbitrary route trees with unexported handlers — their
	// registration lives behind an exported Register() in the package.
	papers "tls-rest/go/modules/papers"
	opencv "tls-rest/go/pages/opencv"
)

// RegisterAll registers every module, page, and feature. Call once from main()
// before the router is assembled.
func InitAll() {
	// Invalidate cached session rights when users/groups/rights change.
	module.OnRightsChange = auth.BumpRightsEpoch
	module.OnConfigChange = cfg.BumpConfigEpoch
	// --- Modules ---
	users.Init()
	posts.Init()
	words.Init()
	usergroups.Init()
	modulerights.Init()
	accesslog.Init()
	accessrules.Init()
	images.Init()
	comments.Init()
	likes.Init()
	logs.Init()
	messages.Init()
	friends.Init()
	publicprofile.Init()
	contact.Init()
	deletion.Init()
	translations.Init()

	// --- Pages ---
	login.Init()
	configmod.Init()
	console.Init()
	profile.Init()
	statistics.Init()
	actionspage.Init()
	netmapper.Init()
	myip.Init()

	// --- Features (own route trees; unexported handlers) ---
	papers.Init()
	opencv.Init()
}
