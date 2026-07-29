package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
