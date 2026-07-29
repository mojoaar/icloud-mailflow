package web

import (
	"net/http"

	"github.com/mojoaar/icloud-mailflow/internal/db"
)

func activityHandler(repo *db.LogRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, _ := repo.ListRecent(50)
		renderPage(w, r, "Activity", "activity", map[string]any{"Entries": entries})
	}
}
