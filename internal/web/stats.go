package web

import (
	"net/http"
	"strconv"
	"time"

	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/poller"
)

var statsDayOptions = []int{7, 30, 90}

func statsHandler(repo *db.StatsRepo, settingsRepo *db.SettingsRepo, p *poller.Poller) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		days := statsRange(r.URL.Query().Get("days"))

		loc := time.UTC
		if tz, _ := settingsRepo.Get("timezone"); tz != "" {
			if l, err := time.LoadLocation(tz); err == nil {
				loc = l
			}
		}

		total, _ := repo.TotalProcessed()
		rules, _ := repo.RuleHits()
		senders, _ := repo.TopSenders(20)
		actions, _ := repo.ActionsBreakdown()
		daily, _ := repo.DailyVolume(days)
		errors, _ := repo.ErrorBreakdown()
		folders, _ := repo.FolderDistribution()
		weeks := days / 7
		if weeks < 8 {
			weeks = 8
		}
		weekly, _ := repo.WeeklyVolume(weeks)

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

		metricsMem, _ := repo.MetricValues("memory", 1440, loc)
		metricsCPU, _ := repo.MetricValues("cpu", 1440, loc)

		pollerInfo := map[string]any{"Configured": false}
		if p != nil {
			s := p.Status()
			last := "never"
			if !s.LastTick.IsZero() {
				last = s.LastTick.Format("15:04:05")
			}
			pollerInfo = map[string]any{
				"Configured":          true,
				"Active":              s.Active,
				"Healthy":             s.Healthy,
				"LastRun":             last,
				"LastDuration":        s.LastDuration.String(),
				"ConsecutiveFailures": s.ConsecutiveFailures,
				"Processing":          s.ProcessingMessages,
				"LastError":           s.LastError,
			}
		}

		data := map[string]any{
			"Total":          total,
			"Rules":          rules,
			"Senders":        senders,
			"Actions":        actions,
			"Daily":          daily,
			"MaxRuleHit":     maxRuleHit,
			"Errors":         errors,
			"Folders":        folders,
			"MaxFolderCount": maxFolderCount,
			"Weekly":         weekly,
			"MetricsMem":     metricsMem,
			"MetricsCPU":     metricsCPU,
			"Days":           days,
			"DaysOptions":    statsDayOptions,
			"Poller":         pollerInfo,
		}
		renderPage(w, r, "Stats", "stats", data)
	}
}

func statsRange(v string) int {
	for _, d := range statsDayOptions {
		if v == strconv.Itoa(d) {
			return d
		}
	}
	return statsDayOptions[0]
}
