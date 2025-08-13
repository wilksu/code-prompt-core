package templates

import (
	"embed"
	"fmt"
	"path/filepath"
	"strings"
)

// The //go:embed directive now embeds all .hbs files located in this same directory.
//
//go:embed *.hbs
var FS embed.FS

// TemplateInfo holds metadata for our built-in templates.
type TemplateInfo struct {
	Name        string // User-friendly name, e.g., "default-md"
	FileName    string // Filename within the embed.FS, e.g., "default.md.hbs"
	Description string
}

// BuiltInTemplates will be populated dynamically at program startup and is exported for other packages to use.
var BuiltInTemplates []TemplateInfo

// The init function populates our list of built-in templates dynamically.
func init() {
	entries, err := FS.ReadDir(".")
	if err != nil {
		panic(fmt.Sprintf("failed to read embedded templates directory: %v", err))
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".hbs") {
			fileName := entry.Name()
			BuiltInTemplates = append(BuiltInTemplates, TemplateInfo{
				Name:        templateFileNameToFriendlyName(fileName),
				FileName:    fileName,
				Description: "A built-in report template.",
			})
		}
	}
}

// templateFileNameToFriendlyName converts a filename like "default-md.hbs" to "default-md".
func templateFileNameToFriendlyName(filename string) string {
	base := filepath.Base(filename)
	return strings.TrimSuffix(base, ".hbs")
}
