// File: cmd/content.go
package cmd

import (
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
	Short: "Batch gets the content of filtered files for a project",
	Long: `Retrieves the contents of multiple files at once using a filter.

You can filter the files using either a saved profile via '--profile-name'
or a temporary filter via '--filter-json'. This command reads the
file contents from disk based on the file paths retrieved from the cache.

Example:
  code-prompt-core content get --project-path /p/proj --filter-json '{"includeExts":[".go"]}'
  code-prompt-core content get --project-path /p/proj --profile-name "go-source"
`,
	Run: func(cmd *cobra.Command, args []string) {
		projectPath, err := getAbsoluteProjectPath("content.get.project-path")
		if err != nil {
			printError(err)
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

		// *** 修改：使用 getFilter 帮助函数 ***
		f, err := getFilter(
			db,
			projectID,
			viper.GetString("content.get.profile-name"),
			viper.GetString("content.get.filter-json"),
		)
		if err != nil {
			printError(err)
			return
		}

		relativePaths, err := filter.GetFilteredFilePaths(db, projectID, f)
		if err != nil {
			printError(fmt.Errorf("error applying filters: %w", err))
			return
		}
		contentMap := make(map[string]string)
		for _, relPath := range relativePaths {
			// *** 修改：使用 projectPath (abs) ***
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

	// *** 修改：移除旧标志，添加新标志 ***
	contentGetCmd.Flags().String("project-path", "", "Path to the project")
	contentGetCmd.Flags().String("profile-name", "", "Name of a saved filter profile to use")
	contentGetCmd.Flags().String("filter-json", "", "A temporary JSON string with filter conditions")

	viper.BindPFlag("content.get.project-path", contentGetCmd.Flags().Lookup("project-path"))
	viper.BindPFlag("content.get.profile-name", contentGetCmd.Flags().Lookup("profile-name"))
	viper.BindPFlag("content.get.filter-json", contentGetCmd.Flags().Lookup("filter-json"))
}
