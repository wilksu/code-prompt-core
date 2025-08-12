package cmd

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"code-prompt-core/pkg/database"
	"code-prompt-core/pkg/scanner"

	gitignore "github.com/sabhiram/go-gitignore"
	"github.com/spf13/cobra"
)

// --- Progress Reporting Helper ---
// A helper function to print structured progress to stderr for the GUI to consume.
func printProgress(status string, data interface{}) {
	progress := map[string]interface{}{"type": "progress", "status": status, "data": data}
	bytes, _ := json.Marshal(progress)
	fmt.Fprintln(os.Stderr, string(bytes))
}

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage the project file cache",
	Long:  `The "cache" command group contains subcommands for creating, updating, and checking the status of a project's file metadata cache.`,
}

// --- `cache update` Command ---
var cacheUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Create or update the cache for a project (full or incremental)",
	Long: `Performs a fresh (full) or incremental scan of the specified project and updates the cache.

By default, this command automatically ignores common dependency directories (like node_modules, venv, target), 
only scans text files, and respects .gitignore rules. Use flags to modify this behavior.

Parameters:
  --db <path> (string, required)
    Path to the database file.

  --project-path <path> (string, required)
    Path to the project's root directory.

  --incremental (bool, optional)
    If set, performs an efficient incremental scan. This is much faster for subsequent scans.

  --no-preset-excludes (bool, optional)
    Disable the default exclusion of common dependency directories. Use this if you need to scan inside
    folders like 'node_modules', 'venv', 'target', etc.

  --include-binary (bool, optional)
    If set, the scan will include binary files. The default behavior is to ignore them.

  --no-git-ignores (bool, optional)
    Disable automatic parsing of .gitignore files.

Example Usage:
  # Perform an initial scan using all smart defaults
  ./code-prompt-core.exe cache update --db my.db --project-path /path/to/project

  # Perform a faster, incremental scan
  ./code-prompt-core.exe cache update --db my.db --project-path /path/to/project --incremental

  # Scan everything, including binaries and node_modules
  ./code-prompt-core.exe cache update --db my.db --project-path /path/to/project --include-binary --no-preset-excludes
`,
	Run: func(cmd *cobra.Command, args []string) {
		dbPath, _ := cmd.Flags().GetString("db")
		projectPath, _ := cmd.Flags().GetString("project-path")
		noGitIgnores, _ := cmd.Flags().GetBool("no-git-ignores")
		isIncremental, _ := cmd.Flags().GetBool("incremental")
		includeBinary, _ := cmd.Flags().GetBool("include-binary")
		noPresetExcludes, _ := cmd.Flags().GetBool("no-preset-excludes")

		db, err := database.InitializeDB(dbPath)
		if err != nil {
			printError(fmt.Errorf("error initializing database: %w", err))
			return
		}
		defer db.Close()

		projectID, err := getOrCreateProject(db, projectPath)
		if err != nil {
			printError(fmt.Errorf("error getting or creating project: %w", err))
			return
		}

		scanOpts := scanner.ScanOptions{
			NoGitIgnores:     noGitIgnores,
			IncludeBinary:    includeBinary,
			NoPresetExcludes: noPresetExcludes,
		}

		if !isIncremental {
			runFullScan(db, projectID, projectPath, scanOpts)
		} else {
			runIncrementalScan(db, projectID, projectPath, scanOpts)
		}
	},
}

