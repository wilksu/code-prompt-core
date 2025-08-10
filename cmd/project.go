package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"code-prompt-core/pkg/database"
	"github.com/spf13/cobra"
)

// Response is the standardized JSON response structure.

type Response struct {
	Status  string      `json:"status"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

func printJSON(data interface{}) {
	resp := Response{Status: "success", Data: data}
	bytes, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		printError(fmt.Errorf("failed to marshal JSON response: %w", err))
		return
	}
	fmt.Println(string(bytes))
}

func printError(err error) {
	resp := Response{Status: "error", Message: err.Error()}
	bytes, _ := json.MarshalIndent(resp, "", "  ")
	// Output error to stderr
	fmt.Fprintln(os.Stderr, string(bytes))
	os.Exit(1)
}

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage projects",
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all projects",
	Run: func(cmd *cobra.Command, args []string) {
		dbPath, _ := cmd.Flags().GetString("db")
		db, err := database.InitializeDB(dbPath)
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
			ProjectPath      string `json:"project_path"`
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
	Short: "Delete a project",
	Run: func(cmd *cobra.Command, args []string) {
		dbPath, _ := cmd.Flags().GetString("db")
		projectPath, _ := cmd.Flags().GetString("project-path")

		db, err := database.InitializeDB(dbPath)
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

		printJSON(fmt.Sprintf("Project '%s' deleted successfully.", projectPath))
	},
}

func init() {
	rootCmd.AddCommand(projectCmd)
	projectCmd.AddCommand(projectListCmd)
	projectListCmd.Flags().String("db", "", "Path to the database file")
	projectListCmd.MarkFlagRequired("db")

	projectCmd.AddCommand(projectDeleteCmd)
	projectDeleteCmd.Flags().String("db", "", "Path to the database file")
	projectDeleteCmd.MarkFlagRequired("db")
	projectDeleteCmd.Flags().String("project-path", "", "Path to the project")
	projectDeleteCmd.MarkFlagRequired("project-path")
}
