package db

import (
	"database/sql"
	"time"
)

type AutoReplyRepo struct{ DB *sql.DB }

func NewAutoReplyRepo(d *sql.DB) *AutoReplyRepo { return &AutoReplyRepo{DB: d} }

func (r *AutoReplyRepo) ShouldReply(recipient string) (bool, error) {
	res, err := r.DB.Exec(
		`INSERT OR IGNORE INTO auto_reply_log (recipient, reply_date) VALUES (?, date('now'))`,
		recipient,
	)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

// HasReplied reports whether the recipient already got a reply today, without
// consuming the throttle slot.
func (r *AutoReplyRepo) HasReplied(recipient string) (bool, error) {
	var n int
	err := r.DB.QueryRow(
		`SELECT COUNT(*) FROM auto_reply_log WHERE recipient=? AND reply_date=date('now')`,
		recipient,
	).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// RecordReply consumes the daily throttle slot for the recipient.
func (r *AutoReplyRepo) RecordReply(recipient string) error {
	_, err := r.DB.Exec(
		`INSERT OR IGNORE INTO auto_reply_log (recipient, reply_date) VALUES (?, date('now'))`,
		recipient,
	)
	return err
}

// Prune deletes throttle rows older than the given date.
func (r *AutoReplyRepo) Prune(olderThan time.Time) error {
	_, err := r.DB.Exec(`DELETE FROM auto_reply_log WHERE reply_date < ?`, olderThan.Format("2006-01-02"))
	return err
}
