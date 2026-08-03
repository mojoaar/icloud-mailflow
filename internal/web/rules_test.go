package web

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	goimap "github.com/emersion/go-imap/v2"
	"github.com/go-chi/chi/v5"

	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/imap"
)

func TestRulesListHandler(t *testing.T) {
	database := openWebTestDB(t)
	repo := db.NewRulesRepo(database)
	repo.Create(&db.Rule{Name: "Rule 1", Priority: 0, Enabled: true})

	h := rulesListHandler(repo, db.NewFoldersRepo(database))
	req := httptest.NewRequest("GET", "/rules", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestRulesListHandlerHTMX(t *testing.T) {
	database := openWebTestDB(t)
	repo := db.NewRulesRepo(database)

	h := rulesListHandler(repo, db.NewFoldersRepo(database))
	req := httptest.NewRequest("GET", "/rules", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestRulesNewHandler(t *testing.T) {
	database := openWebTestDB(t)
	foldersRepo := db.NewFoldersRepo(database)
	contactsRepo := db.NewContactsRepo(database)
	settingsRepo := db.NewSettingsRepo(database)

	h := rulesNewHandler(foldersRepo, contactsRepo, settingsRepo)
	req := httptest.NewRequest("GET", "/rules/new", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestRulesCreateHandler(t *testing.T) {
	database := openWebTestDB(t)
	repo := db.NewRulesRepo(database)
	settingsRepo := db.NewSettingsRepo(database)

	h := rulesCreateHandler(repo, settingsRepo)
	form := url.Values{
		"name":        {"Test Rule"},
		"description": {"A test rule"},
		"priority":    {"5"},
	}
	req := httptest.NewRequest("POST", "/rules", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rec.Code)
	}

	rules, _ := repo.List()
	if len(rules) != 2 {
		t.Errorf("rules count = %d, want 2 (created + catch-all)", len(rules))
	}
}

func TestRulesEditHandler(t *testing.T) {
	database := openWebTestDB(t)
	repo := db.NewRulesRepo(database)
	foldersRepo := db.NewFoldersRepo(database)
	contactsRepo := db.NewContactsRepo(database)
	settingsRepo := db.NewSettingsRepo(database)
	repo.Create(&db.Rule{Name: "Test", Priority: 0, Enabled: true})
	rules, _ := repo.List()

	h := rulesEditHandler(repo, foldersRepo, contactsRepo, settingsRepo)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", fmt.Sprintf("%d", rules[0].ID))
	req := httptest.NewRequest("GET", "/rules/1/edit", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestRulesEditHandlerNotFound(t *testing.T) {
	database := openWebTestDB(t)
	repo := db.NewRulesRepo(database)
	foldersRepo := db.NewFoldersRepo(database)
	contactsRepo := db.NewContactsRepo(database)
	settingsRepo := db.NewSettingsRepo(database)

	h := rulesEditHandler(repo, foldersRepo, contactsRepo, settingsRepo)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "9999")
	req := httptest.NewRequest("GET", "/rules/9999/edit", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestRulesDeleteHandler(t *testing.T) {
	database := openWebTestDB(t)
	repo := db.NewRulesRepo(database)
	repo.Create(&db.Rule{Name: "DeleteMe", Priority: 0, Enabled: true})
	rules, _ := repo.List()

	h := rulesDeleteHandler(repo)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", fmt.Sprintf("%d", rules[0].ID))
	req := httptest.NewRequest("DELETE", "/rules/1", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rec.Code)
	}
}

func TestConditionFields(t *testing.T) {
	fields := conditionFields()
	if len(fields) < 1 {
		t.Error("conditionFields should return at least one field")
	}

	names := map[string]bool{}
	for _, f := range fields {
		names[f["value"]] = true
	}
	for _, expected := range []string{"from", "to", "cc", "subject", "has_attachment"} {
		if !names[expected] {
			t.Errorf("field %q not found", expected)
		}
	}
}

func TestRulesExportHandler(t *testing.T) {
	database := openWebTestDB(t)
	repo := db.NewRulesRepo(database)
	repo.Create(&db.Rule{
		Name: "Work Emails", Description: "Sort work mail", Priority: 0, Enabled: true,
		Groups:  []db.ConditionGroup{{Operator: "AND", Conditions: []db.Condition{{Field: "from", Operator: "contains", Value: "@work.com"}}}},
		Actions: []db.Action{{Type: "move_to_folder", Value: "Work"}},
	})

	h := rulesExportHandler(repo)
	req := httptest.NewRequest("GET", "/settings/rules/export", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Work Emails") {
		t.Error("body should contain rule name")
	}
	if !strings.Contains(body, "move_to_folder") {
		t.Error("body should contain action type")
	}
	if strings.Contains(body, "_catch_all") {
		t.Error("export should not include catch-all")
	}
}

func TestRulesImportHandler(t *testing.T) {
	database := openWebTestDB(t)
	repo := db.NewRulesRepo(database)

	jsonBody := `{"rules":[{"name":"Imported Rule","description":"test","priority":1,"enabled":true,"operator":"AND","conditions":[{"field":"subject","operator":"contains","value":"test"}],"actions":[{"type":"mark_as_read","value":""}]}]}`
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("rules_file", "rules.json")
	part.Write([]byte(jsonBody))
	writer.Close()

	req := httptest.NewRequest("POST", "/settings/rules/import", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	rulesImportHandler(repo).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	rules, _ := repo.List()
	found := false
	for _, r := range rules {
		if r.Name == "Imported Rule" {
			found = true
			break
		}
	}
	if !found {
		t.Error("imported rule not found in repo")
	}
}

func TestRulesReorderHandler(t *testing.T) {
	database := openWebTestDB(t)
	repo := db.NewRulesRepo(database)
	r1 := &db.Rule{Name: "A", Priority: 0, Enabled: true}
	r2 := &db.Rule{Name: "B", Priority: 1, Enabled: true}
	repo.Create(r1)
	repo.Create(r2)

	h := rulesReorderHandler(repo, db.NewFoldersRepo(database))
	form := url.Values{"rule_ids": []string{fmt.Sprintf("%d", r2.ID), fmt.Sprintf("%d", r1.ID)}}
	req := httptest.NewRequest("POST", "/rules/reorder", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	rules, _ := repo.List()
	if rules[0].Name != "B" {
		t.Errorf("first rule = %q, want B", rules[0].Name)
	}
}

func TestParseConditions(t *testing.T) {
	rule := &db.Rule{}
	form := url.Values{
		"cond_field": {"from", "subject"},
		"cond_op":    {"equals", "contains"},
		"cond_value": {"alice@example.com", "invoice"},
	}
	req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	parseConditions(req, rule)

	if len(rule.Groups) != 1 {
		t.Fatalf("Groups len = %d, want 1", len(rule.Groups))
	}
	if len(rule.Groups[0].Conditions) != 2 {
		t.Errorf("Conditions len = %d, want 2", len(rule.Groups[0].Conditions))
	}
}

func TestParseActions(t *testing.T) {
	rule := &db.Rule{}
	form := url.Values{
		"action_type":  {"move_to_folder"},
		"action_value": {"Inbox"},
	}
	req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	parseActions(req, rule)

	if len(rule.Actions) != 1 {
		t.Fatalf("Actions len = %d, want 1", len(rule.Actions))
	}
	if rule.Actions[0].Type != "move_to_folder" {
		t.Errorf("Action type = %q", rule.Actions[0].Type)
	}
}

func TestParseConditionsEmpty(t *testing.T) {
	rule := &db.Rule{}
	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	parseConditions(req, rule)

	if len(rule.Groups) != 0 {
		t.Errorf("Groups len = %d, want 0", len(rule.Groups))
	}
}

func TestCreateRuleWithSchedule(t *testing.T) {
	database := openWebTestDB(t)
	repo := db.NewRulesRepo(database)
	settingsRepo := db.NewSettingsRepo(database)

	h := rulesCreateHandler(repo, settingsRepo)
	form := url.Values{
		"name":           {"Scheduled Rule"},
		"schedule_days":  {"mon", "wed", "fri"},
		"schedule_start": {"09:00"},
		"schedule_end":   {"17:00"},
	}
	req := httptest.NewRequest("POST", "/rules", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rec.Code)
	}

	rules, _ := repo.List()
	var found *db.Rule
	for i := range rules {
		if rules[i].Name == "Scheduled Rule" {
			found = &rules[i]
			break
		}
	}
	if found == nil {
		t.Fatal("scheduled rule not found")
	}
	if found.ScheduleDays != "mon,wed,fri" {
		t.Errorf("ScheduleDays = %q, want mon,wed,fri", found.ScheduleDays)
	}
	if found.ScheduleStart != "09:00" {
		t.Errorf("ScheduleStart = %q, want 09:00", found.ScheduleStart)
	}
	if found.ScheduleEnd != "17:00" {
		t.Errorf("ScheduleEnd = %q, want 17:00", found.ScheduleEnd)
	}
}

func TestCreateRulePartialSchedule(t *testing.T) {
	database := openWebTestDB(t)
	repo := db.NewRulesRepo(database)
	settingsRepo := db.NewSettingsRepo(database)

	h := rulesCreateHandler(repo, settingsRepo)
	form := url.Values{
		"name":          {"Partial Schedule"},
		"schedule_days": {"mon", "tue"},
	}
	req := httptest.NewRequest("POST", "/rules", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rec.Code)
	}

	rules, _ := repo.List()
	var found *db.Rule
	for i := range rules {
		if rules[i].Name == "Partial Schedule" {
			found = &rules[i]
			break
		}
	}
	if found == nil {
		t.Fatal("partial schedule rule not found")
	}
	if found.ScheduleDays != "mon,tue" {
		t.Errorf("ScheduleDays = %q, want mon,tue", found.ScheduleDays)
	}
	if found.ScheduleStart != "" {
		t.Errorf("ScheduleStart = %q, want empty", found.ScheduleStart)
	}
	if found.ScheduleEnd != "" {
		t.Errorf("ScheduleEnd = %q, want empty", found.ScheduleEnd)
	}
}

func TestRuleTestSynthetic(t *testing.T) {
	database := openWebTestDB(t)
	rulesRepo := db.NewRulesRepo(database)

	rule := &db.Rule{Name: "test-rule", Enabled: true, Priority: 10}
	rule.Groups = []db.ConditionGroup{{
		Operator:   "AND",
		Conditions: []db.Condition{{Field: "from", Operator: "contains", Value: "@test.com"}},
	}}
	rulesRepo.Create(rule)

	h := rulesTestHandler(rulesRepo, nil)
	form := url.Values{"from": {"user@test.com"}, "subject": {"hello"}}
	req := httptest.NewRequest(http.MethodPost, "/rules/"+strconv.FormatInt(rule.ID, 10)+"/test", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.FormatInt(rule.ID, 10))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := serveHandler(h, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Matched") {
		t.Error("response should contain 'Matched'")
	}
	if !strings.Contains(rec.Body.String(), "from") {
		t.Error("response should contain condition field 'from'")
	}
}

func TestRuleTestSyntheticNoMatch(t *testing.T) {
	database := openWebTestDB(t)
	rulesRepo := db.NewRulesRepo(database)

	rule := &db.Rule{Name: "test-rule", Enabled: true, Priority: 10}
	rule.Groups = []db.ConditionGroup{{
		Operator:   "AND",
		Conditions: []db.Condition{{Field: "from", Operator: "equals", Value: "admin@test.com"}},
	}}
	rulesRepo.Create(rule)

	h := rulesTestHandler(rulesRepo, nil)
	form := url.Values{"from": {"user@other.com"}, "subject": {"hello"}}
	req := httptest.NewRequest(http.MethodPost, "/rules/"+strconv.FormatInt(rule.ID, 10)+"/test", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.FormatInt(rule.ID, 10))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := serveHandler(h, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "No Match") {
		t.Error("response should contain 'No Match'")
	}
}

func TestRuleTestRealMessage(t *testing.T) {
	database := openWebTestDB(t)
	rulesRepo := db.NewRulesRepo(database)

	rule := &db.Rule{Name: "real-test", Enabled: true, Priority: 10}
	rule.Groups = []db.ConditionGroup{{
		Operator:   "AND",
		Conditions: []db.Condition{{Field: "from", Operator: "contains", Value: "@test.com"}},
	}}
	rulesRepo.Create(rule)

	mockClient := &mockIMAPClient{
		searchUIDs: []goimap.UID{123},
		messages: map[uint32]*imap.Message{
			123: {
				UID:     123,
				Subject: "Test Message",
				From:    []imap.Address{{Email: "sender@test.com"}},
				To:      []imap.Address{{Email: "recipient@test.com"}},
			},
		},
	}

	h := rulesTestMessageHandler(rulesRepo, mockClient)
	form := url.Values{"folder": {"INBOX"}}
	req := httptest.NewRequest(http.MethodPost, "/rules/"+strconv.FormatInt(rule.ID, 10)+"/test-message", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.FormatInt(rule.ID, 10))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := serveHandler(h, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Matched") {
		t.Error("response should contain 'Matched'")
	}
}

func TestRuleTestRealMessageNoMessages(t *testing.T) {
	database := openWebTestDB(t)
	rulesRepo := db.NewRulesRepo(database)

	rule := &db.Rule{Name: "real-test", Enabled: true, Priority: 10}
	rulesRepo.Create(rule)

	mockClient := &mockIMAPClient{
		searchUIDs: nil,
	}

	h := rulesTestMessageHandler(rulesRepo, mockClient)
	form := url.Values{"folder": {"Empty"}}
	req := httptest.NewRequest(http.MethodPost, "/rules/"+strconv.FormatInt(rule.ID, 10)+"/test-message", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.FormatInt(rule.ID, 10))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := serveHandler(h, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "No messages found") {
		t.Error("response should contain 'No messages found'")
	}
}

func TestRulesApplyHandlerRequiresFolder(t *testing.T) {
	database := openWebTestDB(t)
	repo := db.NewRulesRepo(database)

	h := rulesApplyHandler(repo, nil)
	form := url.Values{}
	req := httptest.NewRequest("POST", "/rules/apply", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestRulesApplyStatusHandlerNotFound(t *testing.T) {
	h := rulesApplyStatusHandler()
	req := httptest.NewRequest("GET", "/rules/apply/status?id=nonexistent", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestRuleTestNotFound(t *testing.T) {
	database := openWebTestDB(t)
	rulesRepo := db.NewRulesRepo(database)

	h := rulesTestHandler(rulesRepo, nil)
	req := httptest.NewRequest(http.MethodPost, "/rules/9999/test", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "9999")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := serveHandler(h, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}
