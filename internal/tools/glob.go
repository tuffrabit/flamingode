package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// MaxGlobOutputSize is the maximum number of result bytes to return.
const MaxGlobOutputSize = 100_000

// Glob searches for files matching a glob pattern within the working directory.
type Glob struct {
	WorkingDir string
}

func (g *Glob) GetName() string {
	return "glob"
}

func (g *Glob) GetDescription() string {
	return "Search for files matching a glob pattern within the working directory. Returns matching file paths as newline-separated relative paths. Supports ** for recursive matching. Respects .gitignore patterns and skips common dependency/build directories."
}

func (g *Glob) GetPermissionRequired() bool {
	return false
}

func (g *Glob) GetParameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "Glob pattern to match filenames against (e.g. '*.go', '**/*.md', 'cmd/*/main.go'). Supports ** to match any number of directory levels.",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Relative path to a directory to search within the working directory. Use '.' for the current directory.",
			},
		},
		"required": []string{"pattern", "path"},
	}
}

func (g *Glob) GetAction() ToolAction {
	return func(ctx context.Context, arguments string) (string, error) {
		var args struct {
			Pattern string `json:"pattern"`
			Path    string `json:"path"`
		}
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		if args.Pattern == "" {
			return "", fmt.Errorf("pattern is required")
		}
		if args.Path == "" {
			args.Path = "."
		}

		// Resolve to an absolute path within the working directory.
		absPath, err := filepath.Abs(filepath.Join(g.WorkingDir, args.Path))
		if err != nil {
			return "", fmt.Errorf("invalid path: %w", err)
		}

		// Resolve any symlinks in the path.
		resolvedPath, err := filepath.EvalSymlinks(absPath)
		if err != nil {
			return "", fmt.Errorf("cannot access path: %w", err)
		}

		cleanPath := filepath.Clean(resolvedPath)
		cleanWorkingDir := filepath.Clean(g.WorkingDir)

		// Restrict access to the working directory tree.
		if cleanPath != cleanWorkingDir && !strings.HasPrefix(cleanPath, cleanWorkingDir+string(filepath.Separator)) {
			return "", fmt.Errorf("access denied: path is outside the working directory")
		}

		// Parse .gitignore from the working directory.
		gitignorePath := filepath.Join(g.WorkingDir, ".gitignore")
		giPatterns, err := parseGitignore(gitignorePath)
		if err != nil {
			return "", fmt.Errorf("failed to parse .gitignore: %w", err)
		}

		var matches []string

		// Walk the directory tree and match files.
		err = filepath.WalkDir(cleanPath, func(path string, d os.DirEntry, err error) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err != nil {
				return nil // skip files we can't access
			}

			// Skip gitignore check for the root of the walk so explicitly requested paths are always searched.
			if path != cleanPath {
				relPath, _ := filepath.Rel(cleanWorkingDir, path)
				if matchesGitignore(path, relPath, d.IsDir(), giPatterns) {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}

			if d.IsDir() {
				if skipDirs[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}

			// Only match regular files — skip symlinks, FIFOs, sockets, devices.
			if !d.Type().IsRegular() {
				return nil
			}

			relPath, err := filepath.Rel(cleanWorkingDir, path)
			if err != nil {
				relPath = path
			}

			// Convert to forward slashes for glob matching.
			relPathSlash := filepath.ToSlash(relPath)

			matched, err := doublestar.Match(args.Pattern, relPathSlash)
			if err != nil {
				return nil // invalid pattern, skip
			}
			if matched {
				matches = append(matches, relPathSlash)
			}

			return nil
		})
		if err != nil && err != filepath.SkipAll {
			return "", fmt.Errorf("search failed: %w", err)
		}

		if len(matches) == 0 {
			return "No matches found.", nil
		}

		// Build result string with size limit.
		var result strings.Builder
		for _, m := range matches {
			line := m + "\n"
			if result.Len()+len(line) > MaxGlobOutputSize {
				result.WriteString("[Output truncated: exceeded max result size]\n")
				break
			}
			result.WriteString(line)
		}

		return fmt.Sprintf("(%d matches)\n%s", len(matches), result.String()), nil
	}
}
