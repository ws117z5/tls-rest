package html

import (
	"html/template"
	"net/http"

	"tls-rest/go/constants"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func GetHtml(file string, data constants.Tmpl, w http.ResponseWriter) {
	t, err := template.ParseFiles("/templates/" + file)
	t.Execute(w, data)
	check(err)
}
