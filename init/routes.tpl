package main

import (
    "net/http"
)

var AutoFuncMap = map[string]http.HandlerFunc{
{{- range . }}
    "{{ .Name }}": {{ .Name }},
{{- end }}
}