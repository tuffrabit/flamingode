package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGlob_BasicPattern(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "a.go"), []byte("package a\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "b.txt"), []byte("hello\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "c.go"), []byte("package c\n"), 0644)

	tool := &Glob{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"pattern":"*.go","path":"."}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "a.go") {
		t.Fatalf("expected a.go, got: %q", result)
	}
	if !strings.Contains(result, "c.go") {
		t.Fatalf("expected c.go, got: %q", result)
	}
	if strings.Contains(result, "b.txt") {
		t.Fatalf("unexpected b.txt, got: %q", result)
	}
}

func TestGlob_RecursiveDoubleStar(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.Mkdir(filepath.Join(tmpDir, "sub"), 0755)
	_ = os.Mkdir(filepath.Join(tmpDir, "sub", "nested"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "root.md"), []byte("# root\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "sub", "a.md"), []byte("# a\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "sub", "nested", "b.md"), []byte("# b\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "sub", "c.go"), []byte("package c\n"), 0644)

	tool := &Glob{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"pattern":"**/*.md","path":"."}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "root.md") {
		t.Fatalf("expected root.md, got: %q", result)
	}
	if !strings.Contains(result, "sub/a.md") {
		t.Fatalf("expected sub/a.md, got: %q", result)
	}
	if !strings.Contains(result, "sub/nested/b.md") {
		t.Fatalf("expected sub/nested/b.md, got: %q", result)
	}
	if strings.Contains(result, "c.go") {
		t.Fatalf("unexpected c.go, got: %q", result)
	}
}

func TestGlob_SingleLevelStar(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.Mkdir(filepath.Join(tmpDir, "cmd"), 0755)
	_ = os.Mkdir(filepath.Join(tmpDir, "cmd", "app"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "cmd", "app", "main.go"), []byte("package main\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "cmd", "root.go"), []byte("package root\n"), 0644)

	tool := &Glob{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"pattern":"cmd/*/main.go","path":"."}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "cmd/app/main.go") {
		t.Fatalf("expected cmd/app/main.go, got: %q", result)
	}
	if strings.Contains(result, "root.go") {
		t.Fatalf("unexpected root.go, got: %q", result)
	}
}

func TestGlob_NoMatches(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "a.go"), []byte("package a\n"), 0644)

	tool := &Glob{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"pattern":"*.py","path":"."}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "No matches found." {
		t.Fatalf("expected no matches message, got: %q", result)
	}
}

func TestGlob_BlocksTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	tool := &Glob{WorkingDir: tmpDir}
	_, err := tool.GetAction()(context.Background(), `{"pattern":"*.go","path":"../"}`)
	if err == nil || !strings.Contains(err.Error(), "access denied") {
		t.Fatalf("expected access denied error, got: %v", err)
	}
}

func TestGlob_RespectsSkipDirs(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.Mkdir(filepath.Join(tmpDir, "node_modules"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "node_modules", "pkg.go"), []byte("package pkg\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\n"), 0644)

	tool := &Glob{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"pattern":"*.go","path":"."}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "main.go") {
		t.Fatalf("expected main.go, got: %q", result)
	}
	if strings.Contains(result, "node_modules/pkg.go") {
		t.Fatalf("unexpected node_modules/pkg.go, got: %q", result)
	}
}

func TestGlob_RespectsGitignore(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("build/\n"), 0644)
	_ = os.Mkdir(filepath.Join(tmpDir, "build"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "build", "out.go"), []byte("package build\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\n"), 0644)

	tool := &Glob{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"pattern":"*.go","path":"."}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "main.go") {
		t.Fatalf("expected main.go, got: %q", result)
	}
	if strings.Contains(result, "build/out.go") {
		t.Fatalf("unexpected build/out.go, got: %q", result)
	}
}

func TestGlob_SubdirectoryPath(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.Mkdir(filepath.Join(tmpDir, "internal"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "internal", "a.go"), []byte("package internal\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "root.go"), []byte("package root\n"), 0644)

	tool := &Glob{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"pattern":"internal/*.go","path":"."}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "internal/a.go") {
		t.Fatalf("expected internal/a.go, got: %q", result)
	}
	if strings.Contains(result, "root.go") {
		t.Fatalf("unexpected root.go, got: %q", result)
	}
}
