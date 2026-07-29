package web

import (
	"net/http"

	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/imap"
	"github.com/mojoaar/icloud-mailflow/internal/poller"
)

func dashboardHandler(imapClient *imap.IMAPClient, p *poller.Poller, rulesRepo *db.RulesRepo, foldersRepo *db.FoldersRepo, settingsRepo *db.SettingsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rules, err := rulesRepo.List()
		if err != nil {
			rules = []db.Rule{}
		}
		folders, err := foldersRepo.List()
		if err != nil {
			folders = []db.Folder{}
		}
		status := "disconnected"
		if imapClient != nil {
			status = "connected"
		}
		passwordSet, _ := settingsRepo.Get("admin_password_hash")
		sourceFolder, _ := settingsRepo.Get("source_folder")
		pollInterval, _ := settingsRepo.Get("poll_interval")
		imapEmail, _ := settingsRepo.Get("imap_email")

		data := map[string]any{
			"Rules":        rules,
			"Folders":      folders,
			"Status":       status,
			"Configured":   passwordSet != "" && imapEmail != "",
			"SourceFolder": sourceFolder,
			"PollInterval": pollInterval,
			"IMAPEmail":    imapEmail,
		}
		renderPage(w, r, "Dashboard", "dashboard", data)
	}
}

func pollerTickHandler(p *poller.Poller) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := p.Tick(); err != nil {
			renderPartial(w, "toast", map[string]string{"Type": "error", "Message": err.Error()})
			return
		}
		renderPartial(w, "toast", map[string]string{"Type": "success", "Message": "Poll complete"})
	}
}
