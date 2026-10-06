package web

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/mojoaar/icloud-mailflow/internal/db"
)

func auditHandler(repo *db.AuditRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		action := q.Get("action")
		perPage, _ := strconv.Atoi(q.Get("per_page"))
		if perPage <= 0 {
			perPage = 100
		}
		if perPage > 500 {
			perPage = 500
		}
		page, _ := strconv.Atoi(q.Get("page"))
		if page <= 0 {
			page = 1
		}
		offset := (page - 1) * perPage

		entries, total, err := repo.ListFiltered(perPage, offset, action)
		if err != nil {
			slog.Error("list audit failed", "error", err)
		}
		totalPages := (total + perPage - 1) / perPage

		data := map[string]any{
			"Entries":    entries,
			"Action":     action,
			"Page":       page,
			"PerPage":    strconv.Itoa(perPage),
			"TotalPages": totalPages,
		}
		if r.Header.Get("HX-Request") == "true" {
			renderPartial(w, "audit_content", data)
			return
		}
		renderPage(w, r, "Audit", "audit", data)
	}
}

func auditDeleteHandler(repo *db.AuditRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := repo.DeleteAll(); err != nil {
			renderPartial(w, "toast", map[string]string{"Type": "error", "Message": "Failed to clear audit log"})
			return
		}
		w.Header().Set("HX-Refresh", "true")
		renderPartial(w, "toast", map[string]string{"Type": "success", "Message": "Audit log cleared"})
	}
}
