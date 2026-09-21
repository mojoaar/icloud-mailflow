package web

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
)

type cspNonceKey struct{}

func nonceFrom(r *http.Request) string {
	s, _ := r.Context().Value(cspNonceKey{}).(string)
	return s
}

func newNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
}

func contentSecurityPolicy(nonce string) string {
	return "default-src 'self'; " +
		"script-src 'self' 'nonce-" + nonce + "'; " +
		"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; " +
		"font-src https://fonts.gstatic.com; " +
		"img-src 'self' data:; " +
		"connect-src 'self'; " +
		"object-src 'none'; " +
		"base-uri 'self'; " +
		"form-action 'self'; " +
		"frame-ancestors 'none'"
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonce := newNonce()
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", contentSecurityPolicy(nonce))
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), cspNonceKey{}, nonce)))
	})
}
