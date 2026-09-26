package httpx

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
)

type nonceKey struct{}

// NewNonce returns a random per-response CSP nonce.
func NewNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

// WithNonce returns r carrying nonce, for templates that emit inline scripts.
func WithNonce(r *http.Request, nonce string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), nonceKey{}, nonce))
}

// Nonce returns the CSP nonce set by the security-headers middleware ("" if none).
func Nonce(r *http.Request) string {
	n, _ := r.Context().Value(nonceKey{}).(string)
	return n
}
