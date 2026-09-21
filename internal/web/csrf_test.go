package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func csrfCookieValue(rec *httptest.ResponseRecorder) string {
	for _, c := range rec.Result().Cookies() {
		if c.Name == "mailflow_csrf" {
			return c.Value
		}
	}
	return ""
}

func TestCSRFTokenStableAcrossRenders(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, r, "Login", "login", map[string]any{})
	})

	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, httptest.NewRequest("GET", "/login", nil))
	first := csrfCookieValue(rec1)
	if first == "" {
		t.Fatal("expected a CSRF cookie on the first render")
	}

	req2 := httptest.NewRequest("GET", "/login", nil)
	req2.AddCookie(&http.Cookie{Name: "mailflow_csrf", Value: first})
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)

	if got := csrfCookieValue(rec2); got != first {
		t.Errorf("CSRF token rotated across renders: %q -> %q", first, got)
	}
}

func TestCSRFTokenRotatesWhenCookieAbsent(t *testing.T) {
	rec := httptest.NewRecorder()
	renderPage(rec, httptest.NewRequest("GET", "/login", nil), "Login", "login", map[string]any{})
	if csrfCookieValue(rec) == "" {
		t.Error("expected a CSRF cookie")
	}
}
