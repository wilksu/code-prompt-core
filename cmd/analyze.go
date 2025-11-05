// File: cmd/analyze.go
package cmd

import (
	"database/sql"
	"fmt"
	"path" // *** 关键修改点1：引入 "path" 包 ***
	"path/filepath"
	"sort"
	"strings"

	"code-prompt-core/pkg/database"
	"code-prompt-core/pkg/filter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type TreeNode struct {
	Name           string      `json:"name"`
	Path           string      `json:"path"`
	IsDir          bool        `json:"is_dir"`
	Status         string      `json:"status,omitempty"`
	SizeBytes      int64       `json:"size_bytes,omitempty"`       // 用于文件
	TotalSizeBytes int64       `json:"total_size_bytes,omitempty"` // 用于目录
	TotalFileCount int         `json:"total_file_count,omitempty"` // 用于目录
	Children       []*TreeNode `json:"children"`
}

// calculateTreeAggregates 是一个新函数，用于递归计算目录的大小和文件数
// 它从叶节点（文件）向上聚合到根节点。
func calculateTreeAggregates(node *TreeNode) (size int64, count int) {
	if !node.IsDir {
		// 如果是文件，返回它自己的大小和 1 个计数
		return node.SizeBytes, 1
	}

	var totalSize int64
	var totalCount int

	// 遍历所有子节点
	for _, child := range node.Children {
		// 递归调用
		childSize, childCount := calculateTreeAggregates(child)
		totalSize += childSize
		totalCount += childCount
	}

	// 将聚合结果存回目录节点
	node.TotalSizeBytes = totalSize
	node.TotalFileCount = totalCount
	return totalSize, totalCount
}

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze the cached data of a project",
	Long:  `The "analyze" command group provides tools to query and generate insights from the cached project data without re-scanning the file system. All analysis commands operate on the existing data in the database, making them very fast.`,
}

