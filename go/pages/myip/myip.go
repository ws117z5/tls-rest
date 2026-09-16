// Package myip serves GET /myip: a plain SSR page showing the caller's
// resolved IP and User-Agent, for connectivity/proxy debugging.
package myip

import (
	"html/template"
	"log"
	"net/http"

	"github.com/gorilla/mux"

	"tls-rest/go/engine/controllers/accesslog"
	"tls-rest/go/engine/controllers/module"
)

type viewData struct {
	IP        string
	UserAgent string
}

func handler(w http.ResponseWriter, r *http.Request) {
	data := viewData{
		IP:        accesslog.ClientIP(r),
		UserAgent: r.UserAgent(),
	}

	t, err := template.ParseFiles("templates/myip.gohtml")
	if err != nil {
		log.Println(err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		log.Println(err.Error())
	}
}

// Init registers GET /myip.
func Init() {
	module.AddRouteRegistrar(func(router *mux.Router) {
		router.HandleFunc("/myip", handler).Methods("GET")
	})
}
