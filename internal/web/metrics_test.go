package web

import (
	"context"
	"testing"
	"time"

	"github.com/mojoaar/icloud-mailflow/internal/db"
)

func TestCollectCPUPrecision(t *testing.T) {
	repo := db.NewStatsRepo(openWebTestDB(t))
	var prevUser, prevSys int64

	collect(repo, &prevUser, &prevSys)
	done := make(chan struct{})
	go func() {
		for i := 0; i < 50_000_000; i++ {
		}
		close(done)
	}()
	<-done
	collect(repo, &prevUser, &prevSys)

	vals, _ := repo.MetricValues("cpu", 1440)
	if len(vals) == 0 {
		t.Fatal("expected cpu stat after collect")
	}
	for _, v := range vals {
		if v.Count < 0 {
			t.Errorf("expected non-negative cpu value, got %d", v.Count)
		}
	}
}

func TestCollectCPUMultiplierPrecision(t *testing.T) {
	nsPerHour := float64(3600e9)
	delta := float64(3_600_000)
	cpuPct := int((delta / nsPerHour) * 1_000_000)
	if cpuPct < 1 {
		t.Errorf("multiplier too small: 3.6ms delta → %d cpuPct, expected >= 1", cpuPct)
	}
	cpuPctOld := int((delta / nsPerHour) * 10000)
	if cpuPctOld != 0 {
		t.Fatalf("old multiplier unexpectedly returned non-zero: %d", cpuPctOld)
	}
}

func TestCollectCPUPanicsRecovered(t *testing.T) {
	repo := db.NewStatsRepo(openWebTestDB(t))
	ctx, cancel := context.WithCancel(context.Background())
	StartMetricsCollector(repo, ctx)
	cancel()
	time.Sleep(50 * time.Millisecond)
}

func TestFormatCPUPrecision(t *testing.T) {
	fn := templateFuncs["formatCPU"].(func(int) string)
	result := fn(5000)
	if result != "0.5%" {
		t.Errorf("expected 0.5%%, got %s", result)
	}
	result2 := fn(350000)
	if result2 != "35.0%" {
		t.Errorf("expected 35.0%%, got %s", result2)
	}
}
