package main

import (
	app "tls-rest/go/app"

	// Triggers every module/page's self-registering init() via import.
	_ "tls-rest/go/app/bootstrap"

	input "tls-rest/go/engine/controllers/subroutine/input"
	server "tls-rest/go/engine/controllers/subroutine/server"
)

func main() {
	app.Run(server.RunServer, input.ReadCommand)
}
