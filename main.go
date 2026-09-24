package main

import (
	app "tls-rest/go/app"

	// Triggers every module/page's self-registering init() via import.
	_ "tls-rest/go/app/bootstrap"

	"tls-rest/go/engine/controllers/auth"
	"tls-rest/go/engine/controllers/config"
	input "tls-rest/go/engine/controllers/subroutine/input"
	server "tls-rest/go/engine/controllers/subroutine/server"
)

func main() {
	app.Run(server.RunServer, input.ReadCommand, auth.BumpRightsEpoch, config.BumpConfigEpoch)
}
