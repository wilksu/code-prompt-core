// File: pkg/filter/filter.go

package filter

import (
	"database/sql"
	"fmt"
	"strings"
)

// Filter defines the unified structure for all filtering criteria.
// This struct is now the single source of truth for filter shapes.
type Filter struct {
	ExcludedExtensions []string `json:"excludedExtensions"`
	ExcludedPrefixes   []string `json:"excludedPrefixes"`
	IsTextOnly         bool     `json:"isTextOnly"`
}

// GetFilteredFilePaths queries the database for file metadata based on the provided filter criteria.
// It returns a slice of relative file paths that match the filters.
// This is the core reusable filtering logic.
func GetFilteredFilePaths(db *sql.DB, projectID int64, filter Filter) ([]string, error) {
	baseQuery := "SELECT relative_path, extension, is_text FROM file_metadata WHERE project_id = ?"
	queryParams := []interface{}{projectID}

	// Apply SQL-level filters first for efficiency
	if filter.IsTextOnly {
		baseQuery += " AND is_text = ?"
		queryParams = append(queryParams, true)
	}

	rows, err := db.Query(baseQuery, queryParams...)
	if err != nil {
		return nil, fmt.Errorf("error querying file metadata: %w", err)
	}
	defer rows.Close()

	var resultingPaths []string

FileLoop:
	for rows.Next() {
		var relativePath, extension string
		var isText bool
		if err := rows.Scan(&relativePath, &extension, &isText); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}

		// Apply remaining filters in-memory, as they are harder to express in pure SQL
		for _, ext := range filter.ExcludedExtensions {
			if ext == "" {
				continue
			}
			if ext == "no_extension" {
				if extension == "" {
					continue FileLoop
				}
			} else {
				cleanExt := strings.TrimPrefix(ext, ".")
				if extension == cleanExt {
					continue FileLoop
				}
			}
		}

		for _, prefix := range filter.ExcludedPrefixes {
			if strings.HasPrefix(relativePath, prefix) {
				continue FileLoop
			}
		}

		resultingPaths = append(resultingPaths, relativePath)
	}

	return resultingPaths, nil
}
