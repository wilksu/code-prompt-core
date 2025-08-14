// File: cmd/profiles.go
package cmd

import (
	"encoding/json"
	"fmt"

	"code-prompt-core/pkg/database"
	"code-prompt-core/pkg/filter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var profilesCmd = &cobra.Command{
	Use:   "profiles",
	Short: "Manage filter profiles for projects",
	Long:  `A filter profile is a saved set of filter rules that can be reused across different commands. This command group allows you to save, list, load, and delete these profiles.`,
}

var profilesSaveCmd = &cobra.Command{
	Use:   "save",
	Short: "Save or update a filter profile",
	Long: `Saves a filter configuration as a named profile for a specific project. If a profile with the same name already exists, it will be updated.

The filter configuration must be provided as a JSON string via the --data flag.
The JSON structure should be:
{
  "includes": ["<regex1>", "<regex2>", ...],
  "excludes": ["<regex3>", "<regex4>", ...],
  "priority": "includes"
}

- "includes": A list of regex patterns. Files matching any of these will be included.
- "excludes": A list of regex patterns. Files matching any of these will be excluded.
- "priority": Optional. Can be "includes" or "excludes". Determines which rule wins if a file matches both lists. Defaults to "includes".

Example:
  code-prompt-core profiles save --project-path /p/my-go-proj --name "go-files-only" --data '{"includes":["\\.go$"]}'`,
	Run: func(cmd *cobra.Command, args []string) {
		profileName := viper.GetString("profiles.save.name")
		profileData := viper.GetString("profiles.save.data")
		if profileName == "" || profileData == "" {
			printError(fmt.Errorf("--name and --data are required"))
			return
		}

		// Validate that the data is valid JSON for a filter
		var f filter.Filter
		if err := json.Unmarshal([]byte(profileData), &f); err != nil {
			printError(fmt.Errorf("invalid JSON format for --data: %w", err))
			return
		}

		absProjectPath, err := getAbsoluteProjectPath("profiles.save.project-path")
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
		err = db.QueryRow("SELECT id FROM projects WHERE project_path = ?", absProjectPath).Scan(&projectID)
		if err != nil {
			printError(fmt.Errorf("error finding project '%s': %w", absProjectPath, err))
			return
		}
		upsertSQL := `INSERT INTO profiles (project_id, profile_name, profile_data_json) VALUES (?, ?, ?) ON CONFLICT(project_id, profile_name) DO UPDATE SET profile_data_json = excluded.profile_data_json;`
		_, err = db.Exec(upsertSQL, projectID, profileName, profileData)
		if err != nil {
			printError(fmt.Errorf("error saving profile: %w", err))
			return
		}
		printJSON(fmt.Sprintf("Profile '%s' saved successfully for project '%s'.", profileName, absProjectPath))
	},
}

var profilesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all saved profiles for a project",
	Long:  `Retrieves and displays all filter profiles that have been saved for a specific project.`,
	Run: func(cmd *cobra.Command, args []string) {
		absProjectPath, err := getAbsoluteProjectPath("profiles.list.project-path")
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
		err = db.QueryRow("SELECT id FROM projects WHERE project_path = ?", absProjectPath).Scan(&projectID)
		if err != nil {
			printError(fmt.Errorf("error finding project '%s': %w", absProjectPath, err))
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
	Short: "Load and display a specific filter profile",
	Long:  `Loads a single, named filter profile for a project and displays its JSON content.`,
	Run: func(cmd *cobra.Command, args []string) {
		profileName := viper.GetString("profiles.load.name")
		if profileName == "" {
			printError(fmt.Errorf("--name is required"))
			return
		}
		absProjectPath, err := getAbsoluteProjectPath("profiles.load.project-path")
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
		err = db.QueryRow("SELECT id FROM projects WHERE project_path = ?", absProjectPath).Scan(&projectID)
		if err != nil {
			printError(fmt.Errorf("error finding project '%s': %w", absProjectPath, err))
			return
		}
		var profileData string
		err = db.QueryRow("SELECT profile_data_json FROM profiles WHERE project_id = ? AND profile_name = ?", projectID, profileName).Scan(&profileData)
		if err != nil {
			printError(fmt.Errorf("error loading profile '%s': %w", profileName, err))
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
	Long:  `Deletes a named filter profile from a project. This action is irreversible.`,
	Run: func(cmd *cobra.Command, args []string) {
		profileName := viper.GetString("profiles.delete.name")
		if profileName == "" {
			printError(fmt.Errorf("--name is required"))
			return
		}
		absProjectPath, err := getAbsoluteProjectPath("profiles.delete.project-path")
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
		err = db.QueryRow("SELECT id FROM projects WHERE project_path = ?", absProjectPath).Scan(&projectID)
		if err != nil {
			printError(fmt.Errorf("error finding project '%s': %w", absProjectPath, err))
			return
		}
		result, err := db.Exec("DELETE FROM profiles WHERE project_id = ? AND profile_name = ?", projectID, profileName)
		if err != nil {
			printError(fmt.Errorf("error deleting profile: %w", err))
			return
		}
		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			printError(fmt.Errorf("no profile found with name '%s' for project '%s'", profileName, absProjectPath))
			return
		}
		printJSON(fmt.Sprintf("Profile '%s' deleted successfully.", profileName))
	},
}

func init() {
	rootCmd.AddCommand(profilesCmd)

	profilesCmd.AddCommand(profilesSaveCmd)
	profilesSaveCmd.Flags().String("project-path", "", "Path to the project")
	profilesSaveCmd.Flags().String("name", "", "Name of the profile to save")
	profilesSaveCmd.Flags().String("data", "", "JSON data for the profile's filter rules")
	viper.BindPFlag("profiles.save.project-path", profilesSaveCmd.Flags().Lookup("project-path"))
	viper.BindPFlag("profiles.save.name", profilesSaveCmd.Flags().Lookup("name"))
	viper.BindPFlag("profiles.save.data", profilesSaveCmd.Flags().Lookup("data"))

	profilesCmd.AddCommand(profilesListCmd)
	profilesListCmd.Flags().String("project-path", "", "Path to the project")
	viper.BindPFlag("profiles.list.project-path", profilesListCmd.Flags().Lookup("project-path"))

	profilesCmd.AddCommand(profilesLoadCmd)
	profilesLoadCmd.Flags().String("project-path", "", "Path to the project")
	profilesLoadCmd.Flags().String("name", "", "Name of the profile to load")
	viper.BindPFlag("profiles.load.project-path", profilesLoadCmd.Flags().Lookup("project-path"))
	viper.BindPFlag("profiles.load.name", profilesLoadCmd.Flags().Lookup("name"))

	profilesCmd.AddCommand(profilesDeleteCmd)
	profilesDeleteCmd.Flags().String("project-path", "", "Path to the project")
	profilesDeleteCmd.Flags().String("name", "", "Name of the profile to delete")
	viper.BindPFlag("profiles.delete.project-path", profilesDeleteCmd.Flags().Lookup("project-path"))
	viper.BindPFlag("profiles.delete.name", profilesDeleteCmd.Flags().Lookup("name"))
}
