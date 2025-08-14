package cmd

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"code-prompt-core/pkg/database"
	"code-prompt-core/pkg/scanner"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage the project file cache",
	Long:  `Contains subcommands for creating, updating, and checking the status of a project's file metadata cache.`,
}

var cacheUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Create or update the cache for a project (full or incremental)",
	Long: `Performs a scan of the specified project and updates the cache in the database.

This is the core data-gathering command. It can perform two types of scans:
1. Full Scan (default): Clears any existing data for the project and scans everything from scratch.
2. Incremental Scan (--incremental): Much faster for subsequent scans. It compares the file system with the last cached state and only processes new, modified, or deleted files.

This command intelligently ignores files specified in '.gitignore' and common dependency directories (like 'node_modules', 'vendor', etc.) by default. This behavior can be modified with flags.

All parameters for this command can be configured in your config file under the 'cache.update' key.
For example:
  cache:
    update:
      project-path: /path/to/my/project
      incremental: true
      batch-size: 200`,
	Run: func(cmd *cobra.Command, args []string) {
		projectPath, err := getAbsoluteProjectPath("cache.update.project-path")
		if err != nil {
			printError(err)
			return
		}
		scanOpts := scanner.ScanOptions{
			NoGitIgnores:     viper.GetBool("cache.update.no-git-ignores"),
			IncludeBinary:    viper.GetBool("cache.update.include-binary"),
			NoPresetExcludes: viper.GetBool("cache.update.no-preset-excludes"),
		}
		db, err := database.InitializeDB(viper.GetString("db"))
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
		if !viper.GetBool("cache.update.incremental") {
			runFullScan(db, projectID, projectPath, scanOpts)
		} else {
			runIncrementalScan(db, projectID, projectPath, scanOpts, viper.GetInt("cache.update.batch-size"))
		}
	},
}

func runFullScan(db *sql.DB, projectID int64, projectPath string, scanOpts scanner.ScanOptions) {
	_, err := db.Exec("DELETE FROM file_metadata WHERE project_id = ?", projectID)
	if err != nil {
		printError(fmt.Errorf("error clearing old cache: %w", err))
		return
	}
	files, err := scanner.ScanProject(projectPath, scanOpts)
	if err != nil {
		printError(fmt.Errorf("error scanning project: %w", err))
		return
	}
	tx, err := db.Begin()
	if err != nil {
		printError(fmt.Errorf("error starting transaction: %w", err))
		return
	}
	if err := batchInsert(tx, projectID, files, viper.GetInt("cache.update.batch-size")); err != nil {
		tx.Rollback()
		printError(fmt.Errorf("full scan insert failed: %w", err))
		return
	}
	if err := tx.Commit(); err != nil {
		printError(fmt.Errorf("full scan commit failed: %w", err))
		return
	}
	db.Exec("UPDATE projects SET last_scan_timestamp = ? WHERE id = ?", time.Now().UTC().Format(time.RFC3339), projectID)
	printJSON(map[string]interface{}{
		"status":       "cache updated (full scan)",
		"filesScanned": len(files),
	})
}

func runIncrementalScan(db *sql.DB, projectID int64, projectPath string, scanOpts scanner.ScanOptions, batchSize int) {
	type dbFileInfo struct {
		ModTime time.Time
		Hash    string
	}
	dbFiles := make(map[string]dbFileInfo)
	rows, err := db.Query("SELECT relative_path, last_mod_time, content_hash FROM file_metadata WHERE project_id = ?", projectID)
	if err != nil {
		printError(err)
		return
	}
	for rows.Next() {
		var path, modTimeStr, hash string
		if err := rows.Scan(&path, &modTimeStr, &hash); err != nil {
			rows.Close()
			printError(err)
			return
		}
		modTime, _ := time.Parse(time.RFC3339Nano, modTimeStr)
		dbFiles[path] = dbFileInfo{ModTime: modTime, Hash: hash}
	}
	rows.Close()
	localFiles, err := scanner.ScanProject(projectPath, scanOpts)
	if err != nil {
		printError(err)
		return
	}
	localFilesMap := make(map[string]scanner.FileMetadata)
	var toInsert, toUpdate []scanner.FileMetadata
	for _, f := range localFiles {
		localFilesMap[f.RelativePath] = f
		dbInfo, exists := dbFiles[f.RelativePath]
		if !exists {
			toInsert = append(toInsert, f)
		} else if !f.LastModTime.Equal(dbInfo.ModTime) || f.ContentHash != dbInfo.Hash {
			toUpdate = append(toUpdate, f)
		}
	}
	var toDelete []string
	for path := range dbFiles {
		if _, exists := localFilesMap[path]; !exists {
			toDelete = append(toDelete, path)
		}
	}
	if len(toInsert) == 0 && len(toUpdate) == 0 && len(toDelete) == 0 {
		printJSON(map[string]interface{}{"status": "cache is up-to-date"})
		return
	}
	tx, err := db.Begin()
	if err != nil {
		printError(err)
		return
	}
	if err := batchInsert(tx, projectID, toInsert, batchSize); err != nil {
		tx.Rollback()
		printError(fmt.Errorf("batch insert failed: %w", err))
		return
	}
	if err := singleUpdate(tx, projectID, toUpdate); err != nil {
		tx.Rollback()
		printError(fmt.Errorf("update failed: %w", err))
		return
	}
	if err := batchDelete(tx, projectID, toDelete, batchSize); err != nil {
		tx.Rollback()
		printError(fmt.Errorf("batch delete failed: %w", err))
		return
	}
	if err := tx.Commit(); err != nil {
		printError(fmt.Errorf("transaction commit failed: %w", err))
		return
	}
	db.Exec("UPDATE projects SET last_scan_timestamp = ? WHERE id = ?", time.Now().UTC().Format(time.RFC3339), projectID)
	printJSON(map[string]interface{}{
		"status":         "cache updated (incremental scan)",
		"files_added":    len(toInsert),
		"files_modified": len(toUpdate),
		"files_deleted":  len(toDelete),
	})
}

