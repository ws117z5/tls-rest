package server

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	f "tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/log"
	"tls-rest/go/engine/controllers/route"
	"tls-rest/go/engine/controllers/route/middleware"
)

// shutdownTimeout bounds how long in-flight requests get to finish on shutdown.
const shutdownTimeout = 30 * time.Second

// RunServer starts the HTTPS server on port 8443 using Cloudflare Origin Certificates.
// It blocks until receiving an interrupt or termination signal for graceful shutdown.
func RunServer() {
	// Load Cloudflare Origin Certificate and Private Key.

	certPath := f.FirstNonEmpty(
		os.Getenv("TLS_CERT_PATH"),
		os.Getenv("CF_CERT_PATH"),
		".private/cert.pem",
	)
	keyPath := f.FirstNonEmpty(
		os.Getenv("TLS_KEY_PATH"),
		os.Getenv("CF_KEY_PATH"),
		".private/key.pem",
	)

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		log.Fatalf("failed to load Cloudflare origin certificate and key: %v, certPath=%s, keyPath=%s", err, certPath, keyPath)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
		},
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		},
		NextProtos: []string{"h2", "http/1.1"},
	}

	handler := middleware.SecureHeaders(route.GetRouter())

	srv := &http.Server{
		Addr:              ":8443",
		Handler:           handler,
		TLSConfig:         tlsConfig,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	serveErr := make(chan error, 1)
	go func() {
		log.Infof("HTTPS server listening on %s (Cloudflare TLS / HTTP/2 enabled)", srv.Addr)
		err := srv.ListenAndServeTLS("", "")
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	// Give the server a brief moment (100ms) to detect immediate binding/startup errors
	select {
	case err := <-serveErr:
		log.Fatalf("server failed to start: %v", err)
	case <-time.After(100 * time.Millisecond):
		log.Success("server started successfully and is ready to accept connections")
	}

	// Block until receiving termination signal or a runtime error occurs
	select {
	case err := <-serveErr:
		log.Fatalf("server error occurred: %v", err)
	case sig := <-stop:
		log.Successf("received signal %s, shutting down gracefully...", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed, forcing close: %v", err)
		_ = srv.Close()
	}
	log.Info("server stopped")
}