// --- Full Scan Logic ---
func runFullScan(db *sql.DB, projectID int64, projectPath string, scanOpts scanner.ScanOptions) {
	printProgress("starting_full_scan", nil)

	printProgress("clearing_old_cache", nil)
	_, err := db.Exec("DELETE FROM file_metadata WHERE project_id = ?", projectID)
	if err != nil {
		printError(fmt.Errorf("error clearing old cache: %w", err))
		return
	}

	printProgress("scanning_local_files", scanOpts)
	files, err := scanner.ScanProject(projectPath, scanOpts)
	if err != nil {
		printError(fmt.Errorf("error scanning project: %w", err))
		return
	}
	printProgress("finished_scanning_local_files", map[string]int{"count": len(files)})

	tx, err := db.Begin()
	if err != nil {
		printError(fmt.Errorf("error starting transaction: %w", err))
		return
	}

	stmt, err := tx.Prepare("INSERT INTO file_metadata(project_id, relative_path, filename, extension, size_bytes, line_count, is_text, last_mod_time, content_hash) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		tx.Rollback()
		printError(fmt.Errorf("error preparing statement: %w", err))
		return
	}
	defer stmt.Close()

	printProgress("inserting_new_data", nil)
	for _, file := range files {
		_, err := stmt.Exec(projectID, file.RelativePath, file.Filename, file.Extension, file.SizeBytes, file.LineCount, file.IsText, file.LastModTime.Format(time.RFC3339Nano), file.ContentHash)
		if err != nil {
			tx.Rollback()
			printError(fmt.Errorf("error inserting file metadata for %s: %w", file.RelativePath, err))
			return
		}
	}

	tx.Commit()

	_, err = db.Exec("UPDATE projects SET last_scan_timestamp = ? WHERE id = ?", time.Now().UTC().Format(time.RFC3339), projectID)
	if err != nil {
		printError(fmt.Errorf("error updating project timestamp: %w", err))
		return
	}

	output := map[string]interface{}{
		"status":       "cache updated (full scan)",
		"filesScanned": len(files),
	}
	printJSON(output)
}

