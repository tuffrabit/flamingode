package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ListDirectory lists the contents of a directory within the working directory.
type ListDirectory struct {
	WorkingDir string
}

func (l *ListDirectory) GetName() string {
	return "list_directory"
}

func (l *ListDirectory) GetDescription() string {
	return "List the contents of a directory. Returns a JSON array of objects with name, type (file or directory), and size (for files). The path must be relative to the working directory; use '.' for the current directory."
}

func (l *ListDirectory) GetPermissionRequired() bool {
	return false
}

func (l *ListDirectory) GetParameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Relative path to the directory to list. Use '.' for the current directory. Defaults to the current working directory if omitted.",
			},
		},
		"required": []string{},
	}
}

// ListDirectoryContent lists the contents of a directory within the working directory
// and returns a JSON string describing the entries.
func ListDirectoryContent(workingDir, relPath string) (string, error) {
	if relPath == "" {
		relPath = "."
	}

	cleanPath, err := ResolveWorkingDirPath(workingDir, relPath)
	if err != nil {
		return "", err
	}

	entries, err := os.ReadDir(cleanPath)
	if err != nil {
		return "", fmt.Errorf("failed to read directory: %w", err)
	}

	type item struct {
		Name string `json:"name"`
		Type string `json:"type"`
		Size int64  `json:"size,omitempty"`
	}

	var items []item
	for _, entry := range entries {
		info, err := os.Stat(filepath.Join(cleanPath, entry.Name()))
		if err != nil {
			continue // skip entries we can't stat
		}

		it := item{Name: entry.Name()}
		if info.IsDir() {
			it.Type = "directory"
		} else {
			it.Type = "file"
			it.Size = info.Size()
		}
		items = append(items, it)
	}

	// Sort directories first, then alphabetically.
	sort.Slice(items, func(i, j int) bool {
		if items[i].Type == items[j].Type {
			return items[i].Name < items[j].Name
		}
		return items[i].Type == "directory"
	})

	out, err := json.Marshal(items)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(out), nil
}

func (l *ListDirectory) GetAction() ToolAction {
	return func(ctx context.Context, arguments string) (string, error) {
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		return ListDirectoryContent(l.WorkingDir, args.Path)
	}
}
