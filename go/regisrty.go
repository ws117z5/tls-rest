package registry

import (
	// Engine modules
	accesslog "tls-rest/go/engine/modules/accesslog"
	accessrules "tls-rest/go/engine/modules/accessrules"
	modulerights "tls-rest/go/engine/modules/modulerights"
	usergroups "tls-rest/go/engine/modules/usergroups"
	users "tls-rest/go/engine/modules/users"

	// App modules
	posts "tls-rest/go/modules/posts"

	// Engine pages
	login "tls-rest/go/engine/pages/login"
	profile "tls-rest/go/engine/pages/profile"

	// App pages
	netmapper "tls-rest/go/pages/netmapper"

	// Features that own arbitrary route trees with unexported handlers — their
	// registration lives behind an exported Register() in the package.
	imagesfeature "tls-rest/go/engine/features/images"
	opencv "tls-rest/go/pages/opencv"
	papers "tls-rest/go/pages/papers"
)

// RegisterAll registers every module, page, and feature. Call once from main()
// before the router is assembled.
func InitAll() {
	// --- Modules ---
	users.Init()
	posts.Init()
	usergroups.Init()
	modulerights.Init()
	accesslog.Init()
	accessrules.Init()

	// --- Pages ---
	login.Init()
	profile.Init()
	netmapper.Init()

	// --- Features (own route trees; unexported handlers) ---
	imagesfeature.Init()
	papers.Init()
	opencv.Init()
}
