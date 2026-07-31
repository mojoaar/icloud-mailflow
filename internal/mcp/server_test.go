package mcp

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mojoaar/icloud-mailflow/internal/db"
)

func TestNew(t *testing.T) {
	d := db.NewTestDB(t)
	srvr := New(d, nil, nil, "0.6.0", nil, nil)
	if srvr == nil {
		t.Fatal("New returned nil")
	}
}

func TestAuthMiddleware(t *testing.T) {
	d := db.NewTestDB(t)
	settingsRepo := db.NewSettingsRepo(d)
	settingsRepo.Set("mcp_api_key", "my-secret-key")
	settingsRepo.Set("mcp_enabled", "true")

	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := NewAuthMiddleware(backend, settingsRepo)

	t.Run("missing auth header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("wrong key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		req.Header.Set("Authorization", "Bearer wrong")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("correct key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		req.Header.Set("Authorization", "Bearer my-secret-key")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("disabled", func(t *testing.T) {
		settingsRepo.Set("mcp_enabled", "false")
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		req.Header.Set("Authorization", "Bearer my-secret-key")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rec.Code)
		}
		settingsRepo.Set("mcp_enabled", "true")
	})

	t.Run("disabled when setting missing", func(t *testing.T) {
		d2 := db.NewTestDB(t)
		sr := db.NewSettingsRepo(d2)
		h := NewAuthMiddleware(backend, sr)
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		req.Header.Set("Authorization", "Bearer any-key")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rec.Code)
		}
	})
}

func TestGenerateAPIKey(t *testing.T) {
	key1 := generateAPIKey()
	key2 := generateAPIKey()
	if len(key1) != 64 {
		t.Errorf("expected 64 chars (32 bytes hex), got %d", len(key1))
	}
	if key1 == key2 {
		t.Error("generated keys should be unique")
	}
}

func TestParseRuleInput(t *testing.T) {
	rule, err := parseRuleInput("Test Rule", 1,
		`{"operator":"AND","conditions":[{"field":"from","operator":"contains","value":"@example.com"}]}`,
		`[{"type":"move_to_folder","value":"Archive"}]`,
	)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if rule.Name != "Test Rule" {
		t.Errorf("name: %s", rule.Name)
	}
	if rule.Priority != 1 {
		t.Errorf("priority: %d", rule.Priority)
	}
	if len(rule.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(rule.Groups))
	}
	if rule.Groups[0].Operator != "AND" {
		t.Errorf("operator: %s", rule.Groups[0].Operator)
	}
	if len(rule.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(rule.Actions))
	}
	if rule.Actions[0].Type != "move_to_folder" || rule.Actions[0].Value != "Archive" {
		t.Errorf("action: %+v", rule.Actions[0])
	}
}

func TestParseRuleInputDefaults(t *testing.T) {
	rule, err := parseRuleInput("Minimal", 5,
		`{"conditions":[{"field":"subject","operator":"contains","value":"hello"}]}`,
		`[]`,
	)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if rule.Groups[0].Operator != "OR" {
		t.Errorf("default operator should be OR, got %s", rule.Groups[0].Operator)
	}
}

func TestParseRuleInputNoConditions(t *testing.T) {
	rule, err := parseRuleInput("NoConds", 1, `{"conditions":[]}`, `[{"type":"mark_as_read","value":""}]`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(rule.Groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(rule.Groups))
	}
	if len(rule.Actions) != 1 {
		t.Errorf("expected 1 action, got %d", len(rule.Actions))
	}
}

func TestNewAllowsNilDeps(t *testing.T) {
	var d *sql.DB
	srvr := New(d, nil, nil, "1.0", nil, nil)
	if srvr == nil {
		t.Fatal("New should not panic with nil deps")
	}
}
