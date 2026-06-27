package main

import (
	"os"
	"path/filepath"
	"sort"
)

// FileEntry represents a file or directory in the editor's file tree.
type FileEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
}

// ReadFile reads and returns the contents of a file as a UTF-8 string.
func (a *App) ReadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteFile writes content to a file, creating it if necessary.
func (a *App) WriteFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

// ListDirectory returns the entries in a directory, sorted directories-first then
// alphabetically. Hidden files/directories (names starting with ".") are skipped.
func (a *App) ListDirectory(path string) ([]FileEntry, error) {
	dirEntries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var entries []FileEntry
	for _, e := range dirEntries {
		if e.Name()[0] == '.' {
			continue
		}
		entries = append(entries, FileEntry{
			Name:  e.Name(),
			Path:  filepath.Join(path, e.Name()),
			IsDir: e.IsDir(),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return entries[i].Name < entries[j].Name
	})

	return entries, nil
}
