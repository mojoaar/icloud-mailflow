package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTimeoutExceptSkipsPrefix(t *testing.T) {
	hasDeadline := func(path string) bool {
		var got bool
		h := timeoutExcept("/mcp", 20*time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, got = r.Context().Deadline()
			w.WriteHeader(http.StatusOK)
		}))
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", path, nil))
		return got
	}

	if hasDeadline("/mcp") {
		t.Error("/mcp must not receive a request-timeout deadline")
	}
	if !hasDeadline("/dashboard") {
		t.Error("/dashboard should receive a request-timeout deadline")
	}
}
