package web

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/mojoaar/icloud-mailflow/internal/config"
	"github.com/mojoaar/icloud-mailflow/internal/contacts"
	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/imap"
	"github.com/mojoaar/icloud-mailflow/internal/poller"
)

func dashboardHandler(imapClient imap.Client, p *poller.Poller, rulesRepo *db.RulesRepo, foldersRepo *db.FoldersRepo, settingsRepo *db.SettingsRepo, contactsRepo *db.ContactsRepo, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rules, err := rulesRepo.List()
		if err != nil {
			rules = []db.Rule{}
		}
		folders, err := foldersRepo.List()
		if err != nil {
			folders = []db.Folder{}
		}
		contactsCount, _ := contactsRepo.Count()
		totalProcessed, _ := db.NewStatsRepo(rulesRepo.DB).TotalProcessed()
		status := "disconnected"
		if imapClient != nil {
			status = "connected"
		}
		passwordSet, _ := settingsRepo.Get("admin_password_hash")
		sourceFolder, _ := settingsRepo.Get("source_folder")
		pollInterval, _ := settingsRepo.Get("poll_interval")
		imapEmail, _ := settingsRepo.Get("imap_email")
		if sourceFolder == "" {
			sourceFolder = cfg.SourceFolder
		}
		if pollInterval == "" {
			pollInterval = strconv.Itoa(cfg.PollInterval)
		}
		pollingEnabled, _ := settingsRepo.Get("polling_enabled")
		pollingActive := pollingEnabled != "false"
		nextPoll := ""
		if p != nil && !p.LastTick().IsZero() {
			sec, _ := strconv.Atoi(pollInterval)
			if sec == 0 {
				sec = cfg.PollInterval
			}
			t := p.LastTick().Add(time.Duration(sec) * time.Second)
			if tz, _ := settingsRepo.Get("timezone"); tz != "" && tz != "UTC" {
				if loc, err := time.LoadLocation(tz); err == nil {
					t = t.In(loc)
				}
			}
			nextPoll = t.Format("15:04")
		}

		data := map[string]any{
			"Rules":        rules,
			"Folders":      folders,
			"Contacts":     contactsCount,
			"Status":       status,
			"Configured":   passwordSet != "" && imapEmail != "",
			"SourceFolder": sourceFolder,
			"PollInterval": pollInterval,
			"IMAPEmail":    imapEmail,
			"PollingActive": pollingActive,
			"NextPoll":     nextPoll,
			"Processed":    totalProcessed,
		}
		renderPage(w, r, "Dashboard", "dashboard", data)
	}
}

func dashboardStatusHandler(p *poller.Poller, settingsRepo *db.SettingsRepo, imapClient imap.Client, rulesRepo *db.RulesRepo, foldersRepo *db.FoldersRepo, contactsRepo *db.ContactsRepo, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := map[string]any{}
		imapEmail, _ := settingsRepo.Get("imap_email")
		source, _ := settingsRepo.Get("source_folder")
		interval, _ := settingsRepo.Get("poll_interval")
		enabled, _ := settingsRepo.Get("polling_enabled")

		data["IMAPEmail"] = imapEmail
		data["SourceFolder"] = source
		data["PollInterval"] = interval
		data["PollingActive"] = enabled != "false"
		data["PollingHealthy"] = true

		if p != nil {
			s := p.Status()
			data["PollingHealthy"] = s.Healthy
			data["LastError"] = s.LastError
			data["Processing"] = s.ProcessingMessages
			if !s.LastTick.IsZero() {
				data["LastTick"] = s.LastTick.Format("15:04:05")
				data["LastDuration"] = s.LastDuration.Truncate(time.Millisecond).String()
			}
			if data["PollingActive"].(bool) && s.LastTick.Unix() > 0 {
				intSec, _ := strconv.Atoi(interval)
				if intSec > 0 {
					data["NextPoll"] = time.Unix(0, s.LastTick.UnixNano()).Add(time.Duration(intSec) * time.Second).Format("15:04:05")
				}
			}
		}

		totalProcessed, _ := db.NewStatsRepo(rulesRepo.DB).TotalProcessed()
		data["Processed"] = totalProcessed
		rules, _ := rulesRepo.List()
		data["Rules"] = rules
		folders, _ := foldersRepo.List()
		data["Folders"] = folders
		count, _ := contactsRepo.Count()
		data["Contacts"] = count

		renderPartial(w, "dashboard_status", data)
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

func seedContactsHandler(collector *contacts.Collector, foldersRepo *db.FoldersRepo, contactsRepo *db.ContactsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if collector == nil {
			renderPartial(w, "toast", map[string]string{"Type": "error", "Message": "IMAP not connected"})
			return
		}
		before, _ := contactsRepo.Count()
		folders, err := foldersRepo.List()
		if err != nil || len(folders) == 0 {
			renderPartial(w, "toast", map[string]string{"Type": "error", "Message": "No folders synced"})
			return
		}
		for _, f := range folders {
			collector.SeedFromFolder(f.Name)
		}
		after, _ := contactsRepo.Count()
		diff := after - before
		if diff > 0 {
			renderPartial(w, "toast", map[string]string{"Type": "success", "Message": fmt.Sprintf("Collected %d new contacts from %d folders", diff, len(folders))})
		} else {
			renderPartial(w, "toast", map[string]string{"Type": "success", "Message": "Contacts already up to date"})
		}
	}
}

func settingsTogglePolling(settingsRepo *db.SettingsRepo, p *poller.Poller) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enabled, _ := settingsRepo.Get("polling_enabled")
		if enabled == "false" {
			settingsRepo.Set("polling_enabled", "true")
			if p != nil {
				p.Start()
			}
		} else {
			settingsRepo.Set("polling_enabled", "false")
			if p != nil {
				p.Stop()
			}
		}
		w.Header().Set("HX-Refresh", "true")
		renderPartial(w, "toast", map[string]string{"Type": "success", "Message": "Polling " + getPollingLabel(settingsRepo)})
	}
}

func getPollingLabel(settingsRepo *db.SettingsRepo) string {
	v, _ := settingsRepo.Get("polling_enabled")
	if v == "false" {
		return "disabled"
	}
	return "enabled"
}
