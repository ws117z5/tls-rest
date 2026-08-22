package images

import (
	"tls-rest/go/engine/controllers/module"

	"github.com/gorilla/mux"
)

// init self-registers the image endpoints. Uploading is gated to authenticated
// users by the middleware; serving enforces per-image access itself.
func Init() {
	NewImages().Initialize("images")

	module.AddRouteRegistrar(func(router *mux.Router) {
		router.HandleFunc("/api/images/process", Process).Methods("POST")
		router.HandleFunc("/image/{ref}", ServeByRef).Methods("GET")
	})
}
