package db

import (
	"strconv"
	"testing"
	"time"
)

func insertLogEntry(t *testing.T, repo *StatsRepo, uid int, fromAddr, ruleName, actionType string) {
	t.Helper()
	_, err := repo.DB.Exec(`INSERT INTO message_log (uid, subject, from_addr, rule_name, action_type) VALUES (?, 'test', ?, ?, ?)`,
		uid, fromAddr, ruleName, actionType)
	if err != nil {
		t.Fatalf("insert log entry: %v", err)
	}
	repo.IncrementStat("total", "processed")
	if ruleName != "" {
		repo.IncrementStat("rule_hit", ruleName)
	}
	if fromAddr != "" {
		repo.IncrementStat("sender", fromAddr)
	}
	if actionType != "" {
		repo.IncrementStat("action", actionType)
	}
}

func insertLogEntryAt(t *testing.T, repo *StatsRepo, createdAt string) {
	t.Helper()
	_, err := repo.DB.Exec(`INSERT INTO message_log (uid, subject, created_at) VALUES (1, 'test', ?)`, createdAt)
	if err != nil {
		t.Fatalf("insert log entry: %v", err)
	}
	repo.IncrementStat("total", "processed")
	repo.IncrementStat("daily", createdAt[:10])
	repo.IncrementStat("weekly", "2026-W27")
}

func TestStatsTotalProcessed(t *testing.T) {
	d := NewTestDB(t)
	repo := NewStatsRepo(d)

	insertLogEntry(t, repo, 1, "a@b.com", "rule1", "move")
	insertLogEntry(t, repo, 2, "c@d.com", "rule2", "mark_as_read")

	n, err := repo.TotalProcessed()
	if err != nil {
		t.Fatalf("TotalProcessed: %v", err)
	}
	if n != 2 {
		t.Errorf("TotalProcessed = %d, want 2", n)
	}
}

func TestStatsTotalProcessedEmpty(t *testing.T) {
	d := NewTestDB(t)
	repo := NewStatsRepo(d)

	n, _ := repo.TotalProcessed()
	if n != 0 {
		t.Errorf("TotalProcessed = %d, want 0", n)
	}
}

