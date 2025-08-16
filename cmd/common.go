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
	projectPath := viper.GetString(viperKey)
	if projectPath == "" {
		return "", fmt.Errorf("project-path is required (viper key: %s)", viperKey)
	}
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return "", fmt.Errorf("error resolving absolute path for '%s': %w", projectPath, err)
	}
	return absPath, nil
}

// getFilter 是一个新的帮助函数，用于从 profile 或 JSON 字符串构建 Filter 对象
// 它集中处理加载、解析和编译过滤规则的逻辑
func getFilter(db *sql.DB, projectID int64, profileName, filterJSON string) (filter.Filter, error) {
	var f filter.Filter
	var finalFilterJSON string

	if filterJSON != "" {
		// 优先使用直接传入的 filter-json
		finalFilterJSON = filterJSON
	} else if profileName != "" {
		// 其次，从 profile 加载
		err := db.QueryRow("SELECT profile_data_json FROM profiles WHERE project_id = ? AND profile_name = ?", projectID, profileName).Scan(&finalFilterJSON)
		if err != nil {
			if err == sql.ErrNoRows {
				return f, fmt.Errorf("profile '%s' not found for this project", profileName)
			}
			return f, fmt.Errorf("error loading profile: %w", err)
		}
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

	// 在返回之前，编译所有规则
	if err := f.Compile(); err != nil {
		return f, fmt.Errorf("error compiling filter rules: %w", err)
	}

	return f, nil
}
