package web

import (
	"net/http"

	"github.com/mojoaar/icloud-mailflow/internal/db"
)

func docsStandaloneHandler(settingsRepo *db.SettingsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		monoFont := false
		if v, _ := settingsRepo.Get("font_mono"); v == "true" {
			monoFont = true
		}
		tmpl.ExecuteTemplate(w, "docs", map[string]any{
			"Host":    r.Host,
			"MonoFont": monoFont,
			"Version":  appVersion,
		})
	}
}
