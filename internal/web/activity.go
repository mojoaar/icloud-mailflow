package web

import (
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/poller"
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
			perPage = 100
		}
		if perPage > 500 {
			perPage = 500
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
					entries[i].CreatedAt = t.In(loc).Format("2006-01-02T15:04:05")
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
		sort.Slice(ruleNames, func(i, j int) bool { return strings.ToLower(ruleNames[i]) < strings.ToLower(ruleNames[j]) })

		data := map[string]any{
			"Entries":      entries,
			"Search":       search,
			"Rule":         rule,
			"Status":       status,
			"PerPage":      strconv.Itoa(perPage),
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

func activityRerunHandler(p *poller.Poller) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, _ := strconv.Atoi(r.FormValue("uid"))
		folder := r.FormValue("folder")
		if p == nil {
			renderPartial(w, "toast", map[string]string{"Type": "error", "Message": "IMAP not configured"})
			return
		}
		msg, matched, captures, results, err := p.EvaluateMessage(folder, uint32(uid))
		if err != nil {
			renderPartial(w, "toast", map[string]string{"Type": "error", "Message": "Message no longer available"})
			return
		}
		renderPartial(w, "rules_test_result", map[string]any{
			"Matched":  matched != nil,
			"Captures": captures,
			"Results":  results,
			"Rule":     matched,
			"Message":  msg,
		})
	}
}

func activityDeleteHandler(repo *db.LogRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := repo.DeleteAll(); err != nil {
			renderPartial(w, "toast", map[string]string{"Type": "error", "Message": "Failed to clear activity log"})
			return
		}
		w.Header().Set("HX-Refresh", "true")
		renderPartial(w, "toast", map[string]string{"Type": "success", "Message": "Activity log cleared"})
	}
}

func activityDeleteSelectedHandler(repo *db.LogRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			renderPartial(w, "toast", map[string]string{"Type": "error", "Message": "Invalid request"})
			return
		}
		var ids []int64
		for _, v := range r.Form["id"] {
			if id, err := strconv.ParseInt(v, 10, 64); err == nil {
				ids = append(ids, id)
			}
		}
		if len(ids) == 0 {
			renderPartial(w, "toast", map[string]string{"Type": "error", "Message": "No entries selected"})
			return
		}
		n, err := repo.DeleteByIDs(ids)
		if err != nil {
			renderPartial(w, "toast", map[string]string{"Type": "error", "Message": "Failed to delete activity entries"})
			return
		}
		w.Header().Set("HX-Refresh", "true")
		renderPartial(w, "toast", map[string]string{"Type": "success", "Message": fmt.Sprintf("Deleted %d entries", n)})
	}
}
