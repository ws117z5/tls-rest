package server

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tls-rest/go/lib/route"
	middleware "tls-rest/go/lib/route/middlware"
)

// shutdownTimeout bounds how long in-flight requests get to finish on shutdown.
const shutdownTimeout = 30 * time.Second

// RunServer starts the HTTPS server on port 8443 using Cloudflare Origin Certificates.
// It blocks until receiving an interrupt or termination signal for graceful shutdown.
func RunServer() {
	// Load Cloudflare Origin Certificate and Private Key.
	// Ensure these files exist at the configured path or pass via environment variables.
	certPath := os.Getenv("CF_CERT_PATH")
	if certPath == "" {
		certPath = ".private/cert.pem" // Path to Cloudflare Origin Cert
	}

	keyPath := os.Getenv("CF_KEY_PATH")
	if keyPath == "" {
		keyPath = ".private/key.pem" // Path to Cloudflare Origin Key
	}

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		log.Fatalf("failed to load Cloudflare origin certificate and key: %v", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		// TLS 1.2 minimum; TLS 1.3 is preferred and auto-negotiated by Cloudflare.
		MinVersion: tls.VersionTLS12,
		// Prefer fast, secure curves supported by Cloudflare.
		CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256},
		// Recommended Cipher Suites for Cloudflare origin connections
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		},
		// Advertise HTTP/2 for optimal performance with Cloudflare edge.
		NextProtos: []string{"h2", "http/1.1"},
	}

	// Wrap the router with baseline security headers.
	handler := middleware.SecureHeaders(route.GetRouter())

	srv := &http.Server{
		Addr:      ":8443", // Configured for Cloudflare HTTPS origin port 8443
		Handler:   handler,
		TLSConfig: tlsConfig,

		// Timeout configuration to mitigate slow-loris attacks and connection leaks.
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB
	}

	// Serve in the background so the main goroutine can listen for OS signals.
	serveErr := make(chan error, 1)
	go func() {
		log.Printf("HTTPS server listening on %s (Cloudflare TLS / HTTP/2 enabled)", srv.Addr)
		// Certificate is loaded into TLSConfig; file arguments are left blank.
		err := srv.ListenAndServeTLS("", "")
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	// Wait for serve error or shutdown signal.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serveErr:
		if err != nil {
			log.Fatalf("server error: %v", err)
		}
		return
	case sig := <-stop:
		log.Printf("received signal %s, shutting down gracefully...", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed, forcing close: %v", err)
		_ = srv.Close()
	}
	log.Println("server stopped")
}
