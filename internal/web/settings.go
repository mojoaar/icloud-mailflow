package web

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"

	"github.com/mojoaar/icloud-mailflow/internal/carddav"
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

func settingsPage(settingsRepo *db.SettingsRepo, foldersRepo *db.FoldersRepo, cfg *config.Config, imapClient imap.Client, version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		folders, _ := foldersRepo.List()
		if len(folders) == 0 {
			if imapClient != nil {
				if imapFolders, err := imapClient.ListFolders(); err == nil {
					syncFoldersToDB(imapFolders, foldersRepo)
					folders, _ = foldersRepo.List()
					ensureFolder(imapClient, cfg.SourceFolder, foldersRepo)
				}
			} else {
				email, _ := settingsRepo.Get("imap_email")
				password, _ := settingsRepo.Get("imap_password")
				if email != "" && password != "" {
					cfg.IMAPEmail = email
					cfg.IMAPPassword = password
					temp := imap.New(cfg)
					if err := temp.Connect(); err == nil {
						if imapFolders, err := temp.ListFolders(); err == nil {
							syncFoldersToDB(imapFolders, foldersRepo)
							folders, _ = foldersRepo.List()
							ensureFolder(temp, cfg.SourceFolder, foldersRepo)
						}
						temp.Close()
					}
				}
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
			"Version":      version,
		}
		renderPage(w, r, "Settings", "settings", data)
	}
}

func settingsTestIMAP(cfg *config.Config, settingsRepo *db.SettingsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		email := r.FormValue("imap_email")
		password := r.FormValue("imap_password")
		if email == "" {
			email, _ = settingsRepo.Get("imap_email")
		}
		if password == "" {
			password, _ = settingsRepo.Get("imap_password")
		}
		if email == "" || password == "" {
			renderPartial(w, "toast", map[string]string{"Type": "error", "Message": "Email and password are required"})
			return
		}
		cfg.IMAPEmail = email
		cfg.IMAPPassword = password
		temp := imap.New(cfg)
		if err := temp.Connect(); err != nil {
			renderPartial(w, "toast", map[string]string{"Type": "error", "Message": err.Error()})
			return
		}
		temp.Close()
		renderPartial(w, "toast", map[string]string{"Type": "success", "Message": "Connection successful"})
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

func syncFoldersToDB(imapFolders []imap.Folder, foldersRepo *db.FoldersRepo) {
	var dbFolders []db.Folder
	for _, f := range imapFolders {
		dbFolders = append(dbFolders, db.Folder{
			Name:  f.Name,
			Path:  f.Path,
			Flags: f.Flags,
		})
	}
	foldersRepo.Sync(dbFolders)
}

func ensureFolder(client imap.Client, name string, foldersRepo *db.FoldersRepo) {
	if name == "" || name == "INBOX" {
		return
	}
	dbFolders, err := foldersRepo.List()
	if err != nil {
		return
	}
	for _, f := range dbFolders {
		if f.Name == name {
			return
		}
	}
	if err := client.CreateFolder(name); err == nil {
		if imapFolders, err := client.ListFolders(); err == nil {
			syncFoldersToDB(imapFolders, foldersRepo)
		}
	}
}

func settingsSavePoll(cfg *config.Config, settingsRepo *db.SettingsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		cfg.SourceFolder = r.FormValue("source_folder")
		cfg.PollInterval, _ = strconv.Atoi(r.FormValue("poll_interval"))
		cfg.Save()
		settingsRepo.Set("source_folder", cfg.SourceFolder)
		settingsRepo.Set("poll_interval", strconv.Itoa(cfg.PollInterval))
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
	}
}

func carddavImportHandler(settingsRepo *db.SettingsRepo, contactsRepo *db.ContactsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email, _ := settingsRepo.Get("imap_email")
		password, _ := settingsRepo.Get("imap_password")
		if email == "" || password == "" {
			renderPartial(w, "toast", map[string]string{"Type": "error", "Message": "IMAP not configured"})
			return
		}
		importer := carddav.NewImporter(contactsRepo)
		count, err := importer.ImportFromiCloud(email, password)
		if err != nil {
			renderPartial(w, "toast", map[string]string{"Type": "error", "Message": err.Error()})
			return
		}
		if count == 0 {
			renderPartial(w, "toast", map[string]string{"Type": "success", "Message": "No new contacts found"})
			return
		}
		renderPartial(w, "toast", map[string]string{"Type": "success", "Message": fmt.Sprintf("Imported %d contacts from iCloud", count)})
	}
}
