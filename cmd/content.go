package cmd

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"code-prompt-core/pkg/database"
	"code-prompt-core/pkg/filter"

	"github.com/spf13/cobra"
)

var contentCmd = &cobra.Command{
	Use:   "content",
	Short: "Retrieve file contents",
}

var contentGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Batch gets the content of specified files for a project",
	Long:  "Retrieves the contents of multiple files at once. Provide a list of files with --files-json, or a filter with --profile-name or --filter-json.",
	// --- Start of fix ---
	Run: func(cmd *cobra.Command, args []string) {
		dbPath, _ := cmd.Flags().GetString("db")
		projectPath, _ := cmd.Flags().GetString("project-path")
		filesJSON, _ := cmd.Flags().GetString("files-json")
		profileName, _ := cmd.Flags().GetString("profile-name")
		filterJSON, _ := cmd.Flags().GetString("filter-json")

		// --- Input Validation ---
		modeCount := 0
		if filesJSON != "" {
			modeCount++
		}
		if profileName != "" {
			modeCount++
		}
		if filterJSON != "" {
			modeCount++
		}

		if modeCount == 0 {
			printError(fmt.Errorf("you must provide one of --files-json, --profile-name, or --filter-json"))
			return
		}
		if modeCount > 1 {
			printError(fmt.Errorf("flags --files-json, --profile-name, and --filter-json are mutually exclusive"))
			return
		}

		// --- Common Setup ---
		db, err := database.InitializeDB(dbPath)
		if err != nil {
			printError(fmt.Errorf("error initializing database: %w", err))
			return
		}
		defer db.Close()

		var projectID int64
		err = db.QueryRow("SELECT id FROM projects WHERE project_path = ?", projectPath).Scan(&projectID)
		if err != nil {
			printError(fmt.Errorf("error finding project: %w", err))
			return
		}

		var relativePaths []string

		// --- Logic to get file paths based on mode ---
		if filesJSON != "" {
			// Mode 1: Old way, using a direct list of files
			if err := json.Unmarshal([]byte(filesJSON), &relativePaths); err != nil {
				printError(fmt.Errorf("error parsing --files-json: %w", err))
				return
			}
		} else {
			// Mode 2: New way, using filter rules
			var f filter.Filter
			var finalFilterJSON string

			if profileName != "" {
				// Load filter from a saved profile
				err := db.QueryRow("SELECT profile_data_json FROM profiles WHERE project_id = ? AND profile_name = ?", projectID, profileName).Scan(&finalFilterJSON)
				if err != nil {
					if err == sql.ErrNoRows {
						printError(fmt.Errorf("profile '%s' not found for this project", profileName))
					} else {
						printError(fmt.Errorf("error loading profile: %w", err))
					}
					return
				}
			} else { // filterJSON must be set
				finalFilterJSON = filterJSON
			}

			if err := json.Unmarshal([]byte(finalFilterJSON), &f); err != nil {
				printError(fmt.Errorf("error parsing filter JSON: %w", err))
				return
			}

			// Use the refactored function to get the file list!
			paths, err := filter.GetFilteredFilePaths(db, projectID, f)
			if err != nil {
				printError(fmt.Errorf("error applying filters: %w", err))
				return
			}
			relativePaths = paths
		}

		// --- Common logic to read and return content ---
		contentMap := make(map[string]string)
		for _, relPath := range relativePaths {
			cleanRelPath := filepath.Clean(relPath)
			fullPath := filepath.Join(projectPath, cleanRelPath)

			content, err := os.ReadFile(fullPath)
			if err != nil {
				contentMap[relPath] = fmt.Sprintf("Error: Unable to read file. %v", err)
			} else {
				contentMap[relPath] = string(content)
			}
		}

		printJSON(contentMap)
	},
}

func init() {
	rootCmd.AddCommand(contentCmd)
	contentCmd.AddCommand(contentGetCmd)

	contentGetCmd.Flags().String("db", "", "Path to the database file")
	contentGetCmd.MarkFlagRequired("db")
	contentGetCmd.Flags().String("project-path", "", "Path to the project")
	contentGetCmd.MarkFlagRequired("project-path")
	contentGetCmd.Flags().String("profile-name", "", "Name of a saved filter profile to use")
	contentGetCmd.Flags().String("filter-json", "", "A temporary JSON string with filter conditions to use")
	contentGetCmd.MarkFlagRequired("files-json")
}
