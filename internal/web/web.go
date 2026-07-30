package web

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"io/fs"
	"net/http"
	"strings"
	"sync"
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
	appVersion = version
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Timeout(30 * time.Second))

	settingsRepo := db.NewSettingsRepo(d)
	if v, _ := settingsRepo.Get("font_mono"); v != "false" {
		useMonoFont = true
	}
	sessRepo := db.NewSessionsRepo(d)
	rulesRepo := db.NewRulesRepo(d)
	foldersRepo := db.NewFoldersRepo(d)

	contactsRepo := db.NewContactsRepo(d)

	r.Use(authMiddleware(sessRepo))
	r.Use(csrfMiddleware)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "POST" {
				r.ParseForm()
				if m := r.Form.Get("_method"); m != "" {
					r.Method = strings.ToUpper(m)
				}
			}
			next.ServeHTTP(w, r)
		})
	})

	subFS, _ := fs.Sub(staticFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(subFS))))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	})

	r.Get("/login", loginPage(settingsRepo, sessRepo))
	r.Post("/login", loginPage(settingsRepo, sessRepo))
	r.Get("/logout", logoutHandler(sessRepo))

	r.Get("/setup", setupPage(settingsRepo, d, cfg))
	r.Post("/setup", setupPage(settingsRepo, d, cfg))

	r.Get("/dashboard", dashboardHandler(imapClient, p, rulesRepo, foldersRepo, settingsRepo, contactsRepo, cfg))
	r.Post("/poller/tick", pollerTickHandler(p))

	r.Get("/activity", activityHandler(logRepo, settingsRepo))
	r.Post("/activity/delete", activityDeleteHandler(logRepo))

	r.Get("/rules", rulesListHandler(rulesRepo))
	r.Get("/rules/new", rulesNewHandler(foldersRepo, contactsRepo))
	r.Post("/rules", rulesCreateHandler(rulesRepo))
	r.Get("/rules/{id}/edit", rulesEditHandler(rulesRepo, foldersRepo, contactsRepo))
	r.Put("/rules/{id}", rulesUpdateHandler(rulesRepo))
	r.Get("/rules/{id}/delete", rulesDeleteConfirmHandler(rulesRepo))
	r.Delete("/rules/{id}", rulesDeleteHandler(rulesRepo))
	r.Post("/rules/reorder", rulesReorderHandler(rulesRepo))

	r.Get("/settings", settingsPage(settingsRepo, foldersRepo, cfg, imapClient, version, contactsRepo))
	r.Post("/settings/imap", settingsSaveIMAP(cfg, settingsRepo))
	r.Post("/settings/imap/test", settingsTestIMAP(cfg, settingsRepo))
	r.Post("/settings/password", settingsSavePassword(settingsRepo))
	r.Post("/settings/poll", settingsSavePoll(cfg, settingsRepo))
	r.Post("/settings/carddav-import", carddavImportHandler(settingsRepo, cfg, contactsRepo))
	r.Post("/settings/poll/toggle", settingsTogglePolling(settingsRepo, p))
	r.Get("/settings/rules/export", rulesExportHandler(rulesRepo))
	r.Post("/settings/rules/import", rulesImportHandler(rulesRepo))
	r.Post("/settings/timezone", settingsSaveTimezone(settingsRepo))
	r.Post("/settings/font", settingsSaveFont(settingsRepo))

	r.Get("/api/contacts", contactsSearchHandler(db.NewContactsRepo(d)))
	r.Get("/api/folders", foldersListHandler(imapClient, foldersRepo))
	r.Post("/contacts/seed", seedContactsHandler(collector, foldersRepo, contactsRepo))

	return r
}

var csrfMiddleware = func(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" || r.Method == "HEAD" || r.Header.Get("HX-Request") == "true" ||
			strings.HasPrefix(r.URL.Path, "/login") || strings.HasPrefix(r.URL.Path, "/setup") {
			next.ServeHTTP(w, r)
			return
		}
		token := r.FormValue("csrf_token")
		cookie, _ := r.Cookie("mailflow_csrf")
		if cookie == nil || token == "" || cookie.Value != token {
			http.Error(w, "invalid csrf token", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func csrfToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func csrfCookie() *http.Cookie {
	return &http.Cookie{
		Name:     "mailflow_csrf",
		Value:    csrfToken(),
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
		HttpOnly: false,
	}
}

func csrfCookieWithToken(token string) *http.Cookie {
	return &http.Cookie{
		Name:     "mailflow_csrf",
		Value:    token,
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
		HttpOnly: false,
	}
}

type rateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rateEntry
}

type rateEntry struct {
	count    int
	resetAt  time.Time
}

func newRateLimiter() *rateLimiter {
	rl := &rateLimiter{entries: map[string]*rateEntry{}}
	go func() {
		for {
			time.Sleep(time.Minute)
			rl.mu.Lock()
			now := time.Now()
			for ip, e := range rl.entries {
				if now.After(e.resetAt) {
					delete(rl.entries, ip)
				}
			}
			rl.mu.Unlock()
		}
	}()
	return rl
}

func (rl *rateLimiter) allow(ip string, max int, window time.Duration) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	e, ok := rl.entries[ip]
	if !ok || now.After(e.resetAt) {
		rl.entries[ip] = &rateEntry{count: 1, resetAt: now.Add(window)}
		return true
	}
	e.count++
	return e.count <= max
}

var loginLimiter = newRateLimiter()
