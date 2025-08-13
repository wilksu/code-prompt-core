package cmd

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"code-prompt-core/pkg/database"
	"code-prompt-core/pkg/filter"
	"code-prompt-core/templates"

	"github.com/aymerick/raymond"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate reports from project data",
	Long:  `The "report" command group provides tools to generate rich, user-defined reports by combining various data points (stats, tree, file content) with Handlebars templates.`,
}

var reportListTemplatesCmd = &cobra.Command{
	Use:   "list-templates",
	Short: "List all available built-in report templates",
	Long: `Displays a list of all built-in templates that were compiled into the application, in JSON format.
These template names can be used directly with the '--template' flag of the 'report generate' command.

The returned JSON format is as follows:
{
  "status": "success",
  "data": [
    {
      "name": "default-md",
      "description": "A comprehensive report in Markdown format."
    },
    {
      "name": "detailed-prompt",
      "description": "A detailed snapshot suitable for LLM context."
    },
    {
      "name": "summary-txt",
      "description": "A concise summary in plain text format."
    }
  ]
}`,
	Run: func(cmd *cobra.Command, args []string) {
		// MODIFIED: Changed output to JSON for UI integration.
		type templateListOutput struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}

		output := make([]templateListOutput, 0, len(templates.BuiltInTemplates))
		for _, t := range templates.BuiltInTemplates {
			output = append(output, templateListOutput{
				Name:        t.Name,
				Description: t.Description,
			})
		}

		printJSON(output)
	},
}

var reportGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a report from a template",
	Long: `This command aggregates project statistics, file structure, and file contents, then uses a Handlebars template to generate a final report file.

You can use a built-in template by name, or provide a path to a custom .hbs file.
Run 'code-prompt-core report list-templates' to see all available built-in templates.

Example (using a built-in template):
  code-prompt-core report generate --template default-md --output report.md`,
	Run: func(cmd *cobra.Command, args []string) {
		projectPath := viper.GetString("report.generate.project-path")
		templateIdentifier := viper.GetString("report.generate.template")
		outputPath := viper.GetString("report.generate.output")
		if projectPath == "" || templateIdentifier == "" || outputPath == "" {
			printError(fmt.Errorf("--project-path, --template, and --output are required"))
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
		raymond.RegisterHelper("humanizeBytes", func(bytes int64) string {
			return humanize.Bytes(uint64(bytes))
		})
		templateContent, err := getTemplateContent(templateIdentifier)
		if err != nil {
			printError(err)
			return
		}
		if strings.Contains(templateContent, "{{> treePartial") {
			treePartial := `{{#each nodes}}{{this.indent}}├── {{this.Name}} {{#if this.SizeBytes}}({{humanizeBytes this.SizeBytes}}){{/if}}{{#if this.isDir}}/{{/if}}
{{#if this.Children}}{{> treePartial nodes=this.Children indent=(append this.indent "    ")}}{{/if}}{{/each}}`
			raymond.RegisterPartial("treePartial", treePartial)
			raymond.RegisterHelper("append", func(base, addition string) string {
				return base + addition
			})
		}
		reportCtx, err := buildReportContext(db, projectID, projectPath, viper.GetString("report.generate.profile-name"), viper.GetString("report.generate.filter-json"))
		if err != nil {
			printError(fmt.Errorf("error building report context: %w", err))
			return
		}
		result, err := raymond.Render(templateContent, reportCtx)
		if err != nil {
			printError(fmt.Errorf("error rendering template: %w", err))
			return
		}
		err = os.WriteFile(outputPath, []byte(result), 0644)
		if err != nil {
			printError(fmt.Errorf("error writing output file '%s': %w", outputPath, err))
			return
		}
		printJSON(map[string]string{
			"message":    "Report generated successfully",
			"outputPath": outputPath,
		})
	},
}

// getTemplateContent resolves the template. It first checks the dynamically loaded built-in templates,
// then falls back to treating the identifier as a file path.
func getTemplateContent(identifier string) (string, error) {
	// 3. 使用 templates 包的数据来查找和读取内置模板
	for _, t := range templates.BuiltInTemplates {
		if t.Name == identifier {
			contentBytes, err := templates.FS.ReadFile(t.FileName)
			if err != nil {
				return "", fmt.Errorf("error reading embedded template '%s': %w", identifier, err)
			}
			return string(contentBytes), nil
		}
	}
	if _, statErr := os.Stat(identifier); statErr != nil {
		return "", fmt.Errorf("template '%s' not found as a built-in template or as a local file", identifier)
	}
	contentBytes, err := os.ReadFile(identifier)
	if err != nil {
		return "", fmt.Errorf("error reading local template file '%s': %w", identifier, err)
	}
	return string(contentBytes), nil
}

// buildReportContext prepares all data needed for template rendering.
func buildReportContext(db *sql.DB, projectID int64, projectPath, profileName, filterJSON string) (map[string]interface{}, error) {
	f, err := getFilterConfig(db, projectID, profileName, filterJSON)
	if err != nil {
		return nil, err
	}

	stats, err := getStatsData(db, projectID, f)
	if err != nil {
		return nil, err
	}

	tree, err := getTreeData(db, projectID, projectPath)
	if err != nil {
		return nil, err
	}

	contents, err := getContentsData(db, projectID, projectPath, f)
	if err != nil {
		return nil, err
	}

	absProjectPath, _ := filepath.Abs(projectPath)

	ctx := map[string]interface{}{
		"project_path":       projectPath,
		"absolute_code_path": absProjectPath,
		"generated_at":       time.Now().Format(time.RFC1123),
		"config":             f,
		"stats":              stats,
		"tree":               tree,
		"files":              contents,
	}
	return ctx, nil
}

