package main

import (
	"database/sql"
	"encoding/hex"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/mojoaar/icloud-mailflow/internal/config"
	"github.com/mojoaar/icloud-mailflow/internal/contacts"
	"github.com/mojoaar/icloud-mailflow/internal/crypto"
	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/imap"
	"github.com/mojoaar/icloud-mailflow/internal/poller"
	"github.com/mojoaar/icloud-mailflow/internal/web"
)

var version = "0.7.7"

type App struct {
	Config   *config.Config
	DB       *sql.DB
	ImapConn *imap.IMAPClient
	Poller   *poller.Poller
	Router   http.Handler
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
	startTime := time.Now()
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
	slog.Debug("database opened", "path", dbPath)

	if err := db.Migrate(database); err != nil {
		database.Close()
		return nil, err
	}

	rulesRepo := db.NewRulesRepo(database)
	if err := rulesRepo.EnsureCatchAll(); err != nil {
		database.Close()
		return nil, err
	}

	var imapConn *imap.IMAPClient
	var imapClient imap.Client
	settingsRepo := db.NewSettingsRepo(database)
	logKeep := 1000
	if v, _ := settingsRepo.Get("log_keep"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			logKeep = n
		}
	}
	db.NewLogRepo(database).Cleanup(logKeep)

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
		if imapConn != nil {
			imapClient = imapConn
			imapClient.CreateFolder(cfg.SourceFolder)
			slog.Debug("imap connected and ready")
		}
	}

	contactsRepo := db.NewContactsRepo(database)
	contactsCollector := contacts.NewCollector(contactsRepo, imapClient)
	logRepo := db.NewLogRepo(database)
	statsRepo := db.NewStatsRepo(database)
	foldersRepo := db.NewFoldersRepo(database)

	var p *poller.Poller
	if imapClient != nil {
		batchSize := 50
		if v, _ := settingsRepo.Get("poll_batch"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				batchSize = n
			}
		}
		p = poller.NewPoller(imapClient, rulesRepo, contactsCollector,
			logRepo, settingsRepo, statsRepo, foldersRepo, cfg, batchSize, cfg.PollInterval, cfg.SourceFolder,
			imapEmail,
			func() (imap.Client, error) {
				conn := imap.New(cfg)
				if err := conn.Connect(); err != nil {
					return nil, err
				}
				return conn, nil
			},
		)
		if v, _ := settingsRepo.Get("polling_enabled"); v != "false" {
			p.Start()
		}
	}

	router := web.New(cfg, database, imapClient, contactsCollector, logRepo, statsRepo, version, startTime, p)

	return &App{
		Config:     cfg,
		DB:         database,
		ImapConn: imapConn,
		Poller:     p,
		Router:     router,
	}, nil
}

func main() {
	if os.Getenv("LOG_LEVEL") == "debug" {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}
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
