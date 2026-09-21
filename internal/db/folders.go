package db

import (
	"database/sql"
	"strings"
)

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
	rows, err := r.DB.Query(`SELECT id, name, path, flags, synced_at FROM folders ORDER BY name COLLATE NOCASE`)
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

	paths := make([]string, 0, len(folders))
	seen := map[string]bool{}
	for _, f := range folders {
		if f.Path == "" || seen[f.Path] {
			continue
		}
		seen[f.Path] = true
		paths = append(paths, f.Path)
		if _, err := tx.Exec(
			`INSERT INTO folders (name, path, flags, synced_at) VALUES (?, ?, ?, datetime('now'))
			ON CONFLICT(path) DO UPDATE SET name = excluded.name, flags = excluded.flags, synced_at = excluded.synced_at`,
			f.Name, f.Path, f.Flags,
		); err != nil {
			return err
		}
	}

	if len(paths) == 0 {
		if _, err := tx.Exec(`DELETE FROM folders`); err != nil {
			return err
		}
	} else {
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(paths)), ",")
		args := make([]any, len(paths))
		for i, p := range paths {
			args[i] = p
		}
		if _, err := tx.Exec(`DELETE FROM folders WHERE path NOT IN (`+placeholders+`)`, args...); err != nil {
			return err
		}
	}
	return tx.Commit()
}