// --- Incremental Scan Logic ---
func runIncrementalScan(db *sql.DB, projectID int64, projectPath string, scanOpts scanner.ScanOptions) {
	printProgress("starting_incremental_scan", nil)

	// 1. Get current state from DB
	type dbFileInfo struct {
		ModTime time.Time
		Hash    string
	}
	dbFiles := make(map[string]dbFileInfo)
	rows, err := db.Query("SELECT relative_path, last_mod_time, content_hash FROM file_metadata WHERE project_id = ?", projectID)
	if err != nil {
		printError(fmt.Errorf("error fetching current cache state: %w", err))
		return
	}
	for rows.Next() {
		var path, modTimeStr, hash string
		if err := rows.Scan(&path, &modTimeStr, &hash); err != nil {
			rows.Close()
			printError(fmt.Errorf("error scanning db row: %w", err))
			return
		}
		modTime, _ := time.Parse(time.RFC3339Nano, modTimeStr)
		dbFiles[path] = dbFileInfo{ModTime: modTime, Hash: hash}
	}
	rows.Close()
	printProgress("fetched_db_metadata", map[string]int{"count": len(dbFiles)})

	// 2. Scan local file system
	printProgress("scanning_local_files", scanOpts)
	localFiles, err := scanner.ScanProject(projectPath, scanOpts)
	if err != nil {
		printError(fmt.Errorf("error scanning project: %w", err))
		return
	}
	printProgress("finished_scanning_local_files", map[string]int{"count": len(localFiles)})

	// 3. Compare and determine changes
	localFilesMap := make(map[string]scanner.FileMetadata)
	var toInsert, toUpdate []scanner.FileMetadata
	for _, f := range localFiles {
		localFilesMap[f.RelativePath] = f
		dbInfo, exists := dbFiles[f.RelativePath]
		if !exists {
			toInsert = append(toInsert, f) // New file
		} else if !f.LastModTime.Equal(dbInfo.ModTime) || f.ContentHash != dbInfo.Hash {
			toUpdate = append(toUpdate, f) // Modified file
		}
	}
	var toDelete []string
	for path := range dbFiles {
		if _, exists := localFilesMap[path]; !exists {
			toDelete = append(toDelete, path) // Deleted file
		}
	}
	printProgress("analyzed_diff", map[string]int{
		"new":      len(toInsert),
		"modified": len(toUpdate),
		"deleted":  len(toDelete),
	})

	if len(toInsert) == 0 && len(toUpdate) == 0 && len(toDelete) == 0 {
		output := map[string]interface{}{
			"status": "cache is up-to-date",
		}
		// Still need to update timestamp to indicate a scan was performed
		_, err = db.Exec("UPDATE projects SET last_scan_timestamp = ? WHERE id = ?", time.Now().UTC().Format(time.RFC3339), projectID)
		if err != nil {
			printError(fmt.Errorf("error updating project timestamp: %w", err))
			return
		}
		printJSON(output)
		return
	}

	// 4. Execute changes in a transaction
	tx, err := db.Begin()
	if err != nil {
		printError(fmt.Errorf("error starting transaction: %w", err))
		return
	}

	// Inserts
	if len(toInsert) > 0 {
		stmt, err := tx.Prepare("INSERT INTO file_metadata(project_id, relative_path, filename, extension, size_bytes, line_count, is_text, last_mod_time, content_hash) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)")
		if err != nil {
			tx.Rollback()
			printError(err)
			return
		}
		defer stmt.Close()
		for _, f := range toInsert {
			_, err := stmt.Exec(projectID, f.RelativePath, f.Filename, f.Extension, f.SizeBytes, f.LineCount, f.IsText, f.LastModTime.Format(time.RFC3339Nano), f.ContentHash)
			if err != nil {
				tx.Rollback()
				printError(err)
				return
			}
		}
	}
	// Updates
	if len(toUpdate) > 0 {
		stmt, err := tx.Prepare("UPDATE file_metadata SET size_bytes = ?, line_count = ?, is_text = ?, last_mod_time = ?, content_hash = ? WHERE project_id = ? AND relative_path = ?")
		if err != nil {
			tx.Rollback()
			printError(err)
			return
		}
		defer stmt.Close()
		for _, f := range toUpdate {
			_, err := stmt.Exec(f.SizeBytes, f.LineCount, f.IsText, f.LastModTime.Format(time.RFC3339Nano), f.ContentHash, projectID, f.RelativePath)
			if err != nil {
				tx.Rollback()
				printError(err)
				return
			}
		}
	}
	// Deletes
	if len(toDelete) > 0 {
		stmt, err := tx.Prepare("DELETE FROM file_metadata WHERE project_id = ? AND relative_path = ?")
		if err != nil {
			tx.Rollback()
			printError(err)
			return
		}
		defer stmt.Close()
		for _, path := range toDelete {
			_, err := stmt.Exec(projectID, path)
			if err != nil {
				tx.Rollback()
				printError(err)
				return
			}
		}
	}

	tx.Commit()

	_, err = db.Exec("UPDATE projects SET last_scan_timestamp = ? WHERE id = ?", time.Now().UTC().Format(time.RFC3339), projectID)
	if err != nil {
		printError(fmt.Errorf("error updating project timestamp: %w", err))
		return
	}

	output := map[string]interface{}{
		"status":         "cache updated (incremental scan)",
		"files_added":    len(toInsert),
		"files_modified": len(toUpdate),
		"files_deleted":  len(toDelete),
	}
	printJSON(output)
}