func batchInsert(tx *sql.Tx, projectID int64, files []scanner.FileMetadata, batchSize int) error {
	if len(files) == 0 {
		return nil
	}
	sqlStr := "INSERT INTO file_metadata(project_id, relative_path, filename, extension, size_bytes, line_count, is_text, last_mod_time, content_hash) VALUES "
	for i := 0; i < len(files); i += batchSize {
		end := i + batchSize
		if end > len(files) {
			end = len(files)
		}
		batch := files[i:end]
		vals := []interface{}{}
		placeholders := make([]string, 0, len(batch))
		for _, f := range batch {
			placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?, ?, ?)")
			vals = append(vals, projectID, f.RelativePath, f.Filename, f.Extension, f.SizeBytes, f.LineCount, f.IsText, f.LastModTime.Format(time.RFC3339Nano), f.ContentHash)
		}
		batchSQL := sqlStr + strings.Join(placeholders, ",")
		if _, err := tx.Exec(batchSQL, vals...); err != nil {
			return err
		}
	}
	return nil
}

func singleUpdate(tx *sql.Tx, projectID int64, files []scanner.FileMetadata) error {
	if len(files) == 0 {
		return nil
	}
	stmt, err := tx.Prepare("UPDATE file_metadata SET size_bytes = ?, line_count = ?, is_text = ?, last_mod_time = ?, content_hash = ? WHERE project_id = ? AND relative_path = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, f := range files {
		_, err := stmt.Exec(f.SizeBytes, f.LineCount, f.IsText, f.LastModTime.Format(time.RFC3339Nano), f.ContentHash, projectID, f.RelativePath)
		if err != nil {
			return err
		}
	}
	return nil
}

func batchDelete(tx *sql.Tx, projectID int64, paths []string, batchSize int) error {
	if len(paths) == 0 {
		return nil
	}
	sqlStr := "DELETE FROM file_metadata WHERE project_id = ? AND relative_path IN ("
	for i := 0; i < len(paths); i += batchSize {
		end := i + batchSize
		if end > len(paths) {
			end = len(paths)
		}
		batch := paths[i:end]
		vals := []interface{}{projectID}
		for _, p := range batch {
			vals = append(vals, p)
		}
		placeholders := strings.Repeat("?,", len(batch)-1) + "?"
		batchSQL := sqlStr + placeholders + ")"
		if _, err := tx.Exec(batchSQL, vals...); err != nil {
			return err
		}
	}
	return nil
}

func getOrCreateProject(db *sql.DB, projectPath string) (int64, error) {
	var projectID int64
	err := db.QueryRow("SELECT id FROM projects WHERE project_path = ?", projectPath).Scan(&projectID)
	if err == sql.ErrNoRows {
		res, err := db.Exec("INSERT INTO projects(project_path, last_scan_timestamp) VALUES(?, ?)", projectPath, "not_scanned_yet")
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	}
	return projectID, err
}

func init() {
	rootCmd.AddCommand(cacheCmd)

	cacheCmd.AddCommand(cacheUpdateCmd)
	cacheUpdateCmd.Flags().String("project-path", "", "Path to the project")
	cacheUpdateCmd.Flags().Bool("incremental", false, "Perform an incremental scan")
	cacheUpdateCmd.Flags().Bool("no-git-ignores", false, "Disable .gitignore file parsing")
	cacheUpdateCmd.Flags().Bool("include-binary", false, "Include binary files in the scan")
	cacheUpdateCmd.Flags().Bool("no-preset-excludes", false, "Disable default exclusion of dependency directories")
	cacheUpdateCmd.Flags().Int("batch-size", 100, "Number of DB operations to batch in incremental scans")

	viper.BindPFlag("cache.update.project-path", cacheUpdateCmd.Flags().Lookup("project-path"))
	viper.BindPFlag("cache.update.incremental", cacheUpdateCmd.Flags().Lookup("incremental"))
	viper.BindPFlag("cache.update.no-git-ignores", cacheUpdateCmd.Flags().Lookup("no-git-ignores"))
	viper.BindPFlag("cache.update.include-binary", cacheUpdateCmd.Flags().Lookup("include-binary"))
	viper.BindPFlag("cache.update.no-preset-excludes", cacheUpdateCmd.Flags().Lookup("no-preset-excludes"))
	viper.BindPFlag("cache.update.batch-size", cacheUpdateCmd.Flags().Lookup("batch-size"))
}
