package web

import (
	"context"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/mojoaar/icloud-mailflow/internal/db"
)

func StartMetricsCollector(repo *db.StatsRepo, parentCtx context.Context) {
	go func() {
		var prevUser, prevSys int64
		var ru syscall.Rusage
		if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err == nil {
			prevUser = ru.Utime.Nano()
			prevSys = ru.Stime.Nano()
		}

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

func collect(repo *db.StatsRepo, prevUser, prevSys *int64) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	now := time.Now().Unix()
	key := strconv.FormatInt(now, 10)

	repo.SetStat("memory", key, int(m.Alloc/1024/1024))
	repo.SetStat("goroutines", key, runtime.NumGoroutine())

	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err == nil {
		userNano := ru.Utime.Nano()
		sysNano := ru.Stime.Nano()
		if *prevUser > 0 {
			deltaUser := userNano - *prevUser
			deltaSys := sysNano - *prevSys
			cpuPct := int((float64(deltaUser+deltaSys) / 3600e9) * 10000)
			repo.SetStat("cpu", key, cpuPct)
		}
		*prevUser = userNano
		*prevSys = sysNano
	}

	cutoff := now - 86400
	repo.PruneStats("memory", cutoff)
	repo.PruneStats("goroutines", cutoff)
	repo.PruneStats("cpu", cutoff)
}
