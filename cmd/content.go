// File: cmd/content.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	Long:  `Retrieves the contents of multiple files at once using regex filters.`,
	Run: func(cmd *cobra.Command, args []string) {
		projectPath, err := getAbsoluteProjectPath("content.get.project-path")
		if err != nil {
			printError(err)
			return
		}
		absProjectPath, err := filepath.Abs(projectPath)
		if err != nil {
			printError(fmt.Errorf("error resolving absolute path for '%s': %w", projectPath, err))
			return
		}

		db, err := database.InitializeDB(viper.GetString("db"))
		if err != nil {
			printError(fmt.Errorf("error initializing database: %w", err))
			return
		}
		defer db.Close()
		var projectID int64
		err = db.QueryRow("SELECT id FROM projects WHERE project_path = ?", absProjectPath).Scan(&projectID)
		if err != nil {
			printError(fmt.Errorf("error finding project: %w", err))
			return
		}

		f := filter.Filter{
			Includes: strings.Split(viper.GetString("content.get.includes"), ","),
			Excludes: strings.Split(viper.GetString("content.get.excludes"), ","),
			Priority: viper.GetString("content.get.priority"),
		}

		relativePaths, err := filter.GetFilteredFilePaths(db, projectID, f)
		if err != nil {
			printError(fmt.Errorf("error applying filters: %w", err))
			return
		}
		contentMap := make(map[string]string)
		for _, relPath := range relativePaths {
			// Use absProjectPath to read files
			fullPath := filepath.Join(absProjectPath, filepath.Clean(relPath))
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
	contentGetCmd.Flags().String("includes", "", "Comma-separated regex for files to include")
	contentGetCmd.Flags().String("excludes", "", "Comma-separated regex for files to exclude")
	contentGetCmd.Flags().String("priority", "includes", "Priority if a file matches both lists ('includes' or 'excludes')")

	viper.BindPFlag("content.get.project-path", contentGetCmd.Flags().Lookup("project-path"))
	viper.BindPFlag("content.get.includes", contentGetCmd.Flags().Lookup("includes"))
	viper.BindPFlag("content.get.excludes", contentGetCmd.Flags().Lookup("excludes"))
	viper.BindPFlag("content.get.priority", contentGetCmd.Flags().Lookup("priority"))
}
