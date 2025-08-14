// File: pkg/scanner/scanner.go
package scanner

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"time"

	gitignore "github.com/sabhiram/go-gitignore"
	"github.com/sourcegraph/conc/pool"
)

type FileMetadata struct {
	RelativePath string
	Filename     string
	Extension    string
	SizeBytes    int64
	LineCount    int
	IsText       bool
	LastModTime  time.Time
	ContentHash  string
}

type ScanOptions struct {
	NoGitIgnores     bool
	IncludeBinary    bool
	NoPresetExcludes bool
}

var presetExclusionPatterns = []string{
	`\.git`,
	`node_modules`,
	`venv`,
	`\.venv`,
	`__pycache__`,
	`\.pytest_cache`,
	`\.tox`,
	`build`,
	`dist`,
	`\.egg-info`,
	`target`,
	`vendor`,
	`\.gradle`,
	`\.idea`,
	`\.vscode`,
}

func processFile(path, projectPath string, info os.FileInfo, options ScanOptions) (FileMetadata, error) {
	var meta FileMetadata

	// 1. Text/Binary Check & Open File
	file, err := os.Open(path)
	if err != nil {
		return meta, err
	}
	defer file.Close()

	buffer := make([]byte, 512)
	n, _ := file.Read(buffer)
	isText := true
	for _, b := range buffer[:n] {
		if b == 0 {
			isText = false
			break
		}
	}

	if !isText && !options.IncludeBinary {
		return FileMetadata{}, nil // Return empty struct to signal skipping
	}

	// 2. Hash Calculation
	_, err = file.Seek(0, 0)
	if err != nil {
		return meta, err
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return meta, err
	}
	contentHash := hex.EncodeToString(hash.Sum(nil))

	// 3. Line Count
	lineCount := 0
	if isText {
		_, err = file.Seek(0, 0)
		if err != nil {
			return meta, err
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			lineCount++
		}
	}

	// 4. Assemble Metadata
	relPath, _ := filepath.Rel(projectPath, path)
	ext := filepath.Ext(info.Name())
	if ext != "" {
		ext = ext[1:]
	}

	meta = FileMetadata{
		RelativePath: relPath,
		Filename:     info.Name(),
		Extension:    ext,
		SizeBytes:    info.Size(),
		LineCount:    lineCount,
		IsText:       isText,
		LastModTime:  info.ModTime().UTC(),
		ContentHash:  contentHash,
	}
	return meta, nil
}

func ScanProject(projectPath string, options ScanOptions) ([]FileMetadata, error) {
	var ignoreMatcher *gitignore.GitIgnore
	if !options.NoGitIgnores {
		ignoreMatcher, _ = gitignore.CompileIgnoreFile(filepath.Join(projectPath, ".gitignore"))
	}

	// Compile preset exclusion regex patterns once.
	var compiledPresetExcludes []*regexp.Regexp
	if !options.NoPresetExcludes {
		for _, p := range presetExclusionPatterns {
			re, err := regexp.Compile(p)
			if err != nil {
				return nil, fmt.Errorf("invalid preset exclusion pattern '%s': %w", p, err)
			}
			compiledPresetExcludes = append(compiledPresetExcludes, re)
		}
	}

	resultPool := pool.NewWithResults[FileMetadata]().WithErrors().WithContext(context.Background())
	pathPool := pool.New().WithMaxGoroutines(runtime.NumCPU())

	walkErr := filepath.WalkDir(projectPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == projectPath {
			return nil
		}
		relPath, err := filepath.Rel(projectPath, path)
		if err != nil {
			return err
		}

		// NEW: Check against preset exclusions first
		for _, re := range compiledPresetExcludes {
			if re.MatchString(relPath) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil // Skip this file
			}
		}

		// Check against .gitignore
		if ignoreMatcher != nil && ignoreMatcher.MatchesPath(relPath) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		if !d.Type().IsRegular() {
			return nil
		}

		pathPool.Go(func() {
			info, err := d.Info()
			if err != nil {
				return
			}
			resultPool.Go(func(_ context.Context) (FileMetadata, error) {
				return processFile(path, projectPath, info, options)
			})
		})
		return nil
	})

	pathPool.Wait()
	results, processErr := resultPool.Wait()

	if walkErr != nil {
		return nil, walkErr
	}
	if processErr != nil {
		return nil, processErr
	}

	finalResults := make([]FileMetadata, 0, len(results))
	for _, res := range results {
		if res.RelativePath != "" {
			finalResults = append(finalResults, res)
		}
	}
	return finalResults, nil
}
