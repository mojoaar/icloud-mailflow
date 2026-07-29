package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	IMAPServer    string `json:"imap_server"`
	IMAPPort      int    `json:"imap_port"`
	IMAPEmail     string `json:"imap_email"`
	IMAPPassword  string `json:"imap_password"`
	SourceFolder  string `json:"source_folder"`
	PollInterval  int    `json:"poll_interval"`
	EncryptionKey string `json:"encryption_key"`
	AdminPass     string `json:"-"`
	DataDir       string `json:"-"`
	ListenAddr    string `json:"-"`
}

func Default() *Config {
	return &Config{
		IMAPServer:   "imap.mail.me.com",
		IMAPPort:     993,
		SourceFolder: "Processing",
		PollInterval: 60,
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
			cfg.EncryptionKey = generateKey()
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
	if cfg.EncryptionKey == "" {
		cfg.EncryptionKey = generateKey()
		cfg.Save()
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func generateKey() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (c *Config) Validate() error {
	if c.IMAPPort < 1 || c.IMAPPort > 65535 {
		return fmt.Errorf("invalid IMAP port: %d", c.IMAPPort)
	}
	if c.PollInterval < 10 {
		return fmt.Errorf("poll interval too low: %d (min 10)", c.PollInterval)
	}
	if c.ListenAddr == "" {
		return fmt.Errorf("listen address is required")
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
