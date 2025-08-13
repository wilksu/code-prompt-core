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
	"github.com/spf13/viper"
)

var contentCmd = &cobra.Command{
	Use:   "content",
	Short: "Retrieve file contents",
}

var contentGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Batch gets the content of specified files for a project",
	Long:  `Retrieves the contents of multiple files at once using a filter.`,
	Run: func(cmd *cobra.Command, args []string) {
		projectPath := viper.GetString("content.get.project-path")
		profileName := viper.GetString("content.get.profile-name")
		filterJSON := viper.GetString("content.get.filter-json")

		if projectPath == "" {
			printError(fmt.Errorf("project-path is required"))
			return
		}
		if profileName == "" && filterJSON == "" {
			printError(fmt.Errorf("one of --profile-name or --filter-json is required"))
			return
		}
		db, err := database.InitializeDB(viper.GetString("db"))
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
		var f filter.Filter
		var finalFilterJSON string
		if profileName != "" {
			err := db.QueryRow("SELECT profile_data_json FROM profiles WHERE project_id = ? AND profile_name = ?", projectID, profileName).Scan(&finalFilterJSON)
			if err != nil {
				if err == sql.ErrNoRows {
					printError(fmt.Errorf("profile '%s' not found for this project", profileName))
				} else {
					printError(fmt.Errorf("error loading profile: %w", err))
				}
				return
			}
		} else {
			finalFilterJSON = filterJSON
		}
		if err := json.Unmarshal([]byte(finalFilterJSON), &f); err != nil {
			printError(fmt.Errorf("error parsing filter JSON: %w", err))
			return
		}
		relativePaths, err := filter.GetFilteredFilePaths(db, projectID, f)
		if err != nil {
			printError(fmt.Errorf("error applying filters: %w", err))
			return
		}
		contentMap := make(map[string]string)
		for _, relPath := range relativePaths {
			fullPath := filepath.Join(projectPath, filepath.Clean(relPath))
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

	contentGetCmd.Flags().String("project-path", "", "Path to the project")
	contentGetCmd.Flags().String("profile-name", "", "Name of a saved filter profile to use")
	contentGetCmd.Flags().String("filter-json", "", "A temporary JSON string with filter conditions to use")

	viper.BindPFlag("content.get.project-path", contentGetCmd.Flags().Lookup("project-path"))
	viper.BindPFlag("content.get.profile-name", contentGetCmd.Flags().Lookup("profile-name"))
	viper.BindPFlag("content.get.filter-json", contentGetCmd.Flags().Lookup("filter-json"))
}
