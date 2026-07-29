package web

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/mojoaar/icloud-mailflow/internal/config"
	"github.com/mojoaar/icloud-mailflow/internal/crypto"
	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/imap"
)

func setupPage(settingsRepo *db.SettingsRepo, d *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hash, _ := settingsRepo.Get("admin_password_hash")
		email, _ := settingsRepo.Get("imap_email")
		if hash != "" && email != "" {
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
			return
		}
		if r.Method == "POST" {
			r.ParseForm()
			password := r.FormValue("password")
			if password != "" {
				h, _ := crypto.HashPassword(password)
				settingsRepo.Set("admin_password_hash", h)
				db.NewRulesRepo(d).EnsureCatchAll()
			}
			if e := r.FormValue("imap_email"); e != "" {
				settingsRepo.Set("imap_email", e)
			}
			if p := r.FormValue("imap_password"); p != "" {
				settingsRepo.Set("imap_password", p)
			}
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
			return
		}
		renderPage(w, r, "Setup", "setup", nil)
	}
}

func settingsPage(settingsRepo *db.SettingsRepo, foldersRepo *db.FoldersRepo, cfg *config.Config, imapClient *imap.IMAPClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		folders, _ := foldersRepo.List()
		if len(folders) == 0 && imapClient != nil {
			if imapFolders, err := imapClient.ListFolders(); err == nil {
				var dbFolders []db.Folder
				for _, f := range imapFolders {
					dbFolders = append(dbFolders, db.Folder{
						Name:  f.Name,
						Path:  f.Path,
						Flags: f.Flags,
					})
				}
				foldersRepo.Sync(dbFolders)
				folders, _ = foldersRepo.List()
			}
		}
		imapEmail, _ := settingsRepo.Get("imap_email")
		sourceFolder, _ := settingsRepo.Get("source_folder")
		pollInterval, _ := settingsRepo.Get("poll_interval")
		if sourceFolder == "" {
			sourceFolder = cfg.SourceFolder
		}
		if pollInterval == "" {
			pollInterval = strconv.Itoa(cfg.PollInterval)
		}
		data := map[string]any{
			"Folders":      folders,
			"IMAPEmail":    imapEmail,
			"SourceFolder": sourceFolder,
			"PollInterval": pollInterval,
			"ListenAddr":   cfg.ListenAddr,
		}
		renderPage(w, r, "Settings", "settings", data)
	}
}

func settingsSaveIMAP(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		cfg.IMAPEmail = r.FormValue("imap_email")
		cfg.IMAPPassword = r.FormValue("imap_password")
		cfg.Save()
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
	}
}

func settingsSavePassword(settingsRepo *db.SettingsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		password := r.FormValue("password")
		if password != "" {
			hash, _ := crypto.HashPassword(password)
			settingsRepo.Set("admin_password_hash", hash)
		}
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
	}
}

func settingsSavePoll(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		cfg.SourceFolder = r.FormValue("source_folder")
		cfg.PollInterval, _ = strconv.Atoi(r.FormValue("poll_interval"))
		cfg.Save()
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
	}
}
