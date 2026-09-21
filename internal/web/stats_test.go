package web

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mojoaar/icloud-mailflow/internal/db"
)

func TestStatsExportHandler(t *testing.T) {
	database := openWebTestDB(t)
	statsRepo := db.NewStatsRepo(database)
	statsRepo.IncrementStat("rule_hit", "test-rule")
	statsRepo.IncrementStat("total", "processed")
	statsRepo.IncrementStat("sender", "a@b.com")
	statsRepo.SetStat("daily", "2026-01-01", 5)

	req := httptest.NewRequest("GET", "/stats/export.csv?days=30", nil)
	rec := httptest.NewRecorder()
	statsExportHandler(statsRepo).ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("content-type = %q, want text/csv", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") || !strings.Contains(cd, ".csv") {
		t.Errorf("content-disposition = %q, want attachment filename", cd)
	}

	body := rec.Body.String()
	for _, want := range []string{
		"category,name,count",
		"total,processed,1",
		"rule_hit,test-rule,1",
		"sender,a@b.com,1",
		"daily,2026-01-01,5",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("csv missing %q\n%s", want, body)
		}
	}
}
