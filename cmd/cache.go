package cmd

import (
	"database/sql"
	"fmt"
	"time"

	"code-prompt-core/pkg/database"
	"code-prompt-core/pkg/scanner"

	"github.com/spf13/cobra"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage the cache",
}

var cacheUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update the cache for a project",
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

		_, err = db.Exec("DELETE FROM file_metadata WHERE project_id = ?", projectID)
		if err != nil {
			printError(fmt.Errorf("error clearing old cache: %w", err))
			return
		}

		files, err := scanner.ScanProject(projectPath, noGitIgnores)
		if err != nil {
			printError(fmt.Errorf("error scanning project: %w", err))
			return
		}

		tx, err := db.Begin()
		if err != nil {
			printError(fmt.Errorf("error starting transaction: %w", err))
			return
		}

		stmt, err := tx.Prepare("INSERT INTO file_metadata(project_id, relative_path, filename, extension, size_bytes, line_count, is_text) VALUES(?, ?, ?, ?, ?, ?, ?)")
		if err != nil {
			printError(fmt.Errorf("error preparing statement: %w", err))
			return
		}
		defer stmt.Close()

		for _, file := range files {
			_, err := stmt.Exec(projectID, file.RelativePath, file.Filename, file.Extension, file.SizeBytes, file.LineCount, file.IsText)
			if err != nil {
				tx.Rollback()
				printError(fmt.Errorf("error inserting file metadata: %w", err))
				return
			}
		}

		tx.Commit()

		_, err = db.Exec("UPDATE projects SET last_scan_timestamp = ? WHERE id = ?", time.Now().Format(time.RFC3339), projectID)
		if err != nil {
			printError(fmt.Errorf("error updating project timestamp: %w", err))
			return
		}

		output := map[string]interface{}{
			"status":       "cache updated",
			"filesScanned": len(files),
		}

		printJSON(output)
	},
}

func getOrCreateProject(db *sql.DB, projectPath string) (int64, error) {
	var projectID int64
	err := db.QueryRow("SELECT id FROM projects WHERE project_path = ?", projectPath).Scan(&projectID)
	if err != nil {
		if err == sql.ErrNoRows {
			res, err := db.Exec("INSERT INTO projects(project_path, last_scan_timestamp) VALUES(?, ?)", projectPath, time.Now().Format(time.RFC3339))
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
	cacheUpdateCmd.Flags().String("config-json", "", "JSON string with exclude rules")
	cacheUpdateCmd.Flags().Bool("no-default-ignores", false, "Disable default ignore rules")
	cacheUpdateCmd.Flags().Bool("no-git-ignores", false, "Disable .gitignore file parsing")
	cacheUpdateCmd.Flags().Bool("text-only", false, "Only scan text files")
}
