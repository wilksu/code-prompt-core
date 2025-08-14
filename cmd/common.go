// File: cmd/common.go
package cmd

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"code-prompt-core/pkg/filter"

	"github.com/spf13/viper"
)

// --- Response structs are unchanged ---
type Response struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data,omitempty"`
}

type ErrorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
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
	resp := ErrorResponse{Status: "error", Message: err.Error()}
	bytes, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Fprintln(os.Stderr, string(bytes))
	os.Exit(1)
}

func getAbsoluteProjectPath(viperKey string) (string, error) {
	// CORRECTED: Get the string from viper, not by calling itself.
	projectPath := viper.GetString(viperKey)
	if projectPath == "" {
		// The error message was also incorrect, it should indicate the path is required.
		return "", fmt.Errorf("project-path is required (viper key: %s)", viperKey)
	}
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return "", fmt.Errorf("error resolving absolute path for '%s': %w", projectPath, err)
	}
	return absPath, nil
}

// getFilter is a new helper to build a filter.Filter object from command flags.
// It centralizes the logic of loading from a profile or a direct JSON string.
func getFilter(db *sql.DB, projectID int64, profileName, filterJSON string) (filter.Filter, error) {
	var f filter.Filter
	var finalFilterJSON string

	if profileName != "" {
		// Load from profile
		err := db.QueryRow("SELECT profile_data_json FROM profiles WHERE project_id = ? AND profile_name = ?", projectID, profileName).Scan(&finalFilterJSON)
		if err != nil {
			if err == sql.ErrNoRows {
				return f, fmt.Errorf("profile '%s' not found for this project", profileName)
			}
			return f, fmt.Errorf("error loading profile: %w", err)
		}
	} else if filterJSON != "" {
		// Use direct JSON string
		finalFilterJSON = filterJSON
	}

	if finalFilterJSON != "" {
		if err := json.Unmarshal([]byte(finalFilterJSON), &f); err != nil {
			return f, fmt.Errorf("error parsing filter JSON: %w", err)
		}
	}

	// Set default priority if not specified
	if f.Priority == "" {
		f.Priority = "includes"
	}

	return f, nil
}
