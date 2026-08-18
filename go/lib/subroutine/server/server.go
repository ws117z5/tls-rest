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

// RunServer starts the HTTPS server and blocks until it receives an interrupt
// or termination signal, at which point it shuts down gracefully, draining
// in-flight requests before returning.
func RunServer() {
	cert, err := tls.LoadX509KeyPair(".private/cert.pem", ".private/key.pem")
	if err != nil {
		log.Fatalf("failed to load certificate and key: %v", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		// TLS 1.2 floor; 1.3 is negotiated automatically when the client supports it.
		MinVersion: tls.VersionTLS12,
		// Prefer the fast, modern X25519 curve, then P-256.
		CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256},
		// Advertise HTTP/2. net/http enables h2 natively for TLS servers, so the
		// external golang.org/x/net/http2 package is no longer required.
		NextProtos: []string{"h2", "http/1.1"},
	}

	// Wrap the whole router (static assets included) with baseline security
	// headers as the outermost layer.
	handler := middleware.SecureHeaders(route.GetRouter())

	srv := &http.Server{
		Addr:      ":https", // 443
		Handler:   handler,
		TLSConfig: tlsConfig,

		// A full timeout budget protects against slow-loris and leaked connections.
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB
	}

	// Serve in the background so the main goroutine can wait for a signal.
	serveErr := make(chan error, 1)
	go func() {
		log.Printf("HTTPS server listening on %s (HTTP/2 enabled)", srv.Addr)
		// The certificate is already in TLSConfig, so the file arguments are empty.
		err := srv.ListenAndServeTLS("", "")
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	// Wait for either a fatal serve error or a termination signal.
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
