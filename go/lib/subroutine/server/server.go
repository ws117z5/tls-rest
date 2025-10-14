package server

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/ws117z5/tls-rest/go/lib/route"

	"golang.org/x/net/http2"
)

// RunServer runs the server instance
func RunServer() {

	cert, err := tls.LoadX509KeyPair(".private/ec_certificate.pem", ".private/ec_private_key.pem")
	if err != nil {
		log.Fatalf("failed to load certificate and key: %v", err)
	}

	tlsConfig := &tls.Config{
		Certificates:     []tls.Certificate{cert},
		MinVersion:       tls.VersionTLS12,
		CurvePreferences: []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		NextProtos:       []string{http2.NextProtoTLS, "http/1.1"},
	}

	srv := &http.Server{
		Handler:      route.GetRouter(),
		TLSConfig:    tlsConfig,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	http2Server := &http2.Server{}
	http2.ConfigureServer(srv, http2Server) // Enforce HTTP/2

	fmt.Println(srv.ListenAndServeTLS("", ""))

	//fmt.Println(srv.ListenAndServeTLS("", ""))
}
