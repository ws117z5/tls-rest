//go:build opencv

package opencv

import (
	"net/http"
	engine "tls-rest/go/engine"

	"github.com/gorilla/mux"
)

// init self-registers the OpenCV endpoint through the shared route-registrar
// seam, so route.go no longer hardcodes it. The browser posts its WebRTC offer
// (clientSession) here; the handler runs GoCV motion detection and answers.
//
// NOTE: this package depends on gocv.io/x/gocv, which requires the OpenCV C
// libraries (and ffmpeg at runtime) on the build/host machine. It is compiled
// only because route.go imports it — drop that import to build without OpenCV.
func init() {
	engine.AddRouteRegistrar(func(router *mux.Router) {
		router.HandleFunc("/opencv", Init).Methods(http.MethodPost, http.MethodOptions)

		router.HandleFunc("/opencv/filters", GetFiltersHandler).Methods(http.MethodGet, http.MethodPost, http.MethodOptions)
		router.HandleFunc("/opencv/filter", ChangeFilterHandler).Methods(http.MethodPost, http.MethodOptions)
	})
}
