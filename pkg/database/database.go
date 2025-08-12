package database

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func InitializeDB(dbPath string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_busy_timeout=5000", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	statement := `
	CREATE TABLE IF NOT EXISTS projects (
		id                  INTEGER PRIMARY KEY AUTOINCREMENT,
		project_path        TEXT NOT NULL UNIQUE,
		last_scan_timestamp TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS file_metadata (
		project_id          INTEGER NOT NULL,
		relative_path       TEXT NOT NULL,
		filename            TEXT NOT NULL,
		extension           TEXT,
		size_bytes          INTEGER NOT NULL,
		line_count          INTEGER NOT NULL,
		is_text             BOOLEAN NOT NULL,
		last_mod_time       TEXT NOT NULL,
		content_hash        TEXT NOT NULL,
		UNIQUE (project_id, relative_path),
		FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_file_metadata_project_id ON file_metadata(project_id);
	CREATE TABLE IF NOT EXISTS profiles (
		id                  INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id          INTEGER NOT NULL,
		profile_name        TEXT NOT NULL,
		profile_data_json   TEXT NOT NULL,
		UNIQUE (project_id, profile_name),
		FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
	);
	CREATE TABLE IF NOT EXISTS kv_store (
        key TEXT PRIMARY KEY NOT NULL,
        value TEXT
    );
	`
	_, err = db.Exec(statement)
	if err != nil {
		return nil, err
	}

	return db, nil
}
