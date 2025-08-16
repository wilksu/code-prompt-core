// File: cmd/report.go
package cmd

import (
	"database/sql"
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
      "name": "summary.txt",
      "description": "A built-in report template."
    }
  ]
}`,
	Run: func(cmd *cobra.Command, args []string) {
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

You can filter the files included in the report using either a saved profile via '--profile-name' or a temporary filter via '--filter-json'. If both are provided, '--filter-json' takes precedence.

The filter JSON structure supports both simple and advanced rules:
{
  "includeExts": ["go"],
  "excludePaths": ["vendor/"],
  "priority": "includes"
}

If the '--output' flag is provided with a file path, the report is saved to that file. Otherwise, the report content is printed directly to the standard output.

Example (using a built-in template and a filter):
  code-prompt-core report generate --template summary.txt --filter-json '{"includeExts":["go"]}' --output report.txt`,
	Run: func(cmd *cobra.Command, args []string) {
		templateIdentifier := viper.GetString("report.generate.template")
		outputPath := viper.GetString("report.generate.output")
		if templateIdentifier == "" {
			printError(fmt.Errorf("--template is required"))
			return
		}

		absProjectPath, err := getAbsoluteProjectPath("report.generate.project-path")
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

		raymond.RegisterHelper("humanizeBytes", func(bytes int64) string {
			return humanize.Bytes(uint64(bytes))
		})
		templateContent, err := getTemplateContent(templateIdentifier)
		if err != nil {
			printError(err)
			return
		}
		if strings.Contains(templateContent, "{{> treePartial") {
			treePartial := `{{#each nodes}}{{this.indent}}├── {{{this.Name}}} {{#if this.SizeBytes}}({{humanizeBytes this.SizeBytes}}){{/if}}{{#if this.isDir}}/{{/if}}
{{#if this.Children}}{{> treePartial nodes=this.Children indent=(append this.indent "    ")}}{{/if}}{{/each}}`
			raymond.RegisterPartial("treePartial", treePartial)
			raymond.RegisterHelper("append", func(base, addition string) string {
				return base + addition
			})
		}

		f, err := getFilter(
			db,
			projectID,
			viper.GetString("report.generate.profile-name"),
			viper.GetString("report.generate.filter-json"),
		)
		if err != nil {
			printError(err)
			return
		}

		reportCtx, err := buildReportContext(db, projectID, absProjectPath, f)
		if err != nil {
			printError(fmt.Errorf("error building report context: %w", err))
			return
		}

		result, err := raymond.Render(templateContent, reportCtx)
		if err != nil {
			printError(fmt.Errorf("error rendering template: %w", err))
			return
		}

		if outputPath != "" {
			err = os.WriteFile(outputPath, []byte(result), 0644)
			if err != nil {
				printError(fmt.Errorf("error writing output file '%s': %w", outputPath, err))
				return
			}
			printJSON(map[string]string{
				"message":    "Report generated successfully",
				"outputPath": outputPath,
			})
		} else {
			// 将原始报告文本作为data字段的值，通过标准JSON格式输出
			printJSON(result)
		}
	},
}

func getTemplateContent(identifier string) (string, error) {
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

func buildReportContext(db *sql.DB, projectID int64, absProjectPath string, f filter.Filter) (map[string]interface{}, error) {
	stats, err := getStatsData(db, projectID, f)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats data: %w", err)
	}

	tree, err := getTreeData(db, projectID, absProjectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get tree data: %w", err)
	}

	contents, err := getContentsData(db, projectID, absProjectPath, f)
	if err != nil {
		return nil, fmt.Errorf("failed to get contents data: %w", err)
	}

	ctx := map[string]interface{}{
		"project_path":       absProjectPath,
		"absolute_code_path": absProjectPath,
		"generated_at":       time.Now().Format(time.RFC1123),
		"config":             f,
		"stats":              stats,
		"tree":               tree,
		"files":              contents,
	}
	return ctx, nil
}

