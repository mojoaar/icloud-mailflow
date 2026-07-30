package db

import "database/sql"

type StatsRepo struct{ DB *sql.DB }

func NewStatsRepo(d *sql.DB) *StatsRepo { return &StatsRepo{DB: d} }

type RuleHit struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type TopSender struct {
	Email string `json:"email"`
	Count int    `json:"count"`
}

type ActionBreakdown struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

type DailyVolume struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

func (r *StatsRepo) TotalProcessed() (int, error) {
	var n int
	err := r.DB.QueryRow(`SELECT COUNT(*) FROM message_log`).Scan(&n)
	return n, err
}

func (r *StatsRepo) RuleHits() ([]RuleHit, error) {
	rows, err := r.DB.Query(`SELECT rule_name, COUNT(*) as cnt FROM message_log WHERE rule_name != '' GROUP BY rule_name ORDER BY cnt DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RuleHit
	for rows.Next() {
		var h RuleHit
		if err := rows.Scan(&h.Name, &h.Count); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (r *StatsRepo) TopSenders(limit int) ([]TopSender, error) {
	rows, err := r.DB.Query(`SELECT from_addr, COUNT(*) as cnt FROM message_log WHERE from_addr != '' GROUP BY from_addr ORDER BY cnt DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TopSender
	for rows.Next() {
		var s TopSender
		if err := rows.Scan(&s.Email, &s.Count); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *StatsRepo) ActionsBreakdown() ([]ActionBreakdown, error) {
	rows, err := r.DB.Query(`SELECT action_type, COUNT(*) as cnt FROM message_log WHERE action_type != '' GROUP BY action_type ORDER BY cnt DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ActionBreakdown
	for rows.Next() {
		var a ActionBreakdown
		if err := rows.Scan(&a.Type, &a.Count); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *StatsRepo) DailyVolume(days int) ([]DailyVolume, error) {
	rows, err := r.DB.Query(`SELECT date(created_at) as dt, COUNT(*) as cnt FROM message_log WHERE dt != '' GROUP BY dt ORDER BY dt DESC LIMIT ?`, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DailyVolume
	for rows.Next() {
		var d DailyVolume
		if err := rows.Scan(&d.Date, &d.Count); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
