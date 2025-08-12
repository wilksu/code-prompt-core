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
)

// ----------------------------------------------------------------
// 1. 数据结构修改
// ----------------------------------------------------------------

// TreeNode 定义了文件树中一个节点的结构。
// 这个结构现在包含了 Status 字段，用于UI渲染。
type TreeNode struct {
	// 节点显示的名称（文件名或目录名）
	Name string `json:"name"`
	// 节点相对于项目根的完整路径
	Path string `json:"path"`
	// 标识该节点是否为目录
	IsDir bool `json:"is_dir"`
	// 标识文件节点的过滤状态: "included" (包含) 或 "excluded" (排除)
	// 使用 omitempty 标签，这样目录节点的JSON输出中就不会出现这个字段，保持简洁。
	Status string `json:"status,omitempty"`
	// 该节点的子节点列表
	Children []*TreeNode `json:"children"`
}

// ----------------------------------------------------------------
// 2. 命令定义与实现
// ----------------------------------------------------------------

// analyzeCmd 是 'analyze' 命令组的根命令。
var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze the cached data of a project",
	Long:  `The "analyze" command group provides tools to filter, query, and generate insights from the cached project data without re-scanning the file system.`,
}

// analyzeFilterCmd 的实现已在第一步中被修改为调用公共的 filter 包。
// （此部分代码保持第一步修改后的状态即可，这里为了文件完整性而包含）
var analyzeFilterCmd = &cobra.Command{
	Use:   "filter",
	Short: "Filter, sort, and paginate the cached file metadata",
	Long: `Filters the cached file metadata based on various criteria provided via flags.

The main filtering mechanism is --filter-json, which accepts a JSON string with the following keys:
- "excludedExtensions": An array of strings. To exclude files with no extension, use the special string "no_extension". Example: ["go", "md", "no_extension"]
- "excludedPrefixes": An array of strings representing path prefixes to exclude. Example: ["cmd/"]
- "isTextOnly": A boolean that, if true, only includes text files.

Sorting and pagination are supported via dedicated flags.

Example Usage:
  # Get the 50 largest files, sorted by size descending
  ./code-prompt-core.exe analyze filter --db my.db --project-path /p/project --sort-by size_bytes --sort-order desc --limit 50

  # Get all text files, excluding .md files and files in the 'vendor/' directory
  ./code-prompt-core.exe analyze filter --db my.db --project-path /p/project --filter-json '{"isTextOnly":true, "excludedExtensions":[".md"], "excludedPrefixes":["vendor/"]}'
`,
	Run: func(cmd *cobra.Command, args []string) {
		dbPath, _ := cmd.Flags().GetString("db")
		projectPath, _ := cmd.Flags().GetString("project-path")
		filterJSON, _ := cmd.Flags().GetString("filter-json")

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

		var f filter.Filter
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

		// 此处逻辑是为了获取过滤后文件的完整元数据并返回，保持了 filter 命令的原有功能
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
			printError(fmt.Errorf("error fetching full metadata for filtered files: %w", err))
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

// analyzeStatsCmd 保持不变
var analyzeStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Generate statistics about the project's cached files",
	Long: `Generates statistical information about the project's current cache.
It groups files by their extension and provides counts, total size, and total lines for each type, as well as overall totals.

Example Usage:
  ./code-prompt-core.exe analyze stats --db my.db --project-path /path/to/project
`,
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
			FileCount  int   `json:"fileCount"`
			TotalSize  int64 `json:"totalSize"`
			TotalLines int   `json:"totalLines"`
		}

		stats := make(map[string]ExtStats)
		var totalFiles int
		var totalSize int64
		var totalLines int

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

		output := map[string]interface{}{
			"totalFiles":  totalFiles,
			"totalSize":   totalSize,
			"totalLines":  totalLines,
			"byExtension": stats,
		}

		printJSON(output)
	},
}

