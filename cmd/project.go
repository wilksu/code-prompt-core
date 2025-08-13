package cmd

import (
	"code-prompt-core/pkg/database"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage projects within the database",
	Long:  `The "project" command group allows you to add, list, and delete project records in the database. A project record must exist before you can manage its cache or profiles.`,
}

var projectAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Adds a new project to the database without scanning",
	Long: `This lightweight command creates a project record in the database, allowing profile management or other configurations before performing the first (potentially long) scan.
If the project already exists, this command will do nothing and will not return an error.

Example:
  code-prompt-core project add --project-path /path/to/my-new-project`,
	Run: func(cmd *cobra.Command, args []string) {
		projectPath := viper.GetString("project.add.project-path")
		if projectPath == "" {
			printError(fmt.Errorf("flag --project-path is required"))
			return
		}
		db, err := database.InitializeDB(viper.GetString("db"))
		if err != nil {
			printError(fmt.Errorf("error initializing database: %w", err))
			return
		}
		defer db.Close()
		_, err = db.Exec("INSERT OR IGNORE INTO projects(project_path, last_scan_timestamp) VALUES(?, ?)", projectPath, "not_scanned_yet")
		if err != nil {
			printError(fmt.Errorf("error adding project: %w", err))
			return
		}
		printJSON(fmt.Sprintf("Project '%s' is ready.", projectPath))
	},
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all projects stored in the database",
	Long:  `Retrieves and displays a list of all projects currently managed in the specified database file, along with the timestamp of their last scan.`,
	Run: func(cmd *cobra.Command, args []string) {
		db, err := database.InitializeDB(viper.GetString("db"))
		if err != nil {
			printError(fmt.Errorf("error initializing database: %w", err))
			return
		}
		defer db.Close()
		rows, err := db.Query("SELECT project_path, last_scan_timestamp FROM projects")
		if err != nil {
			printError(fmt.Errorf("error querying projects: %w", err))
			return
		}
		defer rows.Close()
		type Project struct {
			ProjectPath       string `json:"project_path"`
			LastScanTimestamp string `json:"last_scan_timestamp"`
		}
		var projects []Project
		for rows.Next() {
			var p Project
			if err := rows.Scan(&p.ProjectPath, &p.LastScanTimestamp); err != nil {
				printError(fmt.Errorf("error scanning row: %w", err))
				return
			}
			projects = append(projects, p)
		}
		printJSON(projects)
	},
}

var projectDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a project and all its associated data",
	Long: `Deletes a project record from the database.
Due to the database schema's 'ON DELETE CASCADE' setting, this will also automatically delete all associated file metadata and saved filter profiles for that project. This action is irreversible.

Example:
  code-prompt-core project delete --project-path /path/to/project-to-delete`,
	Run: func(cmd *cobra.Command, args []string) {
		projectPath := viper.GetString("project.delete.project-path")
		if projectPath == "" {
			printError(fmt.Errorf("flag --project-path is required"))
			return
		}
		db, err := database.InitializeDB(viper.GetString("db"))
		if err != nil {
			printError(fmt.Errorf("error initializing database: %w", err))
			return
		}
		defer db.Close()
		result, err := db.Exec("DELETE FROM projects WHERE project_path = ?", projectPath)
		if err != nil {
			printError(fmt.Errorf("error deleting project: %w", err))
			return
		}
		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			printError(fmt.Errorf("no project found with path: %s", projectPath))
			return
		}
		printJSON(fmt.Sprintf("Project '%s' and all its data deleted successfully.", projectPath))
	},
}

func init() {
	rootCmd.AddCommand(projectCmd)
	projectCmd.AddCommand(projectAddCmd)
	projectAddCmd.Flags().String("project-path", "", "Path to the project")
	viper.BindPFlag("project.add.project-path", projectAddCmd.Flags().Lookup("project-path"))

	projectCmd.AddCommand(projectListCmd)

	projectCmd.AddCommand(projectDeleteCmd)
	projectDeleteCmd.Flags().String("project-path", "", "Path to the project")
	viper.BindPFlag("project.delete.project-path", projectDeleteCmd.Flags().Lookup("project-path"))
}
