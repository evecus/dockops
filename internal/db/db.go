package db

import (
	"database/sql"
	"path/filepath"
	"strings"

	_ "github.com/glebarez/sqlite"
)

type DB struct {
	*sql.DB
}

func Init(dataPath string) (*DB, error) {
	dbPath := filepath.Join(dataPath, "dockops.db")
	sqlDB, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	database := &DB{sqlDB}
	if err := database.migrate(); err != nil {
		return nil, err
	}
	return database, nil
}

func (d *DB) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS admin (
			id INTEGER PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
	}
	for _, q := range queries {
		if _, err := d.Exec(q); err != nil {
			if !strings.Contains(err.Error(), "duplicate column") {
				return err
			}
		}
	}
	defaults := map[string]string{
		"update_check_interval": "6h",
		"docker_proxy":          "",
		"collect_interval":      "10m",
	}
	for k, v := range defaults {
		d.Exec(`INSERT OR IGNORE INTO settings (key, value) VALUES (?, ?)`, k, v)
	}
	return nil
}

func (d *DB) IsSetup() (bool, error) {
	var count int
	err := d.QueryRow(`SELECT COUNT(*) FROM admin`).Scan(&count)
	return count > 0, err
}

func (d *DB) GetSetting(key string) (string, error) {
	var value string
	err := d.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	return value, err
}

func (d *DB) SetSetting(key, value string) error {
	_, err := d.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)`, key, value)
	return err
}

func (d *DB) GetAllSettings() (map[string]string, error) {
	rows, err := d.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		result[k] = v
	}
	return result, nil
}
