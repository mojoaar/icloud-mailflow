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

func healthHandler(d *sql.DB, p *poller.Poller, imapClient imap.Client, statsRepo *db.StatsRepo, contactsRepo *db.ContactsRepo, rulesRepo *db.RulesRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := "ok"
		health := map[string]any{
			"version":        appVersion,
			"uptime_seconds": int(time.Since(startTime).Seconds()),
		}

		if err := d.Ping(); err != nil {
			health["db"] = "error"
			status = "degraded"
		} else {
			health["db"] = "ok"
		}

		if imapClient != nil {
			health["imap"] = "connected"
		} else {
			health["imap"] = "not_configured"
		}

		if p != nil {
			ps := p.Status()
			health["poller"] = map[string]any{
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

		total, _ := statsRepo.TotalProcessed()
		contacts, _ := contactsRepo.Count()
		rules, _ := rulesRepo.List()
		health["stats"] = map[string]any{
			"total_processed": total,
			"contacts_count":  contacts,
			"rules_count":     len(rules),
		}

		health["status"] = status

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(health)
	}
}
