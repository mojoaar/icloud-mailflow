package web

import (
	"net/http"
	"time"

	"github.com/mojoaar/icloud-mailflow/internal/db"
)

func activityHandler(repo *db.LogRepo, settingsRepo *db.SettingsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, _ := repo.ListRecent(50)
		tz, _ := settingsRepo.Get("timezone")
		if tz != "" && tz != "UTC" {
			loc, err := time.LoadLocation(tz)
			if err == nil {
				for i := range entries {
					t, _ := time.Parse("2006-01-02 15:04:05", entries[i].CreatedAt)
					entries[i].CreatedAt = t.In(loc).Format("2006-01-02 15:04:05")
				}
			}
		}
		renderPage(w, r, "Activity", "activity", map[string]any{"Entries": entries})
	}
}

func activityDeleteHandler(repo *db.LogRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := repo.DeleteAll(); err != nil {
			renderPartial(w, "toast", map[string]string{"Type": "error", "Message": err.Error()})
			return
		}
		w.Header().Set("HX-Refresh", "true")
		renderPartial(w, "toast", map[string]string{"Type": "success", "Message": "Activity log cleared"})
	}
}
