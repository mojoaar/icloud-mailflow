package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPromHandler(t *testing.T) {
	rec := httptest.NewRecorder()
	PromHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "mailflow_") {
		t.Error("metrics body should contain mailflow_ metrics")
	}
}
