package scanner

import (
	"bufio"
	"os"
	"path/filepath"

	gitignore "github.com/sabhiram/go-gitignore"
)

type FileMetadata struct {
	RelativePath string
	Filename     string
	Extension    string
	SizeBytes    int64
	LineCount    int
	IsText       bool
}

func ScanProject(projectPath string, noGitIgnores bool) ([]FileMetadata, error) {
	var files []FileMetadata

	var ignoreMatcher *gitignore.GitIgnore
	if !noGitIgnores {
		var err error
		ignoreMatcher, err = gitignore.CompileIgnoreFile(filepath.Join(projectPath, ".gitignore"))
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(projectPath, path)
		if err != nil {
			return err
		}

		if !noGitIgnores && ignoreMatcher != nil && ignoreMatcher.MatchesPath(relPath) {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return nil // Skip files that can't be opened
		}
		defer file.Close()

		// Basic text file detection
		buffer := make([]byte, 512)
		n, _ := file.Read(buffer)
		file.Seek(0, 0)
		isText := true
		for _, b := range buffer[:n] {
			if b == 0 {
				isText = false
				break
			}
		}

		lineCount := 0
		if isText {
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				lineCount++
			}
		}

		files = append(files, FileMetadata{
			RelativePath: relPath,
			Filename:     info.Name(),
			Extension:    filepath.Ext(info.Name()),
			SizeBytes:    info.Size(),
			LineCount:    lineCount,
			IsText:       isText,
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}
