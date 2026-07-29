package main

import (
	"net/http/httptest"
	"testing"
)

func TestInitialize(t *testing.T) {
	dir := t.TempDir()
	app, err := initialize(dir)
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	defer app.Close()

	if app.Config == nil {
		t.Fatal("expected config, got nil")
	}
	if app.Config.IMAPServer != "imap.mail.me.com" {
		t.Fatalf("expected default IMAP server, got %s", app.Config.IMAPServer)
	}
}

func TestInitializeRouterResponds(t *testing.T) {
	dir := t.TempDir()
	app, err := initialize(dir)
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	defer app.Close()

	req := httptest.NewRequest("GET", "/login", nil)
	rec := httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestInitializeSetupPage(t *testing.T) {
	dir := t.TempDir()
	app, err := initialize(dir)
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	defer app.Close()

	req := httptest.NewRequest("GET", "/setup", nil)
	rec := httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAppCloseNoPanic(t *testing.T) {
	dir := t.TempDir()
	app, err := initialize(dir)
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	app.Close()
}

func TestAppCloseNilPoller(t *testing.T) {
	app := &App{}
	app.Close()
}
