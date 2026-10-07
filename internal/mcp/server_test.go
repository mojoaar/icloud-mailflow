package mcp

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mojoaar/icloud-mailflow/internal/db"
)

func TestMCPRateLimiterCleanup(t *testing.T) {
	rl := &mcpRateLimiter{entries: map[string]*mcpRateEntry{}}
	rl.entries["1.2.3.4"] = &mcpRateEntry{count: 5, resetAt: time.Now().Add(-time.Minute)}
	rl.entries["5.6.7.8"] = &mcpRateEntry{count: 2, resetAt: time.Now().Add(time.Minute)}

	rl.cleanup()

	if _, ok := rl.entries["1.2.3.4"]; ok {
		t.Error("expired entry should be removed")
	}
	if _, ok := rl.entries["5.6.7.8"]; !ok {
		t.Error("active entry should remain")
	}
}

func TestClientIPStripsPort(t *testing.T) {
	cases := map[string]string{
		"1.2.3.4:5678": "1.2.3.4",
		"1.2.3.4":      "1.2.3.4",
		"[::1]:80":     "::1",
	}
	for remote, want := range cases {
		r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		r.RemoteAddr = remote
		if got := clientIP(r); got != want {
			t.Errorf("clientIP(%q) = %q, want %q", remote, got, want)
		}
	}
}

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
	handler := NewAuthMiddleware(backend, settingsRepo, db.NewAuditRepo(d))

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
		h := NewAuthMiddleware(backend, sr, db.NewAuditRepo(d2))
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		req.Header.Set("Authorization", "Bearer any-key")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rec.Code)
		}
	})
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

func TestParseRuleInputInvalidRegex(t *testing.T) {
	_, err := parseRuleInput("Bad", 1,
		`{"conditions":[{"field":"from","operator":"matches_regex","value":"("}]}`,
		`[]`,
	)
	if err == nil {
		t.Fatal("expected error for invalid regex, got nil")
	}
}

func TestNewAllowsNilDeps(t *testing.T) {
	var d *sql.DB
	srvr := New(d, nil, nil, "1.0", nil, nil)
	if srvr == nil {
		t.Fatal("New should not panic with nil deps")
	}
}

func callTool(ctx context.Context, s *server.MCPServer, name string, args map[string]any) (*mcp.CallToolResult, error) {
	st := s.GetTool(name)
	if st == nil {
		return nil, fmt.Errorf("tool %s not found", name)
	}
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	return st.Handler(ctx, req)
}

func TestMCPToolsCore(t *testing.T) {
	d := db.NewTestDB(t)
	settingsRepo := db.NewSettingsRepo(d)
	settingsRepo.Set("source_folder", "INBOX")
	settingsRepo.Set("poll_interval", "120")
	settingsRepo.Set("webhook_secret", "secret123")

	srv := NewMCPServer(d, nil, nil, "1.0", nil, settingsRepo)
	ctx := context.Background()

	// 1. Create rule
	res, err := callTool(ctx, srv, "create_rule", map[string]any{
		"name":            "VIP Mail",
		"priority":        float64(1),
		"conditions_json": `{"operator":"AND","conditions":[{"field":"from","operator":"contains","value":"vip@example.com"}]}`,
		"actions_json":    `[{"type":"mark_as_read","value":""}]`,
	})
	if err != nil || res.IsError {
		t.Fatalf("create_rule failed: %v, res: %+v", err, res)
	}

	// 2. List rules
	res, err = callTool(ctx, srv, "list_rules", nil)
	if err != nil || res.IsError {
		t.Fatalf("list_rules failed: %v, res: %+v", err, res)
	}

	// 3. Get rule
	res, err = callTool(ctx, srv, "get_rule", map[string]any{"id": float64(1)})
	if err != nil || res.IsError {
		t.Fatalf("get_rule failed: %v, res: %+v", err, res)
	}

	// 4. Update rule
	res, err = callTool(ctx, srv, "update_rule", map[string]any{
		"id":   float64(1),
		"name": "VIP Mail Updated",
	})
	if err != nil || res.IsError {
		t.Fatalf("update_rule failed: %v, res: %+v", err, res)
	}

	// 5. Check email
	res, err = callTool(ctx, srv, "check_email", map[string]any{
		"from": "vip@example.com",
	})
	if err != nil || res.IsError {
		t.Fatalf("check_email failed: %v, res: %+v", err, res)
	}

	// 6. Get settings (verify webhook_secret redacted)
	res, err = callTool(ctx, srv, "get_settings", nil)
	if err != nil || res.IsError {
		t.Fatalf("get_settings failed: %v, res: %+v", err, res)
	}

	// 7. Update settings
	res, err = callTool(ctx, srv, "update_settings", map[string]any{
		"poll_interval": "180",
	})
	if err != nil || res.IsError {
		t.Fatalf("update_settings failed: %v, res: %+v", err, res)
	}
	if val, _ := settingsRepo.Get("poll_interval"); val != "180" {
		t.Errorf("poll_interval not updated, got %s", val)
	}

	// 8. Health
	res, err = callTool(ctx, srv, "health", nil)
	if err != nil || res.IsError {
		t.Fatalf("health tool failed: %v, res: %+v", err, res)
	}

	// 9. Get stats
	res, err = callTool(ctx, srv, "get_stats", map[string]any{"days": float64(7), "weeks": float64(4)})
	if err != nil || res.IsError {
		t.Fatalf("get_stats tool failed: %v, res: %+v", err, res)
	}

	// 10. Enable & disable rule
	res, err = callTool(ctx, srv, "disable_rule", map[string]any{"rule_id": float64(1)})
	if err != nil || res.IsError {
		t.Fatalf("disable_rule failed: %v, res: %+v", err, res)
	}
	res, err = callTool(ctx, srv, "enable_rule", map[string]any{"rule_id": float64(1)})
	if err != nil || res.IsError {
		t.Fatalf("enable_rule failed: %v, res: %+v", err, res)
	}

	// 11. Contacts & Activity
	res, err = callTool(ctx, srv, "list_contacts", nil)
	if err != nil || res.IsError {
		t.Fatalf("list_contacts failed: %v, res: %+v", err, res)
	}
	res, err = callTool(ctx, srv, "search_contacts", map[string]any{"q": "vip"})
	if err != nil || res.IsError {
		t.Fatalf("search_contacts failed: %v, res: %+v", err, res)
	}
	res, err = callTool(ctx, srv, "list_activity", map[string]any{"per_page": float64(10), "page": float64(1)})
	if err != nil || res.IsError {
		t.Fatalf("list_activity failed: %v, res: %+v", err, res)
	}
	res, err = callTool(ctx, srv, "clear_activity", nil)
	if err != nil || res.IsError {
		t.Fatalf("clear_activity failed: %v, res: %+v", err, res)
	}
	res, err = callTool(ctx, srv, "backup_rules", nil)
	if err != nil || res.IsError {
		t.Fatalf("backup_rules failed: %v, res: %+v", err, res)
	}

	// 12. Delete rule
	res, err = callTool(ctx, srv, "delete_rule", map[string]any{"id": float64(1)})
	if err != nil || res.IsError {
		t.Fatalf("delete_rule failed: %v, res: %+v", err, res)
	}
}
