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

type FolderCount struct {
	Folder string `json:"folder"`
	Count  int    `json:"count"`
}

type StatusBreakdown struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

func (r *StatsRepo) IncrementStat(category, key string) error {
	_, err := r.DB.Exec(`INSERT INTO stats (category, key, value) VALUES (?, ?, 1)
		ON CONFLICT(category, key) DO UPDATE SET value = value + 1`, category, key)
	return err
}

func (r *StatsRepo) TotalProcessed() (int, error) {
	var n int
	err := r.DB.QueryRow(`SELECT COALESCE((SELECT value FROM stats WHERE category='total' AND key='processed'), 0)`).Scan(&n)
	return n, err
}

func (r *StatsRepo) RuleHits() ([]RuleHit, error) {
	rows, err := r.DB.Query(`SELECT key, value FROM stats WHERE category='rule_hit' ORDER BY value DESC`)
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
	rows, err := r.DB.Query(`SELECT key, value FROM stats WHERE category='sender' ORDER BY value DESC LIMIT ?`, limit)
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
	rows, err := r.DB.Query(`SELECT key, value FROM stats WHERE category='action' ORDER BY value DESC`)
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
	rows, err := r.DB.Query(`SELECT key, value FROM stats WHERE category='daily' ORDER BY key DESC LIMIT ?`, days)
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

func (r *StatsRepo) UnmatchedCount() (int, error) {
	var n int
	err := r.DB.QueryRow(`SELECT COALESCE((SELECT value FROM stats WHERE category='unmatched' AND key='total'), 0)`).Scan(&n)
	return n, err
}

func (r *StatsRepo) ErrorBreakdown() ([]StatusBreakdown, error) {
	rows, err := r.DB.Query(`SELECT key, value FROM stats WHERE category='status'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StatusBreakdown
	for rows.Next() {
		var s StatusBreakdown
		if err := rows.Scan(&s.Status, &s.Count); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *StatsRepo) FolderDistribution() ([]FolderCount, error) {
	rows, err := r.DB.Query(`SELECT key, value FROM stats WHERE category='folder' ORDER BY value DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FolderCount
	for rows.Next() {
		var f FolderCount
		if err := rows.Scan(&f.Folder, &f.Count); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (r *StatsRepo) WeeklyVolume(weeks int) ([]DailyVolume, error) {
	rows, err := r.DB.Query(`SELECT key, value FROM stats WHERE category='weekly' ORDER BY key DESC LIMIT ?`, weeks)
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
