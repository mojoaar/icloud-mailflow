package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func securityTestHandler() http.Handler {
	return securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
}

func TestSecurityHeadersCSP(t *testing.T) {
	rec := httptest.NewRecorder()
	securityTestHandler().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("missing Content-Security-Policy header")
	}
	for _, want := range []string{
		"default-src 'self'",
		"object-src 'none'",
		"base-uri 'self'",
		"form-action 'self'",
		"frame-ancestors 'none'",
		"https://unpkg.com",
	} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP missing %q\n%s", want, csp)
		}
	}

	if rec.Header().Get("Strict-Transport-Security") != "" {
		t.Error("HSTS must not be sent over plain HTTP")
	}
}

func TestSecurityHeadersHSTSOnTLS(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	securityTestHandler().ServeHTTP(rec, req)

	if rec.Header().Get("Strict-Transport-Security") == "" {
		t.Error("expected HSTS when the request is HTTPS")
	}
}
