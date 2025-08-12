package scanner

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"time"

	gitignore "github.com/sabhiram/go-gitignore"
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

// ScanOptions encapsulates all configuration for a scanning operation.
type ScanOptions struct {
	NoGitIgnores     bool
	IncludeBinary    bool
	NoPresetExcludes bool // New: Disables default preset exclusions if true
}

// presetExclusions defines common dependency/build directories to ignore by default.
var presetExclusions = map[string][]string{
	"node":   {"node_modules"},
	"python": {"venv", ".venv", "__pycache__", ".pytest_cache", ".tox", "build", "dist", "*.egg-info"},
	"rust":   {"target"},
	"go":     {"vendor"},
	"java":   {"build", ".gradle", "target", ".idea/libraries"},
	"ide":    {".idea", ".vscode"},
}

func ScanProject(projectPath string, options ScanOptions) ([]FileMetadata, error) {
	var files []FileMetadata

	var ignoreMatcher *gitignore.GitIgnore
	if !options.NoGitIgnores {
		var err error
		ignoreMatcher, err = gitignore.CompileIgnoreFile(filepath.Join(projectPath, ".gitignore"))
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	// If default exclusions are enabled, build the exclusion list from all presets.
	excludePatterns := make(map[string]struct{})
	if !options.NoPresetExcludes {
		for _, patterns := range presetExclusions {
			for _, pattern := range patterns {
				excludePatterns[pattern] = struct{}{}
			}
		}
	}

	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(projectPath, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		// --- Pre-scan Filtering Logic ---

		// 1. Gitignore Check
		if !options.NoGitIgnores && ignoreMatcher != nil && ignoreMatcher.MatchesPath(relPath) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 2. Preset Exclusions Check (for directories)
		if info.IsDir() {
			dirName := info.Name()
			if _, exists := excludePatterns[dirName]; exists {
				return filepath.SkipDir
			}
			if dirName == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		// 3. Text/Binary Check
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
			return nil
		}

		_, err = file.Seek(0, 0)
		if err != nil {
			return err
		}

		hash := sha256.New()
		if _, err := io.Copy(hash, file); err != nil {
			return err
		}
		contentHash := hex.EncodeToString(hash.Sum(nil))

		_, err = file.Seek(0, 0)
		if err != nil {
			return err
		}

		lineCount := 0
		if isText {
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				lineCount++
			}
		}

		ext := filepath.Ext(info.Name())
		if ext != "" {
			ext = ext[1:]
		}

		files = append(files, FileMetadata{
			RelativePath: relPath,
			Filename:     info.Name(),
			Extension:    ext,
			SizeBytes:    info.Size(),
			LineCount:    lineCount,
			IsText:       isText,
			LastModTime:  info.ModTime(),
			ContentHash:  contentHash,
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}
