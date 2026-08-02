package web

import (
	"net/http"

	"github.com/mojoaar/icloud-mailflow/internal/db"
)

func statsHandler(repo *db.StatsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		total, _ := repo.TotalProcessed()
		rules, _ := repo.RuleHits()
		senders, _ := repo.TopSenders(20)
		actions, _ := repo.ActionsBreakdown()
		daily, _ := repo.DailyVolume(7)
		errors, _ := repo.ErrorBreakdown()
		folders, _ := repo.FolderDistribution()
		weekly, _ := repo.WeeklyVolume(24)

		maxRuleHit := 0
		for _, h := range rules {
			if h.Count > maxRuleHit {
				maxRuleHit = h.Count
			}
		}
		maxFolderCount := 0
		for _, f := range folders {
			if f.Count > maxFolderCount {
				maxFolderCount = f.Count
			}
		}

	metricsMem, _ := repo.MetricValues("memory", 1440)
	metricsGoroutines, _ := repo.MetricValues("goroutines", 1440)
	metricsCPU, _ := repo.MetricValues("cpu", 1440)

	data := map[string]any{
		"Total":             total,
		"Rules":             rules,
		"Senders":           senders,
		"Actions":           actions,
		"Daily":             daily,
		"MaxRuleHit":        maxRuleHit,
		"Errors":            errors,
		"Folders":           folders,
		"MaxFolderCount":    maxFolderCount,
		"Weekly":            weekly,
		"MetricsMem":        metricsMem,
		"MetricsGoroutines": metricsGoroutines,
		"MetricsCPU":        metricsCPU,
	}
		renderPage(w, r, "Stats", "stats", data)
	}
}
