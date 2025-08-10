package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"code-prompt-core/pkg/database"

	"github.com/spf13/cobra"
)

type Filter struct {
	ExcludedExtensions []string `json:"excludedExtensions"`
	ExcludedPrefixes   []string `json:"excludedPrefixes"`
	IsTextOnly         bool     `json:"isTextOnly"`
}

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze the cached data",
}

var analyzeFilterCmd = &cobra.Command{
	Use:   "filter",
	Short: "Filter the cached file metadata",
	Long: `Filter the cached file metadata based on a JSON object provided to the --filter-json flag.

The JSON object can contain the following keys:
- "excludedExtensions": An array of strings representing file extensions to exclude (e.g., [".go", ".md"]).
- "excludedPrefixes": An array of strings representing path prefixes to exclude.
- "isTextOnly": A boolean value that, if true, only includes text files in the output.

Example:
--filter-json '{"excludedExtensions":[".log"], "isTextOnly":true}'`,
	Run: func(cmd *cobra.Command, args []string) {
		dbPath, _ := cmd.Flags().GetString("db")
		projectPath, _ := cmd.Flags().GetString("project-path")
		filterJSON, _ := cmd.Flags().GetString("filter-json")

		var filter Filter
		if filterJSON != "" {
			if err := json.Unmarshal([]byte(filterJSON), &filter); err != nil {
				printError(fmt.Errorf("error parsing filter JSON: %w", err))
				return
			}
		}

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

		rows, err := db.Query("SELECT relative_path, filename, extension, size_bytes, line_count, is_text FROM file_metadata WHERE project_id = ?", projectID)
		if err != nil {
			printError(fmt.Errorf("error querying file metadata: %w", err))
			return
		}
		defer rows.Close()

		type FileMetadata struct {
			RelativePath string `json:"relative_path"`
			Filename     string `json:"filename"`
			Extension    string `json:"extension"`
			SizeBytes    int64  `json:"size_bytes"`
			LineCount    int    `json:"line_count"`
			IsText       bool   `json:"is_text"`
		}

		var files []FileMetadata
	FileLoop: for rows.Next() {
			var f FileMetadata
			if err := rows.Scan(&f.RelativePath, &f.Filename, &f.Extension, &f.SizeBytes, &f.LineCount, &f.IsText); err != nil {
				printError(fmt.Errorf("error scanning row: %w", err))
				return
			}

			// Apply filters
			if filter.IsTextOnly && !f.IsText {
				continue
			}

			for _, ext := range filter.ExcludedExtensions {
				if strings.HasSuffix(f.Filename, ext) {
					continue FileLoop
				}
			}

			for _, prefix := range filter.ExcludedPrefixes {
				if strings.HasPrefix(f.RelativePath, prefix) {
					continue FileLoop
				}
			}

			files = append(files, f)
		}

		printJSON(files)
	},
}

var analyzeStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Generate statistics about the project",
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

		rows, err := db.Query("SELECT extension, COUNT(*), SUM(size_bytes), SUM(line_count) FROM file_metadata WHERE project_id = ? GROUP BY extension", projectID)
		if err != nil {
			printError(fmt.Errorf("error querying file metadata: %w", err))
			return
		}
		defer rows.Close()

		type ExtStats struct {
			FileCount int   `json:"fileCount"`
			TotalSize int64 `json:"totalSize"`
			TotalLines int  `json:"totalLines"`
		}

		stats := make(map[string]ExtStats)
		var totalFiles int
		var totalSize int64
		var totalLines int

		for rows.Next() {
			var ext string
			var s ExtStats
			if err := rows.Scan(&ext, &s.FileCount, &s.TotalSize, &s.TotalLines); err != nil {
				printError(fmt.Errorf("error scanning row: %w", err))
				return
			}
			stats[ext] = s
			totalFiles += s.FileCount
			totalSize += s.TotalSize
			totalLines += s.TotalLines
		}

		output := map[string]interface{}{
			"totalFiles": totalFiles,
			"totalSize":  totalSize,
			"totalLines": totalLines,
			"byExtension": stats,
		}

		printJSON(output)
	},
}

var analyzeTreeCmd = &cobra.Command{
	Use:   "tree",
	Short: "Generate a file structure tree",
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

		rows, err := db.Query("SELECT relative_path FROM file_metadata WHERE project_id = ?", projectID)
		if err != nil {
			printError(fmt.Errorf("error querying file metadata: %w", err))
			return
		}
		defer rows.Close()

		tree := make(map[string]interface{})
		for rows.Next() {
			var path string
			if err := rows.Scan(&path); err != nil {
				printError(fmt.Errorf("error scanning row: %w", err))
				return
			}
			parts := strings.Split(path, string(filepath.Separator))
			curr := tree
			for i, part := range parts {
				if i == len(parts)-1 {
					curr[part] = nil
				} else {
					if _, ok := curr[part]; !ok {
						curr[part] = make(map[string]interface{})
					}
					curr = curr[part].(map[string]interface{})
				}
			}
		}

		// For the tree command, we print directly to stdout without JSON wrapping
		printTree(tree, "")
	},
}

func printTree(tree map[string]interface{}, prefix string) {
	// Get sorted keys
	keys := make([]string, 0, len(tree))
	for k := range tree {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for i, key := range keys {
		value := tree[key]
		connector := "├── "
		if i == len(keys)-1 {
			connector = "└── "
		}

		fmt.Println(prefix + connector + key)

		if value != nil {
			newPrefix := prefix
			if i == len(keys)-1 {
				newPrefix += "    "
			} else {
				newPrefix += "│   "
			}
			printTree(value.(map[string]interface{}), newPrefix)
		}
	}
}

func init() {
	rootCmd.AddCommand(analyzeCmd)
	analyzeCmd.AddCommand(analyzeFilterCmd)
	analyzeFilterCmd.Flags().String("db", "", "Path to the database file")
	analyzeFilterCmd.MarkFlagRequired("db")
	analyzeFilterCmd.Flags().String("project-path", "", "Path to the project")
	analyzeFilterCmd.MarkFlagRequired("project-path")
	analyzeFilterCmd.Flags().String("filter-json", "", "JSON string with filter conditions")

	analyzeCmd.AddCommand(analyzeStatsCmd)
	analyzeStatsCmd.Flags().String("db", "", "Path to the database file")
	analyzeStatsCmd.MarkFlagRequired("db")
	analyzeStatsCmd.Flags().String("project-path", "", "Path to the project")
	analyzeStatsCmd.MarkFlagRequired("project-path")

	analyzeCmd.AddCommand(analyzeTreeCmd)
	analyzeTreeCmd.Flags().String("db", "", "Path to the database file")
	analyzeTreeCmd.MarkFlagRequired("db")
	analyzeTreeCmd.Flags().String("project-path", "", "Path to the project")
	analyzeTreeCmd.MarkFlagRequired("project-path")
}
