package images

import (
	engine "tls-rest/go/engine"

	"github.com/gorilla/mux"
)

// init self-registers the image endpoints. Uploading is gated to authenticated
// users by the middleware; serving enforces per-image access itself.
func init() {
	engine.AddRouteRegistrar(func(router *mux.Router) {
		router.HandleFunc("/api/images/process", Process).Methods("POST")
		router.HandleFunc("/image/{ref}", ServeByRef).Methods("GET")
	})
}
