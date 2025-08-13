package cmd

import (
	"encoding/json"
	"fmt"

	"code-prompt-core/pkg/database"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var profilesCmd = &cobra.Command{
	Use:   "profiles",
	Short: "Manage filter profiles",
}

var profilesSaveCmd = &cobra.Command{
	Use:   "save",
	Short: "Save a filter profile",
	Long:  `Saves or updates a filter configuration as a named profile.`,
	Run: func(cmd *cobra.Command, args []string) {
		projectPath := viper.GetString("profiles.save.project-path")
		profileName := viper.GetString("profiles.save.name")
		profileData := viper.GetString("profiles.save.data")
		if projectPath == "" || profileName == "" || profileData == "" {
			printError(fmt.Errorf("--project-path, --name, and --data are required"))
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
		upsertSQL := `INSERT INTO profiles (project_id, profile_name, profile_data_json) VALUES (?, ?, ?) ON CONFLICT(project_id, profile_name) DO UPDATE SET profile_data_json = excluded.profile_data_json;`
		_, err = db.Exec(upsertSQL, projectID, profileName, profileData)
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
		projectPath := viper.GetString("profiles.list.project-path")
		if projectPath == "" {
			printError(fmt.Errorf("--project-path is required"))
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
		rows, err := db.Query("SELECT profile_name, profile_data_json FROM profiles WHERE project_id = ?", projectID)
		if err != nil {
			printError(fmt.Errorf("error listing profiles: %w", err))
			return
		}
		defer rows.Close()
		type Profile struct {
			Name string          `json:"name"`
			Data json.RawMessage `json:"data"`
		}
		var profiles []Profile
		for rows.Next() {
			var p Profile
			var dataStr string
			if err := rows.Scan(&p.Name, &dataStr); err != nil {
				printError(fmt.Errorf("error scanning row: %w", err))
				return
			}
			p.Data = json.RawMessage(dataStr)
			profiles = append(profiles, p)
		}
		printJSON(profiles)
	},
}

var profilesLoadCmd = &cobra.Command{
	Use:   "load",
	Short: "Load a filter profile",
	Run: func(cmd *cobra.Command, args []string) {
		projectPath := viper.GetString("profiles.load.project-path")
		profileName := viper.GetString("profiles.load.name")
		if projectPath == "" || profileName == "" {
			printError(fmt.Errorf("--project-path and --name are required"))
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
		var profileData string
		err = db.QueryRow("SELECT profile_data_json FROM profiles WHERE project_id = ? AND profile_name = ?", projectID, profileName).Scan(&profileData)
		if err != nil {
			printError(fmt.Errorf("error loading profile: %w", err))
			return
		}
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
		projectPath := viper.GetString("profiles.delete.project-path")
		profileName := viper.GetString("profiles.delete.name")
		if projectPath == "" || profileName == "" {
			printError(fmt.Errorf("--project-path and --name are required"))
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
	profilesSaveCmd.Flags().String("project-path", "", "Path to the project")
	profilesSaveCmd.Flags().String("name", "", "Name of the profile")
	profilesSaveCmd.Flags().String("data", "", "JSON data for the profile")
	viper.BindPFlag("profiles.save.project-path", profilesSaveCmd.Flags().Lookup("project-path"))
	viper.BindPFlag("profiles.save.name", profilesSaveCmd.Flags().Lookup("name"))
	viper.BindPFlag("profiles.save.data", profilesSaveCmd.Flags().Lookup("data"))

	profilesCmd.AddCommand(profilesListCmd)
	profilesListCmd.Flags().String("project-path", "", "Path to the project")
	viper.BindPFlag("profiles.list.project-path", profilesListCmd.Flags().Lookup("project-path"))

	profilesCmd.AddCommand(profilesLoadCmd)
	profilesLoadCmd.Flags().String("project-path", "", "Path to the project")
	profilesLoadCmd.Flags().String("name", "", "Name of the profile")
	viper.BindPFlag("profiles.load.project-path", profilesLoadCmd.Flags().Lookup("project-path"))
	viper.BindPFlag("profiles.load.name", profilesLoadCmd.Flags().Lookup("name"))

	profilesCmd.AddCommand(profilesDeleteCmd)
	profilesDeleteCmd.Flags().String("project-path", "", "Path to the project")
	profilesDeleteCmd.Flags().String("name", "", "Name of the profile")
	viper.BindPFlag("profiles.delete.project-path", profilesDeleteCmd.Flags().Lookup("project-path"))
	viper.BindPFlag("profiles.delete.name", profilesDeleteCmd.Flags().Lookup("name"))
}
