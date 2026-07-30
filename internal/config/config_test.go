package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.IMAPServer != "imap.mail.me.com" {
		t.Errorf("IMAPServer = %q, want imap.mail.me.com", cfg.IMAPServer)
	}
	if cfg.IMAPPort != 993 {
		t.Errorf("IMAPPort = %d, want 993", cfg.IMAPPort)
	}
	if cfg.SourceFolder != "Processing" {
		t.Errorf("SourceFolder = %q, want Processing", cfg.SourceFolder)
	}
	if cfg.PollInterval != 300 {
		t.Errorf("PollInterval = %d, want 300", cfg.PollInterval)
	}
	if cfg.ListenAddr != "0.0.0.0:8080" {
		t.Errorf("ListenAddr = %q, want 0.0.0.0:8080", cfg.ListenAddr)
	}
}

func TestLoadMissingCreatesDefault(t *testing.T) {
	dir := t.TempDir()

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.IMAPServer != "imap.mail.me.com" {
		t.Errorf("expected default IMAPServer, got %q", cfg.IMAPServer)
	}

	if cfg.DataDir != dir {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, dir)
	}

	path := filepath.Join(dir, "config.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("config.json should have been created")
	}
}

func TestLoadExistingConfig(t *testing.T) {
	dir := t.TempDir()

	cfg := Default()
	cfg.IMAPServer = "custom.example.com"
	cfg.IMAPPort = 143
	cfg.PollInterval = 120
	cfg.DataDir = dir

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), data, 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.IMAPServer != "custom.example.com" {
		t.Errorf("IMAPServer = %q, want custom.example.com", loaded.IMAPServer)
	}
	if loaded.IMAPPort != 143 {
		t.Errorf("IMAPPort = %d, want 143", loaded.IMAPPort)
	}
	if loaded.PollInterval != 120 {
		t.Errorf("PollInterval = %d, want 120", loaded.PollInterval)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("not json"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := Load(dir)
	if err == nil {
		t.Fatal("Load should fail with invalid JSON")
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()

	cfg := Default()
	cfg.DataDir = dir
	cfg.IMAPServer = "roundtrip.example.com"
	cfg.IMAPPort = 993
	cfg.IMAPEmail = "user@example.com"
	cfg.IMAPPassword = "secret"
	cfg.SourceFolder = "Inbox"
	cfg.PollInterval = 120

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.IMAPServer != cfg.IMAPServer {
		t.Errorf("IMAPServer = %q, want %q", loaded.IMAPServer, cfg.IMAPServer)
	}
	if loaded.IMAPPort != cfg.IMAPPort {
		t.Errorf("IMAPPort = %d, want %d", loaded.IMAPPort, cfg.IMAPPort)
	}
	if loaded.IMAPEmail != cfg.IMAPEmail {
		t.Errorf("IMAPEmail = %q, want %q", loaded.IMAPEmail, cfg.IMAPEmail)
	}
	if loaded.IMAPPassword != cfg.IMAPPassword {
		t.Errorf("IMAPPassword = %q, want %q", loaded.IMAPPassword, cfg.IMAPPassword)
	}
	if loaded.SourceFolder != cfg.SourceFolder {
		t.Errorf("SourceFolder = %q, want %q", loaded.SourceFolder, cfg.SourceFolder)
	}
	if loaded.PollInterval != cfg.PollInterval {
		t.Errorf("PollInterval = %d, want %d", loaded.PollInterval, cfg.PollInterval)
	}
}

func TestLoadPreservesRuntimeFields(t *testing.T) {
	dir := t.TempDir()

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.DataDir != dir {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, dir)
	}
	if cfg.ListenAddr != "0.0.0.0:8080" {
		t.Errorf("ListenAddr = %q, want 0.0.0.0:8080", cfg.ListenAddr)
	}
	if cfg.AdminPass != "" {
		t.Error("AdminPass should be empty after Load")
	}
}