var analyzeFilterCmd = &cobra.Command{
	Use:   "filter",
	Short: "Filter cached file metadata using JSON or a saved profile",
	Long: `Filters the cached file metadata based on various criteria provided as a JSON string.

The filter JSON supports both simple and advanced rules:
{
  "includeExts": ["go", "md"],
  "excludePaths": ["vendor/"],
  "includeRegex": ["^cmd/"],
  "priority": "includes"
}

Example:
  code-prompt-core analyze filter --project-path /p/proj --filter-json '{"includeExts":[".go"]}'`,
	Run: func(cmd *cobra.Command, args []string) {
		absProjectPath, err := getAbsoluteProjectPath("analyze.filter.project-path")
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
			printError(fmt.Errorf("error finding project: %w", err))
			return
		}

		f, err := getFilter(
			db,
			projectID,
			viper.GetString("analyze.filter.profile-name"),
			viper.GetString("analyze.filter.filter-json"),
		)
		if err != nil {
			printError(err)
			return
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

// analyzeSummaryCmd 是一个新命令，用于获取过滤后的摘要信息
var analyzeSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Get a metadata summary (total size, count, file list) for a given filter",
	Long: `Analyzes files matching a filter and returns a JSON summary.

This command is a high-performance way to 'preview' a filter.
It calculates the total file count, total size, and returns the full metadata
list for all matching files without reading their content.

This is ideal for an orchestration layer (like your MCP) to decide if a file set is
too large for a subsequent 'content get' operation before calling the LLM.

Example:
  code-prompt-core analyze summary --project-path /p/proj --filter-json '{"includeExts":[".go"]}'`,
	Run: func(cmd *cobra.Command, args []string) {
		absProjectPath, err := getAbsoluteProjectPath("analyze.summary.project-path")
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
			printError(fmt.Errorf("error finding project: %w", err))
			return
		}

		f, err := getFilter(
			db,
			projectID,
			viper.GetString("analyze.summary.profile-name"),
			viper.GetString("analyze.summary.filter-json"),
		)
		if err != nil {
			printError(err)
			return
		}

		paths, err := filter.GetFilteredFilePaths(db, projectID, f)
		if err != nil {
			printError(err)
			return
		}

		if len(paths) == 0 {
			printJSON(map[string]interface{}{
				"fileCount":      0,
				"totalSizeBytes": 0,
				"files":          []interface{}{},
			})
			return
		}

		// 这个查询与 analyze filter 相同
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

		// (这是新部分：聚合)
		type FileMetadata struct {
			RelativePath string `json:"relative_path"`
			Filename     string `json:"filename"`
			Extension    string `json:"extension"`
			SizeBytes    int64  `json:"size_bytes"`
			LineCount    int    `json:"line_count"`
			IsText       bool   `json:"is_text"`
		}
		var files []FileMetadata
		var totalSize int64

		for rows.Next() {
			var fileMeta FileMetadata
			if err := rows.Scan(&fileMeta.RelativePath, &fileMeta.Filename, &fileMeta.Extension, &fileMeta.SizeBytes, &fileMeta.LineCount, &fileMeta.IsText); err != nil {
				printError(fmt.Errorf("error scanning file metadata row: %w", err))
				return
			}
			files = append(files, fileMeta)
			totalSize += fileMeta.SizeBytes // 聚合大小
		}

		// (这是新的摘要对象)
		printJSON(map[string]interface{}{
			"fileCount":      len(files),
			"totalSizeBytes": totalSize,
			"files":          files,
		})
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
		projectPath, err := getAbsoluteProjectPath("analyze.stats.project-path")
		if err != nil {
			printError(err)
			return
		}
		absProjectPath, err := filepath.Abs(projectPath)
		if err != nil {
			printError(fmt.Errorf("error resolving absolute path for '%s': %w", projectPath, err))
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
	Long: `Generates a file structure tree, optionally annotating it based on a filter.
This command now also recursively calculates the total size and file count for each directory.

The filter (from --filter-json or --profile-name) determines which files are marked as "included".
The filter JSON supports both simple and advanced rules:
{
  "excludeExts": ["md"],
  "includePaths": ["cmd/"]
}

Example (JSON output, annotated):
  code-prompt-core analyze tree --project-path /p/proj --filter-json '{"excludeExts":["md"]}'`,
	Run: func(cmd *cobra.Command, args []string) {
		absProjectPath, err := getAbsoluteProjectPath("analyze.tree.project-path")
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
			printError(fmt.Errorf("error finding project: %w", err))
			return
		}

		f, err := getFilter(
			db,
			projectID,
			viper.GetString("analyze.tree.profile-name"),
			viper.GetString("analyze.tree.filter-json"),
		)
		if err != nil {
			printError(err)
			return
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
		// *** 修改：现在也查询 size_bytes ***
		rows, err := db.Query("SELECT relative_path, size_bytes FROM file_metadata WHERE project_id = ? ORDER BY relative_path ASC", projectID)
		if err != nil {
			printError(fmt.Errorf("error querying all file metadata for tree: %w", err))
			return
		}
		defer rows.Close()

		// `filepath.Base` is safe here as it operates on the project's real path on disk
		root := &TreeNode{Name: filepath.Base(absProjectPath), Path: ".", IsDir: true}
		nodes := make(map[string]*TreeNode)
		nodes["."] = root

		for rows.Next() {
			var dbPath string
			var size int64 // *** 修改：接收 size_bytes ***
			if err := rows.Scan(&dbPath, &size); err != nil {
				printError(fmt.Errorf("error scanning row: %w", err))
				return
			}

			// *** 关键修改点2：总是使用'/'来分割从数据库读出的路径 ***
			parts := strings.Split(dbPath, "/")
			currentPath := ""

			for i, part := range parts {
				isDir := i < len(parts)-1
				if i > 0 {
					// *** 关键修改点3：使用 path.Join 来构建标准化的路径 ***
					currentPath = path.Join(currentPath, part)
				} else {
					currentPath = part
				}

				if _, exists := nodes[currentPath]; !exists {
					newNode := &TreeNode{Name: part, Path: currentPath, IsDir: isDir, Children: []*TreeNode{}}
					if !isDir {
						// *** 修改：仅在文件节点上设置 SizeBytes ***
						newNode.SizeBytes = size
						if _, isIncluded := includedSet[currentPath]; isIncluded {
							newNode.Status = "included"
						} else {
							newNode.Status = "excluded"
						}
					}

					// *** 关键修改点4：使用 path.Dir 来查找父路径 ***
					parentPath := path.Dir(currentPath)
					if parent, ok := nodes[parentPath]; ok {
						parent.Children = append(parent.Children, newNode)
					}
					nodes[currentPath] = newNode
				}
			}
		}

		// *** 新增：在排序前调用聚合函数 ***
		calculateTreeAggregates(root)

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
		// *** 修改：在文本树中也显示大小信息 ***
		sizeInfo := ""
		if child.IsDir {
			sizeInfo = fmt.Sprintf(" (%d files, %d bytes)", child.TotalFileCount, child.TotalSizeBytes)
		} else {
			sizeInfo = fmt.Sprintf(" (%d bytes)", child.SizeBytes)
		}

		fmt.Println(prefix + connector + child.Name + sizeInfo + statusMarker)

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
	analyzeFilterCmd.Flags().String("profile-name", "", "Name of a saved filter profile to use") // 新增
	viper.BindPFlag("analyze.filter.project-path", analyzeFilterCmd.Flags().Lookup("project-path"))
	viper.BindPFlag("analyze.filter.filter-json", analyzeFilterCmd.Flags().Lookup("filter-json"))
	viper.BindPFlag("analyze.filter.profile-name", analyzeFilterCmd.Flags().Lookup("profile-name")) // 新增

	// *** 新增：注册 analyze summary 命令 ***
	analyzeCmd.AddCommand(analyzeSummaryCmd)
	analyzeSummaryCmd.Flags().String("project-path", "", "Path to the project")
	analyzeSummaryCmd.Flags().String("profile-name", "", "Name of a saved filter profile to use")
	analyzeSummaryCmd.Flags().String("filter-json", "", "A temporary JSON string with filter conditions")
	viper.BindPFlag("analyze.summary.project-path", analyzeSummaryCmd.Flags().Lookup("project-path"))
	viper.BindPFlag("analyze.summary.profile-name", analyzeSummaryCmd.Flags().Lookup("profile-name"))
	viper.BindPFlag("analyze.summary.filter-json", analyzeSummaryCmd.Flags().Lookup("filter-json"))

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
