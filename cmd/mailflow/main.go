package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/mojoaar/icloud-mailflow/internal/config"
	"github.com/mojoaar/icloud-mailflow/internal/contacts"
	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/imap"
	"github.com/mojoaar/icloud-mailflow/internal/poller"
	"github.com/mojoaar/icloud-mailflow/internal/web"
)

func main() {
	dataDir := flag.String("data", "./data", "Data directory for config, db, and logs")
	flag.Parse()

	if err := os.MkdirAll(*dataDir, 0700); err != nil {
		slog.Error("failed to create data directory", "error", err)
		os.Exit(1)
	}

	cfg, err := config.Load(*dataDir)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	dbPath := *dataDir + "/mailflow.db"
	database, err := db.Open(dbPath)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	rulesRepo := db.NewRulesRepo(database)
	if err := rulesRepo.EnsureCatchAll(); err != nil {
		slog.Warn("failed to ensure catch-all rule", "error", err)
	}

	var imapClient *imap.IMAPClient
	settingsRepo := db.NewSettingsRepo(database)

	imapEmail, _ := settingsRepo.Get("imap_email")
	imapPassword, _ := settingsRepo.Get("imap_password")

	if imapEmail != "" && imapPassword != "" {
		cfg.IMAPEmail = imapEmail
		cfg.IMAPPassword = imapPassword
		imapClient = imap.New(cfg)
		if err := imapClient.Connect(); err != nil {
			slog.Warn("imap connection failed, server will start without mail processing", "error", err)
			imapClient = nil
		}
	}

	contactsRepo := db.NewContactsRepo(database)
	contactsCollector := contacts.NewCollector(contactsRepo, imapClient)

	var p *poller.Poller
	if imapClient != nil {
		p = poller.NewPoller(imapClient, rulesRepo, contactsCollector, cfg.PollInterval, cfg.SourceFolder)
		p.Start()
		defer p.Stop()
	}

	router := web.New(cfg, database, imapClient, p)

	server := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: router,
	}

	go func() {
		slog.Info("server starting", "addr", cfg.ListenAddr)
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
	if imapClient != nil {
		imapClient.Close()
	}
}
