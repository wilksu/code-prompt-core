package scanner

import (
	"bufio"
	"context" // 1. 引入 context 包
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
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

var presetExclusions = map[string]struct{}{
	"node_modules":  {},
	"venv":          {},
	".venv":         {},
	"__pycache__":   {},
	".pytest_cache": {},
	".tox":          {},
	"build":         {},
	"dist":          {},
	".egg-info":     {},
	"target":        {},
	"vendor":        {},
	".gradle":       {},
	".idea":         {},
	".vscode":       {},
	".git":          {},
}

// processFile handles the heavy lifting for a single file.
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

	// 2. 使用 context.Background() 初始化上下文感知的 Pool
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
		if d.IsDir() {
			if _, exists := presetExclusions[d.Name()]; !options.NoPresetExcludes && exists {
				return filepath.SkipDir
			}
			if ignoreMatcher != nil && ignoreMatcher.MatchesPath(relPath) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || (ignoreMatcher != nil && ignoreMatcher.MatchesPath(relPath)) {
			return nil
		}
		pathPool.Go(func() {
			info, err := d.Info()
			if err != nil {
				return
			}
			// 3. 为提交的函数添加 context.Context 参数
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
