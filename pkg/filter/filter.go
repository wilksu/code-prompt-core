// File: pkg/filter/filter.go
package filter

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

type Filter struct {
	IncludePaths    []string `json:"includePaths,omitempty"`
	ExcludePaths    []string `json:"excludePaths,omitempty"`
	IncludeExts     []string `json:"includeExts,omitempty"`
	ExcludeExts     []string `json:"excludeExts,omitempty"`
	IncludePrefixes []string `json:"includePrefixes,omitempty"`
	ExcludePrefixes []string `json:"excludePrefixes,omitempty"`

	IncludeRegex []string `json:"includeRegex,omitempty"`
	ExcludeRegex []string `json:"excludeRegex,omitempty"`

	Priority string `json:"priority"`

	compiledIncludeRegex []*regexp.Regexp `json:"-"`
	compiledExcludeRegex []*regexp.Regexp `json:"-"`
}

func (f *Filter) Compile() error {
	var allIncludeRegex, allExcludeRegex []string

	allIncludeRegex = append(allIncludeRegex, f.IncludeRegex...)
	allExcludeRegex = append(allExcludeRegex, f.ExcludeRegex...)

	for _, path := range f.IncludePaths {
		regexPath := regexp.QuoteMeta(filepath.ToSlash(path))
		if !strings.HasSuffix(regexPath, "/") {
			allIncludeRegex = append(allIncludeRegex, "^"+regexPath+"$")
		} else {
			allIncludeRegex = append(allIncludeRegex, "^"+regexPath+".*")
		}
	}
	for _, path := range f.ExcludePaths {
		regexPath := regexp.QuoteMeta(filepath.ToSlash(path))
		if !strings.HasSuffix(regexPath, "/") {
			allExcludeRegex = append(allExcludeRegex, "^"+regexPath+"$")
		} else {
			allExcludeRegex = append(allExcludeRegex, "^"+regexPath+".*")
		}
	}

	for _, ext := range f.IncludeExts {
		cleanExt := strings.TrimPrefix(ext, ".")
		allIncludeRegex = append(allIncludeRegex, `\.`+regexp.QuoteMeta(cleanExt)+"$")
	}
	for _, ext := range f.ExcludeExts {
		cleanExt := strings.TrimPrefix(ext, ".")
		allExcludeRegex = append(allExcludeRegex, `\.`+regexp.QuoteMeta(cleanExt)+"$")
	}

	for _, prefix := range f.IncludePrefixes {
		// *** 简化正则表达式，不再需要匹配'\\' ***
		allIncludeRegex = append(allIncludeRegex, `(^|/)`+regexp.QuoteMeta(prefix)+".*")
	}
	for _, prefix := range f.ExcludePrefixes {
		// *** 简化正则表达式，不再需要匹配'\\' ***
		allExcludeRegex = append(allExcludeRegex, `(^|/)`+regexp.QuoteMeta(prefix)+".*")
	}

	f.compiledIncludeRegex = []*regexp.Regexp{}
	for _, p := range allIncludeRegex {
		if p == "" {
			continue
		}
		re, err := regexp.Compile(p)
		if err != nil {
			return fmt.Errorf("invalid include regex pattern '%s': %w", p, err)
		}
		f.compiledIncludeRegex = append(f.compiledIncludeRegex, re)
	}

	f.compiledExcludeRegex = []*regexp.Regexp{}
	for _, p := range allExcludeRegex {
		if p == "" {
			continue
		}
		re, err := regexp.Compile(p)
		if err != nil {
			return fmt.Errorf("invalid exclude regex pattern '%s': %w", p, err)
		}
		f.compiledExcludeRegex = append(f.compiledExcludeRegex, re)
	}

	return nil
}

func (f *Filter) GetCompiledIncludeRegex() []*regexp.Regexp {
	return f.compiledIncludeRegex
}

func (f *Filter) GetCompiledExcludeRegex() []*regexp.Regexp {
	return f.compiledExcludeRegex
}

func GetFilteredFilePaths(db *sql.DB, projectID int64, filter Filter) ([]string, error) {
	rows, err := db.Query("SELECT relative_path FROM file_metadata WHERE project_id = ?", projectID)
	if err != nil {
		return nil, fmt.Errorf("error querying file metadata: %w", err)
	}
	defer rows.Close()

	var resultingPaths []string
	for rows.Next() {
		var relativePath string
		if err := rows.Scan(&relativePath); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}

		matchInclude := len(filter.compiledIncludeRegex) == 0 || MatchesAny(relativePath, filter.compiledIncludeRegex)
		matchExclude := len(filter.compiledExcludeRegex) > 0 && MatchesAny(relativePath, filter.compiledExcludeRegex)

		shouldAdd := false
		if matchInclude && matchExclude {
			shouldAdd = (filter.Priority != "excludes")
		} else if matchInclude {
			shouldAdd = true
		} else if matchExclude {
			shouldAdd = false
		} else {
			shouldAdd = len(filter.compiledIncludeRegex) == 0
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

func MatchesAny(path string, patterns []*regexp.Regexp) bool {
	for _, re := range patterns {
		if re.MatchString(path) {
			return true
		}
	}
	return false
}