// analyzeTreeCmd 是本次修改的核心
var analyzeTreeCmd = &cobra.Command{
	Use:   "tree",
	Short: "Generate a file structure tree from the cache",
	Long: `Generates a file structure tree based on the cached data, optionally annotating nodes based on filter criteria.
The default output is a structured JSON object, ideal for UIs. A plain text format is also available.

Example (JSON output, annotated):
  ./code-prompt-core.exe analyze tree --db my.db --project-path /p/proj --filter-json '{"excludedExtensions":[".md"]}'

Example (Text output, annotated):
  ./code-prompt-core.exe analyze tree --db my.db --project-path /p/proj --filter-json '{"excludedExtensions":[".md"]}' --format=text
`,
	Run: func(cmd *cobra.Command, args []string) {
		// --- 获取所有参数 ---
		dbPath, _ := cmd.Flags().GetString("db")
		projectPath, _ := cmd.Flags().GetString("project-path")
		format, _ := cmd.Flags().GetString("format")
		profileName, _ := cmd.Flags().GetString("profile-name")
		filterJSON, _ := cmd.Flags().GetString("filter-json")

		// --- 数据库和项目ID准备 ---
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

		// --- 步骤 1: 获取并解析过滤规则 ---
		// 即使没有提供过滤参数，我们也会创建一个空的 Filter 对象，
		// 这意味着默认情况下所有文件都会被视为 "included"。
		var f filter.Filter
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

		// --- 步骤 2: 生成“包含”文件的路径集合，用于后续O(1)快速查找 ---
		includedPaths, err := filter.GetFilteredFilePaths(db, projectID, f)
		if err != nil {
			printError(fmt.Errorf("error getting filtered file list: %w", err))
			return
		}
		includedSet := make(map[string]struct{}, len(includedPaths))
		for _, path := range includedPaths {
			includedSet[path] = struct{}{}
		}

		// --- 步骤 3: 获取所有文件路径，用于构建完整的树结构 ---
		rows, err := db.Query("SELECT relative_path FROM file_metadata WHERE project_id = ? ORDER BY relative_path ASC", projectID)
		if err != nil {
			printError(fmt.Errorf("error querying all file metadata for tree: %w", err))
			return
		}
		defer rows.Close()

		// --- 步骤 4: 构建树，并对每个文件节点进行“注解” ---
		root := &TreeNode{
			Name:  filepath.Base(projectPath),
			Path:  ".",
			IsDir: true,
		}
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

				// 修复路径拼接问题
				if i > 0 {
					currentPath = filepath.Join(currentPath, part)
				} else {
					currentPath = part
				}

				if _, exists := nodes[currentPath]; !exists {
					newNode := &TreeNode{
						Name:     part,
						Path:     currentPath,
						IsDir:    isDir,
						Children: []*TreeNode{},
					}

					// --- 这是核心的注解逻辑 ---
					if !isDir { // 只对文件节点设置状态
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

		// --- 步骤 5: 根据格式要求输出结果 ---
		if format == "text" {
			fmt.Println(root.Name)
			printPlainTextTree(root, "")
		} else {
			printJSON(root)
		}
	},
}

// sortTree 递归地对树节点进行排序，目录在前，文件在后，同类型按名称排序。
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

// ----------------------------------------------------------------
// 3. 文本输出函数修改
// ----------------------------------------------------------------

// printPlainTextTree 打印纯文本格式的树，现在会为被排除的文件添加标记。
func printPlainTextTree(node *TreeNode, prefix string) {
	for i, child := range node.Children {
		connector := "├── "
		if i == len(node.Children)-1 {
			connector = "└── "
		}

		// --- 这是核心的文本标记逻辑 ---
		statusMarker := ""
		if child.Status == "excluded" {
			statusMarker = " [excluded]"
		}

		fmt.Println(prefix + connector + child.Name + statusMarker)

		if child.IsDir {
			newPrefix := prefix
			if i == len(node.Children)-1 {
				newPrefix += "    " // 4个空格
			} else {
				newPrefix += "│   " // 竖线和3个空格
			}
			printPlainTextTree(child, newPrefix)
		}
	}
}

// init 函数负责将所有命令和参数注册到Cobra框架中。
func init() {
	rootCmd.AddCommand(analyzeCmd)
	analyzeCmd.AddCommand(analyzeFilterCmd)
	analyzeFilterCmd.Flags().String("db", "", "Path to the database file")
	analyzeFilterCmd.MarkFlagRequired("db")
	analyzeFilterCmd.Flags().String("project-path", "", "Path to the project")
	analyzeFilterCmd.MarkFlagRequired("project-path")
	analyzeFilterCmd.Flags().String("filter-json", "", "JSON string with filter conditions")
	analyzeFilterCmd.Flags().String("sort-by", "relative_path", "Column to sort by (relative_path, filename, size_bytes, line_count)")
	analyzeFilterCmd.Flags().String("sort-order", "asc", "Sort order (asc or desc)")
	analyzeFilterCmd.Flags().Int("limit", -1, "Limit the number of results (-1 for no limit)")
	analyzeFilterCmd.Flags().Int("offset", 0, "Offset for pagination")

	analyzeCmd.AddCommand(analyzeStatsCmd)
	analyzeStatsCmd.Flags().String("db", "", "Path to the database file")
	analyzeStatsCmd.MarkFlagRequired("db")
	analyzeStatsCmd.Flags().String("project-path", "", "Path to the project")
	analyzeStatsCmd.MarkFlagRequired("project-path")

	// --- analyzeTreeCmd 参数注册 ---
	analyzeCmd.AddCommand(analyzeTreeCmd)
	analyzeTreeCmd.Flags().String("db", "", "Path to the database file")
	analyzeTreeCmd.MarkFlagRequired("db")
	analyzeTreeCmd.Flags().String("project-path", "", "Path to the project")
	analyzeTreeCmd.MarkFlagRequired("project-path")
	analyzeTreeCmd.Flags().String("format", "json", "Output format for the tree (json or text)")
	// --- 新增的过滤参数 ---
	analyzeTreeCmd.Flags().String("profile-name", "", "Name of a saved filter profile to use for annotating the tree")
	analyzeTreeCmd.Flags().String("filter-json", "", "A temporary JSON string with filter conditions to use for annotating the tree")
}
