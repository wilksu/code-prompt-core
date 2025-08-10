package cmd

import (
	"encoding/json"
	"fmt"

	"code-prompt-core/pkg/database"
	"github.com/spf13/cobra"
)

var profilesCmd = &cobra.Command{
	Use:   "profiles",
	Short: "Manage filter profiles",
}

var profilesSaveCmd = &cobra.Command{
	Use:   "save",
	Short: "Save a filter profile",
	Long: `Saves a filter configuration as a named profile.
The --data flag accepts a JSON string with the same structure as the --filter-json flag in the 'analyze:filter' command.

Example:
--data '{"excludedExtensions":[".tmp", ".bak"], "isTextOnly":true}'`,
	Run: func(cmd *cobra.Command, args []string) {
		dbPath, _ := cmd.Flags().GetString("db")
		projectPath, _ := cmd.Flags().GetString("project-path")
		profileName, _ := cmd.Flags().GetString("name")
		profileData, _ := cmd.Flags().GetString("data")

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

		_, err = db.Exec("INSERT INTO profiles (project_id, profile_name, profile_data_json) VALUES (?, ?, ?)", projectID, profileName, profileData)
		if err != nil {
			printError(fmt.Errorf("error saving profile: %w", err))
			return
		}

		printJSON(fmt.Sprintf("Profile '%s' saved successfully.", profileName))
	},
}

var profilesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all profiles for a project",
	Run: func(cmd *cobra.Command, args []string) {
		dbPath, _ := cmd.Flags().GetString("db")
		projectPath, _ := cmd.Flags().GetString("project-path")

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

		rows, err := db.Query("SELECT profile_name, profile_data_json FROM profiles WHERE project_id = ?", projectID)
		if err != nil {
			printError(fmt.Errorf("error listing profiles: %w", err))
			return
		}
		defer rows.Close()

		type Profile struct {
			Name string `json:"name"`
			Data string `json:"data"`
		}

		var profiles []Profile
		for rows.Next() {
			var p Profile
			if err := rows.Scan(&p.Name, &p.Data); err != nil {
				printError(fmt.Errorf("error scanning row: %w", err))
				return
			}
			profiles = append(profiles, p)
		}

		printJSON(profiles)
	},
}

var profilesLoadCmd = &cobra.Command{
	Use:   "load",
	Short: "Load a filter profile",
	Run: func(cmd *cobra.Command, args []string) {
		dbPath, _ := cmd.Flags().GetString("db")
		projectPath, _ := cmd.Flags().GetString("project-path")
		profileName, _ := cmd.Flags().GetString("name")

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

		var profileData string
		err = db.QueryRow("SELECT profile_data_json FROM profiles WHERE project_id = ? AND profile_name = ?", projectID, profileName).Scan(&profileData)
		if err != nil {
			printError(fmt.Errorf("error loading profile: %w", err))
			return
		}

		// Attempt to unmarshal to verify it's valid JSON, then return the raw string
		var js json.RawMessage
		if err := json.Unmarshal([]byte(profileData), &js); err != nil {
			printError(fmt.Errorf("profile data is not valid JSON: %w", err))
			return
		}

		printJSON(js)
	},
}

var profilesDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a filter profile",
	Run: func(cmd *cobra.Command, args []string) {
		dbPath, _ := cmd.Flags().GetString("db")
		projectPath, _ := cmd.Flags().GetString("project-path")
		profileName, _ := cmd.Flags().GetString("name")

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

		result, err := db.Exec("DELETE FROM profiles WHERE project_id = ? AND profile_name = ?", projectID, profileName)
		if err != nil {
			printError(fmt.Errorf("error deleting profile: %w", err))
			return
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			printError(fmt.Errorf("no profile found with name: %s", profileName))
			return
		}

		printJSON(fmt.Sprintf("Profile '%s' deleted successfully.", profileName))
	},
}

func init() {
	rootCmd.AddCommand(profilesCmd)
	profilesCmd.AddCommand(profilesSaveCmd)
	profilesSaveCmd.Flags().String("db", "", "Path to the database file")
	profilesSaveCmd.MarkFlagRequired("db")
	profilesSaveCmd.Flags().String("project-path", "", "Path to the project")
	profilesSaveCmd.MarkFlagRequired("project-path")
	profilesSaveCmd.Flags().String("name", "", "Name of the profile")
	profilesSaveCmd.MarkFlagRequired("name")
	profilesSaveCmd.Flags().String("data", "", "JSON data for the profile")
	profilesSaveCmd.MarkFlagRequired("data")

	profilesCmd.AddCommand(profilesListCmd)
	profilesListCmd.Flags().String("db", "", "Path to the database file")
	profilesListCmd.MarkFlagRequired("db")
	profilesListCmd.Flags().String("project-path", "", "Path to the project")
	profilesListCmd.MarkFlagRequired("project-path")

	profilesCmd.AddCommand(profilesLoadCmd)
	profilesLoadCmd.Flags().String("db", "", "Path to the database file")
	profilesLoadCmd.MarkFlagRequired("db")
	profilesLoadCmd.Flags().String("project-path", "", "Path to the project")
	profilesLoadCmd.MarkFlagRequired("project-path")
	profilesLoadCmd.Flags().String("name", "", "Name of the profile")
	profilesLoadCmd.MarkFlagRequired("name")

	profilesCmd.AddCommand(profilesDeleteCmd)
	profilesDeleteCmd.Flags().String("db", "", "Path to the database file")
	profilesDeleteCmd.MarkFlagRequired("db")
	profilesDeleteCmd.Flags().String("project-path", "", "Path to the project")
	profilesDeleteCmd.MarkFlagRequired("project-path")
	profilesDeleteCmd.Flags().String("name", "", "Name of the profile")
	profilesDeleteCmd.MarkFlagRequired("name")
}