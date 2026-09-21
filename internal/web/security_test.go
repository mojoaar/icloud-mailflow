package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func securityTestHandler(capture *string) http.Handler {
	return securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if capture != nil {
			*capture = nonceFrom(r)
		}
		w.WriteHeader(http.StatusOK)
	}))
}

func TestSecurityHeadersCSP(t *testing.T) {
	rec := httptest.NewRecorder()
	securityTestHandler(nil).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("missing Content-Security-Policy header")
	}
	for _, want := range []string{
		"default-src 'self'",
		"script-src 'self' 'nonce-",
		"object-src 'none'",
		"base-uri 'self'",
		"form-action 'self'",
		"frame-ancestors 'none'",
	} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP missing %q\n%s", want, csp)
		}
	}
	for _, notWant := range []string{"https://unpkg.com", "https://cdn.jsdelivr.net", "unsafe-eval", "script-src 'self' 'unsafe-inline'"} {
		if strings.Contains(csp, notWant) {
			t.Errorf("strict CSP should not contain %q\n%s", notWant, csp)
		}
	}

	if rec.Header().Get("Strict-Transport-Security") != "" {
		t.Error("HSTS must not be sent over plain HTTP")
	}
}

func TestSecurityHeadersNonceMatchesContext(t *testing.T) {
	var ctxNonce string
	rec := httptest.NewRecorder()
	securityTestHandler(&ctxNonce).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if ctxNonce == "" {
		t.Fatal("nonce not placed in request context")
	}
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "'nonce-"+ctxNonce+"'") {
		t.Errorf("CSP nonce does not match context nonce %q\n%s", ctxNonce, csp)
	}
}

func TestSecurityHeadersHSTSOnTLS(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	securityTestHandler(nil).ServeHTTP(rec, req)

	if rec.Header().Get("Strict-Transport-Security") == "" {
		t.Error("expected HSTS when the request is HTTPS")
	}
}
