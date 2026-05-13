package tools

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ResolveWorkingDirPath resolves relPath against workingDir, follows symlinks,
// and ensures the final path is within the working directory tree.
func ResolveWorkingDirPath(workingDir, relPath string) (string, error) {
	absPath, err := filepath.Abs(filepath.Join(workingDir, relPath))
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", fmt.Errorf("cannot access path: %w", err)
	}

	cleanPath := filepath.Clean(resolvedPath)
	cleanWorkingDir := filepath.Clean(workingDir)

	if cleanPath != cleanWorkingDir && !strings.HasPrefix(cleanPath, cleanWorkingDir+string(filepath.Separator)) {
		return "", fmt.Errorf("access denied: path is outside the working directory")
	}

	return cleanPath, nil
}