type TemplateStat struct {
	ExtName    string `json:"extName"`
	FileCount  int    `json:"fileCount"`
	TotalSize  int64  `json:"totalSize"`
	TotalLines int    `json:"totalLines"`
	IsIncluded bool   `json:"isIncluded"`
}

func getStatsData(db *sql.DB, projectID int64, f filter.Filter) (map[string]interface{}, error) {
	rows, err := db.Query("SELECT extension, COUNT(*), SUM(size_bytes), SUM(line_count), GROUP_CONCAT(relative_path) FROM file_metadata WHERE project_id = ? GROUP BY extension", projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var statsList []TemplateStat
	var totalFiles, totalLines int
	var totalSize int64

	compiledIncludes := f.GetCompiledIncludeRegex()
	compiledExcludes := f.GetCompiledExcludeRegex()

	for rows.Next() {
		var ext sql.NullString
		var s TemplateStat
		var relativePathsStr sql.NullString
		if err := rows.Scan(&ext, &s.FileCount, &s.TotalSize, &s.TotalLines, &relativePathsStr); err != nil {
			return nil, err
		}

		s.ExtName = "no_extension"
		if ext.Valid && ext.String != "" {
			s.ExtName = ext.String
		}

		s.IsIncluded = false
		if relativePathsStr.Valid {
			paths := strings.Split(relativePathsStr.String, ",")
			for _, path := range paths {
				matchInclude := len(compiledIncludes) == 0 || filter.MatchesAny(path, compiledIncludes)
				matchExclude := len(compiledExcludes) > 0 && filter.MatchesAny(path, compiledExcludes)

				priority := f.Priority
				if priority == "" {
					priority = "includes"
				}

				if (matchInclude && !matchExclude) || (matchInclude && matchExclude && priority == "includes") {
					s.IsIncluded = true
					break
				}
			}
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

func getTreeData(db *sql.DB, projectID int64, absProjectPath string) (*TreeNode, error) {
	rows, err := db.Query("SELECT relative_path, size_bytes FROM file_metadata WHERE project_id = ? ORDER BY relative_path ASC", projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	root := &TreeNode{Name: filepath.Base(absProjectPath), IsDir: true}
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

func getContentsData(db *sql.DB, projectID int64, absProjectPath string, f filter.Filter) (map[string]string, error) {
	relativePaths, err := filter.GetFilteredFilePaths(db, projectID, f)
	if err != nil {
		return nil, err
	}
	contentMap := make(map[string]string)
	for _, relPath := range relativePaths {
		fullPath := filepath.Join(absProjectPath, filepath.Clean(relPath))
		content, err := os.ReadFile(fullPath)
		if err != nil {
			contentMap[relPath] = fmt.Sprintf("Error: Unable to read file. %v", err)
		} else {
			contentMap[relPath] = string(content)
		}
	}
	return contentMap, nil
}

func init() {
	rootCmd.AddCommand(reportCmd)

	reportCmd.AddCommand(reportListTemplatesCmd)

	reportCmd.AddCommand(reportGenerateCmd)
	reportGenerateCmd.Flags().String("project-path", "", "Path to the project")
	reportGenerateCmd.Flags().String("template", "summary.txt", "Name of a built-in template or path to a custom .hbs file")
	reportGenerateCmd.Flags().String("output", "", "Path to the output report file. If empty, prints to stdout.")
	reportGenerateCmd.Flags().String("profile-name", "", "Name of a saved filter profile to use for filtering content")
	reportGenerateCmd.Flags().String("filter-json", "", "A temporary JSON string with filter conditions to use (overrides profile-name)")
	viper.BindPFlag("report.generate.project-path", reportGenerateCmd.Flags().Lookup("project-path"))
	viper.BindPFlag("report.generate.template", reportGenerateCmd.Flags().Lookup("template"))
	viper.BindPFlag("report.generate.output", reportGenerateCmd.Flags().Lookup("output"))
	viper.BindPFlag("report.generate.profile-name", reportGenerateCmd.Flags().Lookup("profile-name"))
	viper.BindPFlag("report.generate.filter-json", reportGenerateCmd.Flags().Lookup("filter-json"))
}
