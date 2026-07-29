package web

import (
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/mojoaar/icloud-mailflow/internal/carddav"
	"github.com/mojoaar/icloud-mailflow/internal/config"
	"github.com/mojoaar/icloud-mailflow/internal/crypto"
	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/imap"
)

func setupPage(settingsRepo *db.SettingsRepo, d *sql.DB, cfg *config.Config) http.HandlerFunc {
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
				storePassword(settingsRepo, cfg, p)
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
				password := getPassword(settingsRepo, cfg)
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
		timezone, _ := settingsRepo.Get("timezone")
		data := map[string]any{
			"Folders":      folders,
			"IMAPEmail":    imapEmail,
			"SourceFolder": sourceFolder,
			"PollInterval": pollInterval,
			"ListenAddr":   cfg.ListenAddr,
			"Version":      version,
			"Timezone":     timezone,
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
			password = getPassword(settingsRepo, cfg)
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

func encryptPassword(plain, hexKey string) string {
	if hexKey == "" {
		return plain
	}
	key, _ := hex.DecodeString(hexKey)
	enc, err := crypto.Encrypt([]byte(plain), key)
	if err != nil {
		return plain
	}
	return string(enc)
}

func decryptPassword(encrypted, hexKey string) string {
	if hexKey == "" {
		return encrypted
	}
	key, _ := hex.DecodeString(hexKey)
	dec, err := crypto.Decrypt([]byte(encrypted), key)
	if err != nil {
		return encrypted
	}
	return string(dec)
}

func storePassword(settingsRepo *db.SettingsRepo, cfg *config.Config, password string) {
	if password != "" {
		settingsRepo.Set("imap_password", encryptPassword(password, cfg.EncryptionKey))
	}
}

func getPassword(settingsRepo *db.SettingsRepo, cfg *config.Config) string {
	p, _ := settingsRepo.Get("imap_password")
	return decryptPassword(p, cfg.EncryptionKey)
}

func settingsSaveIMAP(cfg *config.Config, settingsRepo *db.SettingsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		cfg.IMAPEmail = r.FormValue("imap_email")
		if p := r.FormValue("imap_password"); p != "" {
			storePassword(settingsRepo, cfg, p)
		}
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

func carddavImportHandler(settingsRepo *db.SettingsRepo, cfg *config.Config, contactsRepo *db.ContactsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email, _ := settingsRepo.Get("imap_email")
		password := getPassword(settingsRepo, cfg)
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

type rulesExport struct {
	Rules []ruleExport `json:"rules"`
}

type ruleExport struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Priority    int              `json:"priority"`
	Enabled     bool             `json:"enabled"`
	Operator    string           `json:"operator"`
	Conditions  []ruleCondExport `json:"conditions"`
	Actions     []ruleActExport  `json:"actions"`
}

type ruleCondExport struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

type ruleActExport struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

func rulesExportHandler(repo *db.RulesRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rules, err := repo.List()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var exp rulesExport
		for _, rule := range rules {
			if rule.Name == "_catch_all" {
				continue
			}
			re := ruleExport{
				Name:        rule.Name,
				Description: rule.Description,
				Priority:    rule.Priority,
				Enabled:     rule.Enabled,
			}
			for _, g := range rule.Groups {
				re.Operator = g.Operator
				for _, c := range g.Conditions {
					re.Conditions = append(re.Conditions, ruleCondExport{
						Field: c.Field, Operator: c.Operator, Value: c.Value,
					})
				}
			}
			for _, a := range rule.Actions {
				re.Actions = append(re.Actions, ruleActExport{Type: a.Type, Value: a.Value})
			}
			exp.Rules = append(exp.Rules, re)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=\"icloud-mailflow-rules.json\"")
		json.NewEncoder(w).Encode(exp)
	}
}

func rulesImportHandler(repo *db.RulesRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(10 << 20)
		file, _, err := r.FormFile("rules_file")
		if err != nil {
			renderPartial(w, "toast", map[string]string{"Type": "error", "Message": "No file selected"})
			return
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			renderPartial(w, "toast", map[string]string{"Type": "error", "Message": "Failed to read file"})
			return
		}

		var exp rulesExport
		if err := json.Unmarshal(data, &exp); err != nil {
			renderPartial(w, "toast", map[string]string{"Type": "error", "Message": "Invalid JSON: " + err.Error()})
			return
		}

		imported := 0
		for _, re := range exp.Rules {
			rule := &db.Rule{
				Name:        re.Name,
				Description: re.Description,
				Priority:    re.Priority,
				Enabled:     re.Enabled,
			}
			if len(re.Conditions) > 0 {
				g := db.ConditionGroup{Operator: re.Operator}
				if g.Operator == "" {
					g.Operator = "AND"
				}
				for _, c := range re.Conditions {
					g.Conditions = append(g.Conditions, db.Condition{
						Field: c.Field, Operator: c.Operator, Value: c.Value,
					})
				}
				rule.Groups = []db.ConditionGroup{g}
			}
			for _, a := range re.Actions {
				rule.Actions = append(rule.Actions, db.Action{Type: a.Type, Value: a.Value})
			}
			if err := repo.Create(rule); err != nil {
				renderPartial(w, "toast", map[string]string{"Type": "error", "Message": "Import failed at '" + re.Name + "': " + err.Error()})
				return
			}
			imported++
		}

		if imported == 0 {
			renderPartial(w, "toast", map[string]string{"Type": "success", "Message": "No rules to import"})
			return
		}
		renderPartial(w, "toast", map[string]string{"Type": "success", "Message": fmt.Sprintf("Imported %d rules", imported)})
	}
}

func settingsSaveTimezone(settingsRepo *db.SettingsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		settingsRepo.Set("timezone", r.FormValue("timezone"))
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
	}
}
