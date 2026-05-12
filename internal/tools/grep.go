package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

// MaxGrepOutputSize is the maximum number of result bytes to return.
const MaxGrepOutputSize = 100_000

// MaxGrepFileSize is the largest file we will scan (10 MB).
const MaxGrepFileSize = 10 * 1024 * 1024

// Default scanner buffer size for long lines (1 MB).
const grepScanBuffer = 1024 * 1024

// skipDirs are directories we never descend into.
var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	"__pycache__":  true,
	".venv":        true,
	"venv":         true,
	"target":       true,
	"dist":         true,
	"build":        true,
	".idea":        true,
	".vscode":      true,
}

// Grep searches for a pattern in files within the working directory.
type Grep struct {
	WorkingDir string
}

func (g *Grep) GetName() string {
	return "grep"
}

func (g *Grep) GetDescription() string {
	return "Search for a pattern in files within the working directory. Returns matching lines with file paths and optional line numbers. Skips binary files, respects .gitignore patterns, and ignores common dependency/build directories. Supports literal text or regex search."
}

func (g *Grep) GetParameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "The search pattern. Treated as literal text unless regex is true.",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Relative path to a file or directory to search within the working directory. Use '.' for the current directory.",
			},
			"regex": map[string]interface{}{
				"type":        "boolean",
				"description": "If true, treat query as a regular expression. If false, perform a literal text search.",
			},
			"line_numbers": map[string]interface{}{
				"type":        "boolean",
				"description": "If true, include line numbers in the output (e.g., file.go:42 match). Defaults to true.",
			},
		},
		"required": []string{"query", "path"},
	}
}

func (g *Grep) GetAction() ToolAction {
	return func(ctx context.Context, arguments string) (string, error) {
		var args struct {
			Query           string `json:"query"`
			Path            string `json:"path"`
			Regex           bool   `json:"regex"`
			LineNumbers     *bool  `json:"line_numbers"`
			CaseInsensitive bool   `json:"case_insensitive"`
		}
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		if args.Query == "" {
			return "", fmt.Errorf("query is required")
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

		// Compile regex if needed.
		var re *regexp.Regexp
		if args.Regex {
			pattern := args.Query
			if args.CaseInsensitive {
				pattern = "(?i:" + pattern + ")"
			}
			re, err = regexp.Compile(pattern)
			if err != nil {
				return "", fmt.Errorf("invalid regex: %w", err)
			}
		}

		lineNumbers := true
		if args.LineNumbers != nil {
			lineNumbers = *args.LineNumbers
		}

		var result bytes.Buffer
		var totalMatches int

		// Parse .gitignore from the working directory.
		gitignorePath := filepath.Join(g.WorkingDir, ".gitignore")
		giPatterns, err := parseGitignore(gitignorePath)
		if err != nil {
			return "", fmt.Errorf("failed to parse .gitignore: %w", err)
		}

		// Walk the directory tree and search files.
		err = filepath.WalkDir(cleanPath, func(path string, d os.DirEntry, err error) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err != nil {
				return nil // skip files we can't access
			}

			relPath, _ := filepath.Rel(cleanWorkingDir, path)
			if matchesGitignore(path, relPath, d.IsDir(), giPatterns) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if d.IsDir() {
				if skipDirs[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}

			// Only open regular files — skip symlinks, FIFOs, sockets, devices.
			if !d.Type().IsRegular() {
				return nil
			}

			matches, err := searchFile(ctx, path, cleanWorkingDir, args.Query, re, lineNumbers, args.CaseInsensitive)
			if err != nil {
				return nil // skip files we can't read
			}

			if len(matches) > 0 {
				totalMatches += len(matches)
				for _, m := range matches {
					if result.Len()+len(m)+1 > MaxGrepOutputSize {
						result.WriteString("[Output truncated: exceeded max result size]\n")
						return filepath.SkipAll
					}
					result.WriteString(m)
					result.WriteByte('\n')
				}
			}
			return nil
		})
		if err != nil && err != filepath.SkipAll {
			return "", fmt.Errorf("search failed: %w", err)
		}

		if result.Len() == 0 {
			return "No matches found.", nil
		}
		return fmt.Sprintf("(%d matches)\n%s", totalMatches, result.String()), nil
	}
}

// searchFile searches a single file for the query and returns matching lines.
func searchFile(ctx context.Context, path, workingDir, query string, re *regexp.Regexp, lineNumbers bool, caseInsensitive bool) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, nil
	}
	if info.Size() > MaxGrepFileSize {
		return nil, nil // skip huge files
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Quick check: skip binary/non-UTF8 files by sampling the first 8KB.
	sample := make([]byte, 8192)
	n, _ := f.Read(sample)
	if bytes.IndexByte(sample[:n], 0) != -1 {
		return nil, nil // skip binary files
	}
	if n > 0 && !utf8.Valid(sample[:n]) {
		return nil, nil // skip non-UTF8 files
	}

	// Reset to beginning for line-by-line scan.
	_, err = f.Seek(0, 0)
	if err != nil {
		return nil, err
	}

	// Use a path relative to the working directory for cleaner output.
	relPath, err := filepath.Rel(workingDir, path)
	if err != nil {
		relPath = path
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, grepScanBuffer), grepScanBuffer)

	lineNum := 1
	var matches []string

	for scanner.Scan() {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		text := scanner.Bytes()
		found := false
		if re != nil {
			found = re.Match(text)
		} else if caseInsensitive {
			found = strings.Contains(strings.ToLower(string(text)), strings.ToLower(query))
		} else {
			found = bytes.Contains(text, []byte(query))
		}

		if found {
			if lineNumbers {
				matches = append(matches, fmt.Sprintf("%s:%d %s", relPath, lineNum, string(text)))
			} else {
				matches = append(matches, fmt.Sprintf("%s %s", relPath, string(text)))
			}
		}
		lineNum++
	}

	return matches, scanner.Err()
}

type gitignorePattern struct {
	raw      string
	anchored bool
	dirOnly  bool
}

func parseGitignore(gitignorePath string) ([]gitignorePattern, error) {
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var patterns []gitignorePattern
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Skip negation patterns for simplicity.
		if strings.HasPrefix(line, "!") {
			continue
		}

		dirOnly := strings.HasSuffix(line, "/")
		if dirOnly {
			line = strings.TrimSuffix(line, "/")
		}

		anchored := strings.HasPrefix(line, "/") || strings.Contains(line, "/")
		if strings.HasPrefix(line, "/") {
			line = strings.TrimPrefix(line, "/")
		}

		patterns = append(patterns, gitignorePattern{
			raw:      line,
			anchored: anchored,
			dirOnly:  dirOnly,
		})
	}
	return patterns, nil
}

func matchesGitignore(pathStr, relPath string, isDir bool, patterns []gitignorePattern) bool {
	if len(patterns) == 0 {
		return false
	}

	base := filepath.Base(pathStr)
	base = filepath.ToSlash(base)
	relPath = filepath.ToSlash(relPath)

	for _, p := range patterns {
		if p.dirOnly && !isDir {
			continue
		}

		var target string
		if p.anchored {
			target = relPath
		} else {
			target = base
		}

		matched, _ := path.Match(p.raw, target)
		if matched {
			return true
		}
	}
	return false
}
