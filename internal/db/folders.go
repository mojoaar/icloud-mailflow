package db

import "database/sql"

type Folder struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Flags    string `json:"flags"`
	SyncedAt string `json:"synced_at"`
}

type FoldersRepo struct{ DB *sql.DB }

func NewFoldersRepo(d *sql.DB) *FoldersRepo {
	return &FoldersRepo{DB: d}
}

func (r *FoldersRepo) List() ([]Folder, error) {
	rows, err := r.DB.Query(`SELECT id, name, path, flags, synced_at FROM folders ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Folder
	for rows.Next() {
		var f Folder
		if err := rows.Scan(&f.ID, &f.Name, &f.Path, &f.Flags, &f.SyncedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (r *FoldersRepo) Sync(folders []Folder) error {
	tx, err := r.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM folders`); err != nil {
		return err
	}
	for _, f := range folders {
		_, err := tx.Exec(
			`INSERT INTO folders (name, path, flags, synced_at) VALUES (?, ?, ?, datetime('now'))`,
			f.Name, f.Path, f.Flags,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
