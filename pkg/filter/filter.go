// File: pkg/filter/filter.go
package filter

import (
	"database/sql"
	"fmt"
	"regexp"
)

// Filter defines the new, unified structure for all filtering criteria using regular expressions.
type Filter struct {
	Includes []string `json:"includes"` // List of regex patterns to include files.
	Excludes []string `json:"excludes"` // List of regex patterns to exclude files.
	Priority string   `json:"priority"` // "includes" or "excludes". Determines precedence when a file matches both lists. Defaults to "includes".
}

// GetFilteredFilePaths queries the database and applies the new regex-based filtering logic.
func GetFilteredFilePaths(db *sql.DB, projectID int64, filter Filter) ([]string, error) {
	// 1. Compile regex patterns
	var includesRegex []*regexp.Regexp
	for _, p := range filter.Includes {
		if p == "" {
			continue
		}
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("invalid 'includes' regex pattern '%s': %w", p, err)
		}
		includesRegex = append(includesRegex, re)
	}

	var excludesRegex []*regexp.Regexp
	for _, p := range filter.Excludes {
		if p == "" {
			continue
		}
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("invalid 'excludes' regex pattern '%s': %w", p, err)
		}
		excludesRegex = append(excludesRegex, re)
	}

	// 2. Fetch all file paths for the project
	rows, err := db.Query("SELECT relative_path FROM file_metadata WHERE project_id = ?", projectID)
	if err != nil {
		return nil, fmt.Errorf("error querying file metadata: %w", err)
	}
	defer rows.Close()

	// 3. Apply filtering logic
	var resultingPaths []string
	for rows.Next() {
		var relativePath string
		if err := rows.Scan(&relativePath); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}

		// Determine if the path should be included
		matchInclude := len(includesRegex) == 0 || MatchesAny(relativePath, includesRegex)
		matchExclude := len(excludesRegex) > 0 && MatchesAny(relativePath, excludesRegex)

		shouldAdd := false
		if matchInclude && matchExclude {
			// If a file matches both lists, priority rule applies. Default to 'includes'
			shouldAdd = (filter.Priority != "excludes")
		} else if matchInclude {
			shouldAdd = true
		} else if matchExclude {
			shouldAdd = false
		} else {
			// If no rules are matched, it's not included unless the includes list was empty.
			shouldAdd = len(includesRegex) == 0
		}

		if shouldAdd {
			resultingPaths = append(resultingPaths, relativePath)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during row iteration: %w", err)
	}

	return resultingPaths, nil
}

// MatchesAny checks if the given path matches any of the regex patterns.
func MatchesAny(path string, patterns []*regexp.Regexp) bool {
	for _, re := range patterns {
		if re.MatchString(path) {
			return true
		}
	}
	return false
}
