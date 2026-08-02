package db

import "database/sql"

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
