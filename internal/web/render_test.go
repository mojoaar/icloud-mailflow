package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRenderPage(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	renderPage(rec, req, "Test Page", "login", nil)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if body == "" {
		t.Error("response body should not be empty")
	}
}

func TestRenderPartial(t *testing.T) {
	rec := httptest.NewRecorder()

	renderPartial(rec, "toast", map[string]string{"Type": "success", "Message": "Done"})

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if body == "" {
		t.Error("response body should not be empty")
	}
}

func TestFormatUptime(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{45 * time.Second, "45s"},
		{3*time.Minute + 2*time.Second, "3m 2s"},
		{5*time.Hour + 3*time.Minute + 2*time.Second, "5h 3m 2s"},
		{24 * time.Hour, "1d 0h 0m 0s"},
		{25 * time.Hour, "1d 1h 0m 0s"},
		{222*time.Hour + 1*time.Minute + 36*time.Second, "9d 6h 1m 36s"},
		{30 * 24 * time.Hour, "30d 0h 0m 0s"},
	}
	for _, c := range cases {
		if got := formatUptime(c.d); got != c.want {
			t.Errorf("formatUptime(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestGenerateToken(t *testing.T) {
	t1, err := generateToken()
	if err != nil {
		t.Fatalf("generateToken: %v", err)
	}
	if len(t1) != 64 {
		t.Errorf("token length = %d, want 64", len(t1))
	}

	t2, err := generateToken()
	if err != nil {
		t.Fatalf("generateToken 2: %v", err)
	}
	if t1 == t2 {
		t.Error("tokens should be unique")
	}
}

func TestRulesTestResultColors(t *testing.T) {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "rules_test_result", map[string]any{"Matched": true, "Results": nil}); err != nil {
		t.Fatalf("execute matched: %v", err)
	}
	if !strings.Contains(buf.String(), "var(--green)") {
		t.Error("matched result should use var(--green)")
	}

	buf.Reset()
	if err := tmpl.ExecuteTemplate(&buf, "rules_test_result", map[string]any{"Matched": false, "Results": nil}); err != nil {
		t.Fatalf("execute no-match: %v", err)
	}
	if !strings.Contains(buf.String(), "var(--red)") {
		t.Error("no-match result should use var(--red)")
	}
}
