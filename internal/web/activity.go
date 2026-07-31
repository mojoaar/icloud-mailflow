package web

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/mojoaar/icloud-mailflow/internal/db"
)

func activityHandler(repo *db.LogRepo, rulesRepo *db.RulesRepo, settingsRepo *db.SettingsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		search := q.Get("q")
		rule := q.Get("rule")
		status := q.Get("status")
		perPageStr := q.Get("per_page")
		pageStr := q.Get("page")

		perPage, _ := strconv.Atoi(perPageStr)
		if perPage <= 0 {
			perPage = 50
		}
		page, _ := strconv.Atoi(pageStr)
		if page <= 0 {
			page = 1
		}
		offset := (page - 1) * perPage

		entries, total, err := repo.ListFiltered(perPage, offset, search, rule, status)
		if err != nil {
			slog.Error("list filtered activity", "error", err)
		}

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

		totalPages := (total + perPage - 1) / perPage

		rules, _ := rulesRepo.List()
		var ruleNames []string
		for _, rl := range rules {
			if rl.Name != "_catch_all" {
				ruleNames = append(ruleNames, rl.Name)
			}
		}

		data := map[string]any{
			"Entries":      entries,
			"Search":       search,
			"Rule":         rule,
			"Status":       status,
			"PerPage":      perPageStr,
			"Page":         page,
			"TotalEntries": total,
			"TotalPages":   totalPages,
			"RuleNames":    ruleNames,
		}
		if r.Header.Get("HX-Request") == "true" {
			renderPartial(w, "activity_content", data)
			return
		}
		renderPage(w, r, "Activity", "activity", data)
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
