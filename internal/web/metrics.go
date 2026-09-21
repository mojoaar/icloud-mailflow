package web

import (
	"context"
	"database/sql"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/metrics"
)

func StartMetricsCollector(repo *db.StatsRepo, parentCtx context.Context) {
	go func() {
		defer func() { _ = recover() }()
		var prevUser, prevSys int64
		var ru syscall.Rusage
		if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err == nil {
			prevUser = ru.Utime.Nano()
			prevSys = ru.Stime.Nano()
		}

		metrics.BuildInfo.WithLabelValues(appVersion, buildCommit).Set(1)
		collect(repo, &prevUser, &prevSys)

		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-parentCtx.Done():
				return
			case <-ticker.C:
				collect(repo, &prevUser, &prevSys)
			}
		}
	}()
}

// dbSizeBytes returns the SQLite database size in bytes.
func dbSizeBytes(d *sql.DB) int64 {
	var pages, pageSize int64
	if err := d.QueryRow(`PRAGMA page_count`).Scan(&pages); err != nil {
		return 0
	}
	if err := d.QueryRow(`PRAGMA page_size`).Scan(&pageSize); err != nil {
		return 0
	}
	return pages * pageSize
}

func collect(repo *db.StatsRepo, prevUser, prevSys *int64) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	now := time.Now().Unix()
	key := strconv.FormatInt(now, 10)

	metrics.MemoryBytes.Set(float64(m.Alloc))
	metrics.UptimeSeconds.Set(time.Since(startTime).Seconds())
	repo.SetStat("memory", key, int(m.Alloc/1024/1024))
	repo.SetStat("goroutines", key, runtime.NumGoroutine())

	if rules, err := db.NewRulesRepo(repo.DB).List(); err == nil {
		metrics.RulesTotal.Set(float64(len(rules)))
	}
	if contacts, err := db.NewContactsRepo(repo.DB).Count(); err == nil {
		metrics.ContactsTotal.Set(float64(contacts))
	}
	metrics.DBSizeBytes.Set(float64(dbSizeBytes(repo.DB)))

	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err == nil {
		userNano := ru.Utime.Nano()
		sysNano := ru.Stime.Nano()
		if *prevUser > 0 {
			deltaUser := userNano - *prevUser
			deltaSys := sysNano - *prevSys
			cpuPct := int((float64(deltaUser+deltaSys) / 3600e9 / float64(runtime.NumCPU())) * 1_000_000)
			repo.SetStat("cpu", key, cpuPct)
			metrics.CPUPercent.Set(float64(cpuPct) / 10000.0)
		}
		*prevUser = userNano
		*prevSys = sysNano
	}

	cutoff := now - 86400
	repo.PruneStats("memory", cutoff)
	repo.PruneStats("goroutines", cutoff)
	repo.PruneStats("cpu", cutoff)

	// Retain the auto-reply throttle log for 7 days.
	db.NewAutoReplyRepo(repo.DB).Prune(time.Now().AddDate(0, 0, -7))
}
