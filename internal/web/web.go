package web

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/mojoaar/icloud-mailflow/internal/config"
	"github.com/mojoaar/icloud-mailflow/internal/contacts"
	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/imap"
	"github.com/mojoaar/icloud-mailflow/internal/poller"
)

func New(cfg *config.Config, d *sql.DB, imapClient imap.Client, collector *contacts.Collector, logRepo *db.LogRepo, version string, p *poller.Poller) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Timeout(30 * time.Second))

	settingsRepo := db.NewSettingsRepo(d)
	sessRepo := db.NewSessionsRepo(d)
	rulesRepo := db.NewRulesRepo(d)
	foldersRepo := db.NewFoldersRepo(d)

	contactsRepo := db.NewContactsRepo(d)

	r.Use(authMiddleware(sessRepo))

	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	r.Get("/login", loginPage(settingsRepo, sessRepo))
	r.Post("/login", loginPage(settingsRepo, sessRepo))
	r.Get("/logout", logoutHandler(sessRepo))

	r.Get("/setup", setupPage(settingsRepo, d))
	r.Post("/setup", setupPage(settingsRepo, d))

	r.Get("/dashboard", dashboardHandler(imapClient, p, rulesRepo, foldersRepo, settingsRepo, contactsRepo, cfg))
	r.Post("/poller/tick", pollerTickHandler(p))

	r.Get("/activity", activityHandler(logRepo))

	r.Get("/rules", rulesListHandler(rulesRepo))
	r.Get("/rules/new", rulesNewHandler(foldersRepo, contactsRepo))
	r.Post("/rules", rulesCreateHandler(rulesRepo))
	r.Get("/rules/{id}/edit", rulesEditHandler(rulesRepo, foldersRepo, contactsRepo))
	r.Put("/rules/{id}", rulesUpdateHandler(rulesRepo))
	r.Get("/rules/{id}/delete", rulesDeleteConfirmHandler(rulesRepo))
	r.Delete("/rules/{id}", rulesDeleteHandler(rulesRepo))
	r.Post("/rules/reorder", rulesReorderHandler(rulesRepo))

	r.Get("/settings", settingsPage(settingsRepo, foldersRepo, cfg, imapClient, version))
	r.Post("/settings/imap", settingsSaveIMAP(cfg))
	r.Post("/settings/imap/test", settingsTestIMAP(cfg, settingsRepo))
	r.Post("/settings/password", settingsSavePassword(settingsRepo))
	r.Post("/settings/poll", settingsSavePoll(cfg, settingsRepo))
	r.Post("/settings/carddav-import", carddavImportHandler(settingsRepo, contactsRepo))

	r.Get("/api/contacts", contactsSearchHandler(db.NewContactsRepo(d)))
	r.Get("/api/folders", foldersListHandler(imapClient, foldersRepo))
	r.Post("/contacts/seed", seedContactsHandler(collector, foldersRepo, contactsRepo))

	return r
}
