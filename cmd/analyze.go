package cmd

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"code-prompt-core/pkg/database"
	"code-prompt-core/pkg/filter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type TreeNode struct {
	Name      string      `json:"name"`
	Path      string      `json:"path"`
	IsDir     bool        `json:"is_dir"`
	Status    string      `json:"status,omitempty"`
	SizeBytes int64       `json:"size_bytes,omitempty"` // <-- 新增字段
	Children  []*TreeNode `json:"children"`
}

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze the cached data of a project",
	Long:  `The "analyze" command group provides tools to query and generate insights from the cached project data without re-scanning the file system. All analysis commands operate on the existing data in the database, making them very fast.`,
}

var analyzeFilterCmd = &cobra.Command{
	Use:   "filter",
	Short: "Filter, sort, and paginate the cached file metadata",
	Long: `Filters the cached file metadata based on various criteria. 
This command is useful for getting a specific subset of file information from the project.

The primary filtering mechanism is --filter-json, which accepts a JSON string with the following keys:
- "excludedExtensions": An array of strings. To exclude files with no extension, use "no_extension". Example: ["go", "md"]
- "excludedPrefixes": An array of path prefixes to exclude. Example: ["cmd/"]
- "isTextOnly": A boolean that, if true, only includes text files.

Example:
  # Get all text files, excluding .md files and files in the 'vendor/' directory
  code-prompt-core analyze filter --project-path /p/proj --filter-json '{"isTextOnly":true, "excludedExtensions":[".md"], "excludedPrefixes":["vendor/"]}'`,
	Run: func(cmd *cobra.Command, args []string) {
		projectPath := viper.GetString("analyze.filter.project-path")
		if projectPath == "" {
			printError(fmt.Errorf("project-path is required"))
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
		var f filter.Filter
		filterJSON := viper.GetString("analyze.filter.filter-json")
		if filterJSON != "" {
			if err := json.Unmarshal([]byte(filterJSON), &f); err != nil {
				printError(fmt.Errorf("error parsing filter JSON: %w", err))
				return
			}
		}
		paths, err := filter.GetFilteredFilePaths(db, projectID, f)
		if err != nil {
			printError(err)
			return
		}
		if len(paths) == 0 {
			printJSON([]interface{}{})
			return
		}
		query := `
			SELECT relative_path, filename, extension, size_bytes, line_count, is_text 
			FROM file_metadata 
			WHERE project_id = ? AND relative_path IN (?` + strings.Repeat(",?", len(paths)-1) + `)`
		params := []interface{}{projectID}
		for _, p := range paths {
			params = append(params, p)
		}
		rows, err := db.Query(query, params...)
		if err != nil {
			printError(fmt.Errorf("error fetching metadata: %w", err))
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
		for rows.Next() {
			var fileMeta FileMetadata
			if err := rows.Scan(&fileMeta.RelativePath, &fileMeta.Filename, &fileMeta.Extension, &fileMeta.SizeBytes, &fileMeta.LineCount, &fileMeta.IsText); err != nil {
				printError(fmt.Errorf("error scanning file metadata row: %w", err))
				return
			}
			files = append(files, fileMeta)
		}
		printJSON(files)
	},
}

var analyzeStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Generate statistics about the project's cached files",
	Long: `Generates statistical information about the project's current cache.
It groups files by their extension and provides counts, total size, and total lines for each type, as well as overall totals. This command gives a high-level overview of the project's composition.

Example:
  code-prompt-core analyze stats --project-path /path/to/project`,
	Run: func(cmd *cobra.Command, args []string) {
		projectPath := viper.GetString("analyze.stats.project-path")
		if projectPath == "" {
			printError(fmt.Errorf("project-path is required"))
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
		rows, err := db.Query("SELECT extension, COUNT(*), SUM(size_bytes), SUM(line_count) FROM file_metadata WHERE project_id = ? GROUP BY extension", projectID)
		if err != nil {
			printError(fmt.Errorf("error querying file metadata: %w", err))
			return
		}
		defer rows.Close()
		type ExtStats struct {
			FileCount  int   `json:"fileCount"`
			TotalSize  int64 `json:"totalSize"`
			TotalLines int   `json:"totalLines"`
		}
		stats := make(map[string]ExtStats)
		var totalFiles, totalLines int
		var totalSize int64
		for rows.Next() {
			var ext sql.NullString
			var s ExtStats
			if err := rows.Scan(&ext, &s.FileCount, &s.TotalSize, &s.TotalLines); err != nil {
				printError(fmt.Errorf("error scanning row: %w", err))
				return
			}
			extName := "no_extension"
			if ext.Valid && ext.String != "" {
				extName = ext.String
			}
			stats[extName] = s
			totalFiles += s.FileCount
			totalSize += s.TotalSize
			totalLines += s.TotalLines
		}
		printJSON(map[string]interface{}{
			"totalFiles":  totalFiles,
			"totalSize":   totalSize,
			"totalLines":  totalLines,
			"byExtension": stats,
		})
	},
}

var analyzeTreeCmd = &cobra.Command{
	Use:   "tree",
	Short: "Generate a file structure tree from the cache",
	Long: `Generates a file structure tree based on the cached data.

This command can optionally "annotate" the tree, marking which files are included or excluded based on a filter profile or a temporary filter JSON. This is ideal for user interfaces that need to visually represent the file structure and filter results simultaneously.

Output can be a structured JSON (default, for UIs) or plain text (for command-line viewing).

Example (JSON output, annotated):
  code-prompt-core analyze tree --project-path /p/proj --filter-json '{"excludedExtensions":[".md"]}'

Example (Text output):
  code-prompt-core analyze tree --project-path /p/proj --format=text`,
	Run: func(cmd *cobra.Command, args []string) {
		projectPath := viper.GetString("analyze.tree.project-path")
		if projectPath == "" {
			printError(fmt.Errorf("project-path is required"))
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
		var f filter.Filter
		filterJSON := viper.GetString("analyze.tree.filter-json")
		profileName := viper.GetString("analyze.tree.profile-name")
		if profileName != "" {
			var profileData string
			err := db.QueryRow("SELECT profile_data_json FROM profiles WHERE project_id = ? AND profile_name = ?", projectID, profileName).Scan(&profileData)
			if err != nil {
				if err == sql.ErrNoRows {
					printError(fmt.Errorf("profile '%s' not found", profileName))
				} else {
					printError(fmt.Errorf("error loading profile: %w", err))
				}
				return
			}
			json.Unmarshal([]byte(profileData), &f)
		} else if filterJSON != "" {
			if err := json.Unmarshal([]byte(filterJSON), &f); err != nil {
				printError(fmt.Errorf("error parsing filter JSON: %w", err))
				return
			}
		}
		includedPaths, err := filter.GetFilteredFilePaths(db, projectID, f)
		if err != nil {
			printError(fmt.Errorf("error getting filtered file list: %w", err))
			return
		}
		includedSet := make(map[string]struct{}, len(includedPaths))
		for _, path := range includedPaths {
			includedSet[path] = struct{}{}
		}
		rows, err := db.Query("SELECT relative_path FROM file_metadata WHERE project_id = ? ORDER BY relative_path ASC", projectID)
		if err != nil {
			printError(fmt.Errorf("error querying all file metadata for tree: %w", err))
			return
		}
		defer rows.Close()
		root := &TreeNode{Name: filepath.Base(projectPath), Path: ".", IsDir: true}
		nodes := make(map[string]*TreeNode)
		nodes["."] = root
		for rows.Next() {
			var path string
			if err := rows.Scan(&path); err != nil {
				printError(fmt.Errorf("error scanning row: %w", err))
				return
			}
			parts := strings.Split(path, string(filepath.Separator))
			currentPath := ""
			for i, part := range parts {
				isDir := i < len(parts)-1
				if i > 0 {
					currentPath = filepath.Join(currentPath, part)
				} else {
					currentPath = part
				}
				if _, exists := nodes[currentPath]; !exists {
					newNode := &TreeNode{Name: part, Path: currentPath, IsDir: isDir, Children: []*TreeNode{}}
					if !isDir {
						if _, isIncluded := includedSet[currentPath]; isIncluded {
							newNode.Status = "included"
						} else {
							newNode.Status = "excluded"
						}
					}
					parentPath := filepath.Dir(currentPath)
					if parent, ok := nodes[parentPath]; ok {
						parent.Children = append(parent.Children, newNode)
					}
					nodes[currentPath] = newNode
				}
			}
		}
		sortTree(root)
		if viper.GetString("analyze.tree.format") == "text" {
			fmt.Println(root.Name)
			printPlainTextTree(root, "")
		} else {
			printJSON(root)
		}
	},
}

func sortTree(node *TreeNode) {
	if !node.IsDir || len(node.Children) == 0 {
		return
	}
	sort.Slice(node.Children, func(i, j int) bool {
		if node.Children[i].IsDir != node.Children[j].IsDir {
			return node.Children[i].IsDir
		}
		return node.Children[i].Name < node.Children[j].Name
	})
	for _, child := range node.Children {
		sortTree(child)
	}
}

func printPlainTextTree(node *TreeNode, prefix string) {
	for i, child := range node.Children {
		connector := "├── "
		if i == len(node.Children)-1 {
			connector = "└── "
		}
		statusMarker := ""
		if child.Status == "excluded" {
			statusMarker = " [excluded]"
		}
		fmt.Println(prefix + connector + child.Name + statusMarker)
		if child.IsDir {
			newPrefix := prefix
			if i == len(node.Children)-1 {
				newPrefix += "    "
			} else {
				newPrefix += "│   "
			}
			printPlainTextTree(child, newPrefix)
		}
	}
}

func init() {
	rootCmd.AddCommand(analyzeCmd)

	analyzeCmd.AddCommand(analyzeFilterCmd)
	analyzeFilterCmd.Flags().String("project-path", "", "Path to the project")
	analyzeFilterCmd.Flags().String("filter-json", "", "JSON string with filter conditions")
	viper.BindPFlag("analyze.filter.project-path", analyzeFilterCmd.Flags().Lookup("project-path"))
	viper.BindPFlag("analyze.filter.filter-json", analyzeFilterCmd.Flags().Lookup("filter-json"))

	analyzeCmd.AddCommand(analyzeStatsCmd)
	analyzeStatsCmd.Flags().String("project-path", "", "Path to the project")
	viper.BindPFlag("analyze.stats.project-path", analyzeStatsCmd.Flags().Lookup("project-path"))

	analyzeCmd.AddCommand(analyzeTreeCmd)
	analyzeTreeCmd.Flags().String("project-path", "", "Path to the project")
	analyzeTreeCmd.Flags().String("format", "json", "Output format for the tree (json or text)")
	analyzeTreeCmd.Flags().String("profile-name", "", "Name of a saved filter profile to use for annotating the tree")
	analyzeTreeCmd.Flags().String("filter-json", "", "A temporary JSON string with filter conditions")
	viper.BindPFlag("analyze.tree.project-path", analyzeTreeCmd.Flags().Lookup("project-path"))
	viper.BindPFlag("analyze.tree.format", analyzeTreeCmd.Flags().Lookup("format"))
	viper.BindPFlag("analyze.tree.profile-name", analyzeTreeCmd.Flags().Lookup("profile-name"))
	viper.BindPFlag("analyze.tree.filter-json", analyzeTreeCmd.Flags().Lookup("filter-json"))
}
