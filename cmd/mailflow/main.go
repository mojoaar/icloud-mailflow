package main

import (
	"database/sql"
	"encoding/hex"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/mojoaar/icloud-mailflow/internal/config"
	"github.com/mojoaar/icloud-mailflow/internal/contacts"
	"github.com/mojoaar/icloud-mailflow/internal/crypto"
	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/imap"
	"github.com/mojoaar/icloud-mailflow/internal/poller"
	"github.com/mojoaar/icloud-mailflow/internal/web"
)

var version = "0.3.1"

type App struct {
	Config     *config.Config
	DB         *sql.DB
	ImapConn   *imap.IMAPClient
	ImapClient imap.Client
	Poller     *poller.Poller
	Router     http.Handler
}

func (a *App) Close() {
	if a.Poller != nil {
		a.Poller.Stop()
	}
	if a.ImapConn != nil {
		a.ImapConn.Close()
	}
	if a.DB != nil {
		a.DB.Close()
	}
}

func initialize(dataDir string) (*App, error) {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, err
	}

	cfg, err := config.Load(dataDir)
	if err != nil {
		return nil, err
	}

	dbPath := dataDir + "/mailflow.db"
	database, err := db.Open(dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Migrate(database); err != nil {
		database.Close()
		return nil, err
	}

	rulesRepo := db.NewRulesRepo(database)
	if err := rulesRepo.EnsureCatchAll(); err != nil {
		database.Close()
		return nil, err
	}

	db.NewLogRepo(database).Cleanup(1000)

	var imapConn *imap.IMAPClient
	var imapClient imap.Client
	settingsRepo := db.NewSettingsRepo(database)

	imapEmail, _ := settingsRepo.Get("imap_email")
	storedPassword, _ := settingsRepo.Get("imap_password")

	if imapEmail != "" && storedPassword != "" {
		imapPassword := storedPassword
		if cfg.EncryptionKey != "" {
			if key, err := hex.DecodeString(cfg.EncryptionKey); err == nil {
				if dec, err := crypto.Decrypt([]byte(storedPassword), key); err == nil {
					imapPassword = string(dec)
				}
			}
		}
		cfg.IMAPEmail = imapEmail
		cfg.IMAPPassword = imapPassword
		imapConn = imap.New(cfg)
		if err := imapConn.Connect(); err != nil {
			slog.Warn("imap connection failed, server will start without mail processing", "error", err)
			imapConn = nil
		}
		imapClient = imapConn
		imapClient.CreateFolder(cfg.SourceFolder)
	}

	contactsRepo := db.NewContactsRepo(database)
	contactsCollector := contacts.NewCollector(contactsRepo, imapClient)
	logRepo := db.NewLogRepo(database)

	var p *poller.Poller
	if imapClient != nil {
		p = poller.NewPoller(imapClient, rulesRepo, contactsCollector, logRepo, cfg.PollInterval, cfg.SourceFolder)
		p.Start()
	}

	router := web.New(cfg, database, imapClient, contactsCollector, logRepo, version, p)

	return &App{
		Config:     cfg,
		DB:         database,
		ImapConn:   imapConn,
		ImapClient: imapClient,
		Poller:     p,
		Router:     router,
	}, nil
}

func main() {
	dataDir := flag.String("data", "./data", "Data directory for config, db, and logs")
	flag.Parse()

	app, err := initialize(*dataDir)
	if err != nil {
		slog.Error("failed to initialize", "error", err)
		os.Exit(1)
	}
	defer app.Close()

	server := &http.Server{
		Addr:    app.Config.ListenAddr,
		Handler: app.Router,
	}

	go func() {
		slog.Info("server starting", "addr", app.Config.ListenAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down")
	server.Close()
}
