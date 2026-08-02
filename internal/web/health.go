package web

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/imap"
	"github.com/mojoaar/icloud-mailflow/internal/poller"
)

func healthHandler(d *sql.DB, p *poller.Poller, imapClient imap.Client, statsRepo *db.StatsRepo, contactsRepo *db.ContactsRepo, rulesRepo *db.RulesRepo, sessRepo *db.SessionsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := "ok"

		dbStatus := "ok"
		if err := d.Ping(); err != nil {
			dbStatus = "error"
			status = "degraded"
		}

		imapStatus := "not_configured"
		if imapClient != nil {
			imapStatus = "connected"
		}

		var pollerDetail map[string]any
		if p != nil {
			ps := p.Status()
			pollerDetail = map[string]any{
				"active":               ps.Active,
				"healthy":              ps.Healthy,
				"last_tick":            ps.LastTick.Format(time.RFC3339),
				"last_duration_ms":     ps.LastDuration.Milliseconds(),
				"consecutive_failures": ps.ConsecutiveFailures,
			}
			if !ps.Healthy {
				status = "degraded"
			}
		}

		authenticated := false
		if c, err := r.Cookie(sessionCookie); err == nil {
			if valid, _ := sessRepo.Validate(c.Value); valid {
				authenticated = true
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if !authenticated {
			json.NewEncoder(w).Encode(map[string]any{"status": status})
			return
		}

		health := map[string]any{
			"version":        appVersion,
			"uptime_seconds": int(time.Since(startTime).Seconds()),
			"db":             dbStatus,
			"imap":           imapStatus,
			"status":         status,
		}
		if pollerDetail != nil {
			health["poller"] = pollerDetail
		}

		total, _ := statsRepo.TotalProcessed()
		contacts, _ := contactsRepo.Count()
		rules, _ := rulesRepo.List()
		health["stats"] = map[string]any{
			"total_processed": total,
			"contacts_count":  contacts,
			"rules_count":     len(rules),
		}

		json.NewEncoder(w).Encode(health)
	}
}