func TestStatsRuleHits(t *testing.T) {
	d := NewTestDB(t)
	repo := NewStatsRepo(d)

	insertLogEntry(t, repo, 1, "a@b.com", "rule1", "move")
	insertLogEntry(t, repo, 2, "a@b.com", "rule1", "mark_as_read")
	insertLogEntry(t, repo, 3, "c@d.com", "rule2", "move")

	hits, err := repo.RuleHits()
	if err != nil {
		t.Fatalf("RuleHits: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("len = %d, want 2", len(hits))
	}
	if hits[0].Name != "rule1" || hits[0].Count != 2 {
		t.Errorf("top hit: name=%s count=%d, want rule1 2", hits[0].Name, hits[0].Count)
	}
	if hits[1].Name != "rule2" || hits[1].Count != 1 {
		t.Errorf("second hit: name=%s count=%d, want rule2 1", hits[1].Name, hits[1].Count)
	}
}

func TestStatsTopSenders(t *testing.T) {
	d := NewTestDB(t)
	repo := NewStatsRepo(d)

	insertLogEntry(t, repo, 1, "bob@example.com", "rule1", "move")
	insertLogEntry(t, repo, 2, "bob@example.com", "rule1", "move")
	insertLogEntry(t, repo, 3, "alice@example.com", "rule2", "move")

	senders, err := repo.TopSenders(10)
	if err != nil {
		t.Fatalf("TopSenders: %v", err)
	}
	if len(senders) != 2 {
		t.Fatalf("len = %d, want 2", len(senders))
	}
	if senders[0].Email != "bob@example.com" || senders[0].Count != 2 {
		t.Errorf("top sender: email=%s count=%d, want bob@example.com 2", senders[0].Email, senders[0].Count)
	}
}

func TestStatsTopSendersLimit(t *testing.T) {
	d := NewTestDB(t)
	repo := NewStatsRepo(d)

	insertLogEntry(t, repo, 1, "a@b.com", "r", "x")
	insertLogEntry(t, repo, 2, "c@d.com", "r", "x")

	senders, _ := repo.TopSenders(1)
	if len(senders) != 1 {
		t.Errorf("len = %d, want 1 (limit)", len(senders))
	}
}

func TestStatsActionsBreakdown(t *testing.T) {
	d := NewTestDB(t)
	repo := NewStatsRepo(d)

	insertLogEntry(t, repo, 1, "a@b.com", "rule1", "move")
	insertLogEntry(t, repo, 2, "a@b.com", "rule1", "move")
	insertLogEntry(t, repo, 3, "a@b.com", "rule1", "mark_as_read")

	breakdown, err := repo.ActionsBreakdown()
	if err != nil {
		t.Fatalf("ActionsBreakdown: %v", err)
	}
	if len(breakdown) != 2 {
		t.Fatalf("len = %d, want 2", len(breakdown))
	}
	if breakdown[0].Type != "move" || breakdown[0].Count != 2 {
		t.Errorf("top action: type=%s count=%d, want move 2", breakdown[0].Type, breakdown[0].Count)
	}
}

func TestStatsDailyVolume(t *testing.T) {
	d := NewTestDB(t)
	repo := NewStatsRepo(d)

	insertLogEntryAt(t, repo, "2026-07-01 10:00:00")
	insertLogEntryAt(t, repo, "2026-07-01 11:00:00")
	insertLogEntryAt(t, repo, "2026-07-02 09:00:00")

	vol, err := repo.DailyVolume(7)
	if err != nil {
		t.Fatalf("DailyVolume: %v", err)
	}
	if len(vol) != 2 {
		t.Fatalf("len = %d, want 2", len(vol))
	}
	if vol[0].Date != "2026-07-01" || vol[0].Count != 2 {
		t.Errorf("day 1: date=%s count=%d, want 2026-07-01 2", vol[0].Date, vol[0].Count)
	}
	if vol[1].Date != "2026-07-02" || vol[1].Count != 1 {
		t.Errorf("day 2: date=%s count=%d, want 2026-07-02 1", vol[1].Date, vol[1].Count)
	}
}

func TestStatsWeeklyVolumeAscending(t *testing.T) {
	d := NewTestDB(t)
	repo := NewStatsRepo(d)

	repo.IncrementStat("weekly", "2026-W30")
	repo.IncrementStat("weekly", "2026-W28")
	repo.IncrementStat("weekly", "2026-W29")

	vol, err := repo.WeeklyVolume(2)
	if err != nil {
		t.Fatalf("WeeklyVolume: %v", err)
	}
	if len(vol) != 2 {
		t.Fatalf("len = %d, want 2 (most recent)", len(vol))
	}
	if vol[0].Date != "2026-W29" || vol[1].Date != "2026-W30" {
		t.Errorf("want ascending W29,W30; got %s,%s", vol[0].Date, vol[1].Date)
	}
}

func TestStatsMetricValuesOrderAndTimezone(t *testing.T) {
	d := NewTestDB(t)
	repo := NewStatsRepo(d)

	t1 := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC).Unix()
	t2 := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC).Unix()
	repo.IncrementStat("cpu", strconv.FormatInt(t1, 10))
	repo.IncrementStat("cpu", strconv.FormatInt(t2, 10))

	loc, err := time.LoadLocation("Europe/Copenhagen")
	if err != nil {
		t.Fatal(err)
	}

	vals, err := repo.MetricValues("cpu", 1440, loc)
	if err != nil {
		t.Fatalf("MetricValues: %v", err)
	}
	if len(vals) != 2 {
		t.Fatalf("len = %d, want 2", len(vals))
	}
	if vals[0].Date != "10:00" || vals[1].Date != "12:00" {
		t.Errorf("want ascending 10:00,12:00 (CEST); got %s,%s", vals[0].Date, vals[1].Date)
	}
}
