package papers

import (
	engine "tls-rest/go/engine"

	"github.com/gorilla/mux"
)

// init self-registers the papers routes through the shared route-registrar seam,
// so route.go no longer hardcodes them. (papers.go's other init() wires the
// negotiator; Go runs both.)
func init() {
	engine.AddRouteRegistrar(func(router *mux.Router) {
		p := router.PathPrefix("/papers").Subrouter()
		p.HandleFunc("", List).Methods("GET")
		p.HandleFunc("/create", CreateRoom).Methods("POST")
		p.HandleFunc("/{roomId}", AddRoomUser).Methods("POST")
		p.HandleFunc("/{roomId}", ViewRoomUsers).Methods("GET")
		p.HandleFunc("/{roomId}/report", ReportLink).Methods("POST")
		p.HandleFunc("/{roomId}/plan", GetPlan).Methods("GET")
		p.HandleFunc("/{roomId}/{userId}", RegisterUser).Methods("POST")
	})
}
