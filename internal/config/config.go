package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

type Config struct {
	IMAPServer    string `json:"imap_server"`
	IMAPPort      int    `json:"imap_port"`
	IMAPEmail     string `json:"imap_email"`
	IMAPPassword  string `json:"-"`
	SourceFolder  string `json:"source_folder"`
	PollInterval  int    `json:"poll_interval"`
	EncryptionKey string `json:"encryption_key"`
	DataDir       string `json:"-"`
	ListenAddr    string `json:"-"`

	legacyIMAPPassword        string
	legacyIMAPPasswordPresent bool
}

// LegacyIMAPPassword returns a plaintext IMAP password found in a pre-migration
// config.json. It is never persisted; the caller moves it into the encrypted store.
func (c *Config) LegacyIMAPPassword() string { return c.legacyIMAPPassword }

// LegacyIMAPPasswordPresent reports whether config.json contained an
// imap_password key (even empty), so the caller can scrub it.
func (c *Config) LegacyIMAPPasswordPresent() bool { return c.legacyIMAPPasswordPresent }

func Default() *Config {
	return &Config{
		IMAPServer:   "imap.mail.me.com",
		IMAPPort:     993,
		SourceFolder: "Processing",
		PollInterval: 300,
		ListenAddr:   "0.0.0.0:8080",
	}
}

func Load(dataDir string) (*Config, error) {
	cfg := Default()
	cfg.DataDir = dataDir

	path := filepath.Join(dataDir, "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			key, err := generateKey()
			if err != nil {
				return nil, err
			}
			cfg.EncryptionKey = key
			if err := cfg.Save(); err != nil {
				return nil, err
			}
			return cfg, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawMap); err == nil {
		if v, ok := rawMap["imap_password"]; ok {
			cfg.legacyIMAPPasswordPresent = true
			_ = json.Unmarshal(v, &cfg.legacyIMAPPassword)
		}
	}
	if cfg.EncryptionKey == "" {
		key, err := generateKey()
		if err != nil {
			return nil, err
		}
		cfg.EncryptionKey = key
		if err := cfg.Save(); err != nil {
			return nil, err
		}
	}
	if err := ValidatePollInterval(cfg.PollInterval); err != nil {
		slog.Warn("invalid poll interval in config, using default", "value", cfg.PollInterval, "default", Default().PollInterval)
		cfg.PollInterval = Default().PollInterval
		if err := cfg.Save(); err != nil {
			return nil, err
		}
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func generateKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (c *Config) Validate() error {
	if c.IMAPPort < 1 || c.IMAPPort > 65535 {
		return fmt.Errorf("invalid IMAP port: %d", c.IMAPPort)
	}
	if err := ValidatePollInterval(c.PollInterval); err != nil {
		return err
	}
	if c.ListenAddr == "" {
		return fmt.Errorf("listen address is required")
	}
	return nil
}

func ValidatePollInterval(n int) error {
	if n < 60 {
		return fmt.Errorf("poll interval too low: %d (min 60)", n)
	}
	return nil
}

func ValidatePollBatch(n int) error {
	if n < 1 || n > 200 {
		return fmt.Errorf("messages per poll must be 1-200, got %d", n)
	}
	return nil
}

func ValidateLogKeep(n int) error {
	if n < 100 {
		return fmt.Errorf("log retention must be at least 100, got %d", n)
	}
	return nil
}

func (c *Config) Save() error {
	path := filepath.Join(c.DataDir, "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
