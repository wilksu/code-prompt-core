package database

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

func InitializeDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	statement := `
	CREATE TABLE IF NOT EXISTS projects (
		id                INTEGER PRIMARY KEY AUTOINCREMENT,
		project_path      TEXT NOT NULL UNIQUE,
		last_scan_timestamp TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS file_metadata (
		project_id    INTEGER NOT NULL,
		relative_path TEXT NOT NULL,
		filename      TEXT NOT NULL,
		extension     TEXT,
		size_bytes    INTEGER NOT NULL,
		line_count    INTEGER NOT NULL,
		is_text       BOOLEAN NOT NULL,
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
	`
	_, err = db.Exec(statement)
	if err != nil {
		return nil, err
	}

	return db, nil
}
