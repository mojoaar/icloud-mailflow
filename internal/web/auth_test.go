package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mojoaar/icloud-mailflow/internal/config"
	"github.com/mojoaar/icloud-mailflow/internal/crypto"
	"github.com/mojoaar/icloud-mailflow/internal/db"
)

func TestAuthMiddlewareAllowsLogin(t *testing.T) {
	database := openWebTestDB(t)
	sessRepo := db.NewSessionsRepo(database)

	mw := authMiddleware(sessRepo)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/login", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestAuthMiddlewareAllowsSetup(t *testing.T) {
	database := openWebTestDB(t)
	sessRepo := db.NewSessionsRepo(database)

	mw := authMiddleware(sessRepo)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/setup", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestAuthMiddlewareAllowsStatic(t *testing.T) {
	database := openWebTestDB(t)
	sessRepo := db.NewSessionsRepo(database)

	mw := authMiddleware(sessRepo)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/static/style.css", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestAuthMiddlewareRedirectsUnauthenticated(t *testing.T) {
	database := openWebTestDB(t)
	sessRepo := db.NewSessionsRepo(database)

	mw := authMiddleware(sessRepo)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/dashboard", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rec.Code)
	}
}

func TestAuthMiddlewareAllowsWithValidCookie(t *testing.T) {
	database := openWebTestDB(t)
	sessRepo := db.NewSessionsRepo(database)

	token, _ := generateToken()
	sessRepo.Create(token, time.Hour)

	mw := authMiddleware(sessRepo)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestLoginPageGet(t *testing.T) {
	database := openWebTestDB(t)
	settingsRepo := db.NewSettingsRepo(database)
	sessRepo := db.NewSessionsRepo(database)

	h := loginPage(settingsRepo, sessRepo)
	req := httptest.NewRequest("GET", "/login", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestLoginPagePostInvalidPassword(t *testing.T) {
	database := openWebTestDB(t)
	settingsRepo := db.NewSettingsRepo(database)
	sessRepo := db.NewSessionsRepo(database)

	hash, _ := crypto.HashPassword("correct")
	settingsRepo.Set("admin_password_hash", hash)

	h := loginPage(settingsRepo, sessRepo)
	form := url.Values{"password": {"wrong"}}
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestLoginPagePostValidPassword(t *testing.T) {
	database := openWebTestDB(t)
	settingsRepo := db.NewSettingsRepo(database)
	sessRepo := db.NewSessionsRepo(database)

	hash, _ := crypto.HashPassword("correct")
	settingsRepo.Set("admin_password_hash", hash)

	h := loginPage(settingsRepo, sessRepo)
	form := url.Values{"password": {"correct"}}
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rec.Code)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Error("should set a session cookie")
	}
}

func TestLogoutHandler(t *testing.T) {
	database := openWebTestDB(t)
	sessRepo := db.NewSessionsRepo(database)

	token, _ := generateToken()
	sessRepo.Create(token, time.Hour)

	h := logoutHandler(sessRepo)
	req := httptest.NewRequest("GET", "/logout", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rec.Code)
	}
}

func TestSetupPageRedirectsWhenConfigured(t *testing.T) {
	database := openWebTestDB(t)
	settingsRepo := db.NewSettingsRepo(database)
	settingsRepo.Set("admin_password_hash", "somehash")
	settingsRepo.Set("imap_email", "user@example.com")

	h := setupPage(settingsRepo, database, &config.Config{})
	req := httptest.NewRequest("GET", "/setup", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rec.Code)
	}
}

func TestSetupPageShowsWhenUnconfigured(t *testing.T) {
	database := openWebTestDB(t)
	settingsRepo := db.NewSettingsRepo(database)

	h := setupPage(settingsRepo, database, &config.Config{})
	req := httptest.NewRequest("GET", "/setup", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}