// --- `cache status` Command ---
var cacheStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Quickly checks if the project cache is stale",
	Long: `Quickly checks if the project's cache is stale by comparing file system metadata with the database records.
This is a very fast, read-only operation designed to give a UI a quick signal on whether a "cache update" is needed.
It checks for new files, deleted files, and modified files based on their last modification time.

This check respects .gitignore, but it IGNORES the default preset exclusions, because the creation of a new
'node_modules' directory is itself a change that makes the cache stale.
`,
	Run: func(cmd *cobra.Command, args []string) {
		dbPath, _ := cmd.Flags().GetString("db")
		projectPath, _ := cmd.Flags().GetString("project-path")
		noGitIgnores, _ := cmd.Flags().GetBool("no-git-ignores")

		db, err := database.InitializeDB(dbPath)
		if err != nil {
			printError(fmt.Errorf("error initializing database: %w", err))
			return
		}
		defer db.Close()

		projectID, err := getOrCreateProject(db, projectPath)
		if err != nil {
			printError(fmt.Errorf("error getting or creating project: %w", err))
			return
		}

		dbFiles := make(map[string]time.Time)
		rows, err := db.Query("SELECT relative_path, last_mod_time FROM file_metadata WHERE project_id = ?", projectID)
		if err != nil {
			printError(fmt.Errorf("error fetching current cache state: %w", err))
			return
		}
		for rows.Next() {
			var path, modTimeStr string
			if err := rows.Scan(&path, &modTimeStr); err != nil {
				rows.Close()
				printError(fmt.Errorf("error scanning db row: %w", err))
				return
			}
			modTime, _ := time.Parse(time.RFC3339Nano, modTimeStr)
			dbFiles[path] = modTime
		}
		rows.Close()

		var ignoreMatcher *gitignore.GitIgnore
		if !noGitIgnores {
			ignoreMatcher, _ = gitignore.CompileIgnoreFile(filepath.Join(projectPath, ".gitignore"))
		}

		isStale := false
		err = filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			relPath, err := filepath.Rel(projectPath, path)
			if err != nil {
				return err
			}
			if relPath == "." {
				return nil
			}

			if !noGitIgnores && ignoreMatcher != nil && ignoreMatcher.MatchesPath(relPath) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if info.IsDir() {
				if info.Name() == ".git" {
					return filepath.SkipDir
				}
				return nil
			}

			if !info.Mode().IsRegular() {
				return nil
			}

			dbModTime, exists := dbFiles[relPath]

			if !exists || !info.ModTime().Equal(dbModTime) {
				isStale = true
				return filepath.SkipDir
			}

			delete(dbFiles, relPath)
			return nil
		})

		if err != nil && err != filepath.SkipDir {
			printError(fmt.Errorf("error walking filesystem for status check: %w", err))
			return
		}

		if len(dbFiles) > 0 {
			isStale = true
		}

		printJSON(map[string]interface{}{"is_stale": isStale})
	},
}

func getOrCreateProject(db *sql.DB, projectPath string) (int64, error) {
	var projectID int64
	err := db.QueryRow("SELECT id FROM projects WHERE project_path = ?", projectPath).Scan(&projectID)
	if err != nil {
		if err == sql.ErrNoRows {
			res, err := db.Exec("INSERT INTO projects(project_path, last_scan_timestamp) VALUES(?, ?)", projectPath, "not_scanned_yet")
			if err != nil {
				return 0, err
			}
			return res.LastInsertId()
		} else {
			return 0, err
		}
	}
	return projectID, nil
}

func init() {
	rootCmd.AddCommand(cacheCmd)

	cacheCmd.AddCommand(cacheUpdateCmd)
	cacheUpdateCmd.Flags().String("db", "", "Path to the database file")
	cacheUpdateCmd.MarkFlagRequired("db")
	cacheUpdateCmd.Flags().String("project-path", "", "Path to the project")
	cacheUpdateCmd.MarkFlagRequired("project-path")
	cacheUpdateCmd.Flags().Bool("incremental", false, "Perform an incremental scan instead of a full one")
	cacheUpdateCmd.Flags().Bool("no-git-ignores", false, "Disable .gitignore file parsing")
	cacheUpdateCmd.Flags().Bool("include-binary", false, "Include binary files in the scan (default is text-only)")
	cacheUpdateCmd.Flags().Bool("no-preset-excludes", false, "Disable the default exclusion of common dependency directories")

	cacheCmd.AddCommand(cacheStatusCmd)
	cacheStatusCmd.Flags().String("db", "", "Path to the database file")
	cacheStatusCmd.MarkFlagRequired("db")
	cacheStatusCmd.Flags().String("project-path", "", "Path to the project")
	cacheStatusCmd.MarkFlagRequired("project-path")
	cacheStatusCmd.Flags().Bool("no-git-ignores", false, "Disable .gitignore file parsing")
}
