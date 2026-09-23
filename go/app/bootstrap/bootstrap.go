// Package bootstrap blank-imports every module/page/controller/feature so
// each one's own init() self-registers with go/app as a side effect.
package bootstrap

import (
	// Engine modules
	_ "tls-rest/go/engine/modules/accesslog"
	_ "tls-rest/go/engine/modules/accessrules"
	_ "tls-rest/go/engine/modules/comments"
	_ "tls-rest/go/engine/modules/contact"
	_ "tls-rest/go/engine/modules/deletion"
	_ "tls-rest/go/engine/modules/externalconfig"
	_ "tls-rest/go/engine/modules/friends"
	_ "tls-rest/go/engine/modules/images"
	_ "tls-rest/go/engine/modules/likes"
	_ "tls-rest/go/engine/modules/logs"
	_ "tls-rest/go/engine/modules/messages"
	_ "tls-rest/go/engine/modules/modulerights"
	_ "tls-rest/go/engine/modules/publicprofile"
	_ "tls-rest/go/engine/modules/translations"
	_ "tls-rest/go/engine/modules/usergroups"
	_ "tls-rest/go/engine/modules/users"

	// App modules
	_ "tls-rest/go/modules/posts"
	_ "tls-rest/go/modules/words"

	// Engine pages
	_ "tls-rest/go/engine/modules/config"
	_ "tls-rest/go/engine/pages/actions"
	_ "tls-rest/go/engine/pages/console"
	_ "tls-rest/go/engine/pages/login"
	_ "tls-rest/go/engine/pages/profile"
	_ "tls-rest/go/engine/pages/statistics"

	// App pages
	_ "tls-rest/go/pages/myip"
	_ "tls-rest/go/pages/netmapper"

	// Shared controllers: reusable by any module, not owned by one.
	_ "tls-rest/go/engine/controllers/turn"

	// Features that own arbitrary route trees with unexported handlers.
	_ "tls-rest/go/modules/papers"
	_ "tls-rest/go/pages/opencv"
)