// TemplateStat is a richer struct for the stats template
type TemplateStat struct {
	ExtName    string `json:"extName"`
	FileCount  int    `json:"fileCount"`
	TotalSize  int64  `json:"totalSize"`
	TotalLines int    `json:"totalLines"`
	IsIncluded bool   `json:"isIncluded"`
}

// getStatsData prepares statistics for the report.
func getStatsData(db *sql.DB, projectID int64, f filter.Filter) (map[string]interface{}, error) {
	rows, err := db.Query("SELECT extension, COUNT(*), SUM(size_bytes), SUM(line_count) FROM file_metadata WHERE project_id = ? GROUP BY extension", projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var statsList []TemplateStat
	var totalFiles, totalLines int
	var totalSize int64

	excludedExts := make(map[string]struct{})
	for _, ext := range f.ExcludedExtensions {
		excludedExts[strings.TrimPrefix(ext, ".")] = struct{}{}
	}

	for rows.Next() {
		var ext sql.NullString
		var s TemplateStat
		if err := rows.Scan(&ext, &s.FileCount, &s.TotalSize, &s.TotalLines); err != nil {
			return nil, err
		}

		s.ExtName = "no_extension"
		if ext.Valid && ext.String != "" {
			s.ExtName = ext.String
		}

		if _, excluded := excludedExts[s.ExtName]; excluded {
			s.IsIncluded = false
		} else {
			s.IsIncluded = true
		}

		statsList = append(statsList, s)
		totalFiles += s.FileCount
		totalSize += s.TotalSize
		totalLines += s.TotalLines
	}

	sort.Slice(statsList, func(i, j int) bool {
		return statsList[i].TotalSize > statsList[j].TotalSize
	})

	return map[string]interface{}{
		"totalFiles":  totalFiles,
		"totalSize":   totalSize,
		"totalLines":  totalLines,
		"byExtension": statsList,
	}, nil
}

// getTreeData prepares the file tree structure for the report.
func getTreeData(db *sql.DB, projectID int64, projectPath string) (*TreeNode, error) {
	rows, err := db.Query("SELECT relative_path, size_bytes FROM file_metadata WHERE project_id = ? ORDER BY relative_path ASC", projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	root := &TreeNode{Name: filepath.Base(projectPath), IsDir: true}
	nodes := make(map[string]*TreeNode)
	nodes["."] = root

	for rows.Next() {
		var path string
		var size int64
		if err := rows.Scan(&path, &size); err != nil {
			return nil, err
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
					newNode.SizeBytes = size
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
	return root, nil
}

// getContentsData prepares the file contents for the report.
func getContentsData(db *sql.DB, projectID int64, projectPath string, f filter.Filter) (map[string]string, error) {
	relativePaths, err := filter.GetFilteredFilePaths(db, projectID, f)
	if err != nil {
		return nil, err
	}
	contentMap := make(map[string]string)
	for _, relPath := range relativePaths {
		fullPath := filepath.Join(projectPath, filepath.Clean(relPath))
		content, err := os.ReadFile(fullPath)
		if err != nil {
			contentMap[relPath] = fmt.Sprintf("Error: Unable to read file. %v", err)
		} else {
			contentMap[relPath] = string(content)
		}
	}
	return contentMap, nil
}

// getFilterConfig prepares the filter configuration for the report.
func getFilterConfig(db *sql.DB, projectID int64, profileName, filterJSON string) (filter.Filter, error) {
	var f filter.Filter
	var finalFilterJSON string
	if profileName != "" {
		err := db.QueryRow("SELECT profile_data_json FROM profiles WHERE project_id = ? AND profile_name = ?", projectID, profileName).Scan(&finalFilterJSON)
		if err != nil {
			if err == sql.ErrNoRows {
				return f, fmt.Errorf("profile '%s' not found", profileName)
			}
			return f, err
		}
	} else if filterJSON != "" {
		finalFilterJSON = filterJSON
	}
	if finalFilterJSON != "" {
		if err := json.Unmarshal([]byte(finalFilterJSON), &f); err != nil {
			return f, err
		}
	}
	return f, nil
}

// This init function registers all commands and their flags to Cobra and Viper.
func init() {
	rootCmd.AddCommand(reportCmd)

	reportCmd.AddCommand(reportListTemplatesCmd)

	reportCmd.AddCommand(reportGenerateCmd)
	reportGenerateCmd.Flags().String("project-path", "", "Path to the project")
	reportGenerateCmd.Flags().String("template", "default-md", "Name of a built-in template or path to a custom .hbs file")
	reportGenerateCmd.Flags().String("output", "report.md", "Path to the output report file")
	reportGenerateCmd.Flags().String("profile-name", "", "Name of a saved filter profile to use for filtering content")
	reportGenerateCmd.Flags().String("filter-json", "", "A temporary JSON string with filter conditions to use")
	viper.BindPFlag("report.generate.project-path", reportGenerateCmd.Flags().Lookup("project-path"))
	viper.BindPFlag("report.generate.template", reportGenerateCmd.Flags().Lookup("template"))
	viper.BindPFlag("report.generate.output", reportGenerateCmd.Flags().Lookup("output"))
	viper.BindPFlag("report.generate.profile-name", reportGenerateCmd.Flags().Lookup("profile-name"))
	viper.BindPFlag("report.generate.filter-json", reportGenerateCmd.Flags().Lookup("filter-json"))
}
