package web

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/metrics"
)

func TestMetricsExposeOperationalGauges(t *testing.T) {
	repo := db.NewStatsRepo(openWebTestDB(t))
	var prevUser, prevSys int64
	collect(repo, &prevUser, &prevSys)
	metrics.BuildInfo.WithLabelValues("testver", "testcommit").Set(1)

	rec := httptest.NewRecorder()
	metrics.PromHandler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	body := rec.Body.String()

	for _, name := range []string{
		"mailflow_rules_total",
		"mailflow_contacts_total",
		"mailflow_db_size_bytes",
		"mailflow_build_info",
	} {
		if !strings.Contains(body, name) {
			t.Errorf("metrics output missing %s", name)
		}
	}
}
