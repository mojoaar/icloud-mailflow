package web

import (
	"net/http"

	"github.com/mojoaar/icloud-mailflow/internal/db"
)

func statsHandler(repo *db.StatsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		total, _ := repo.TotalProcessed()
		rules, _ := repo.RuleHits()
		senders, _ := repo.TopSenders(10)
		actions, _ := repo.ActionsBreakdown()
		daily, _ := repo.DailyVolume(7)

		maxRuleHit := 0
		for _, h := range rules {
			if h.Count > maxRuleHit {
				maxRuleHit = h.Count
			}
		}

		data := map[string]any{
			"Total":      total,
			"Rules":      rules,
			"Senders":    senders,
			"Actions":    actions,
			"Daily":      daily,
			"MaxRuleHit": maxRuleHit,
		}
		renderPage(w, r, "Stats", "stats", data)
	}
}
