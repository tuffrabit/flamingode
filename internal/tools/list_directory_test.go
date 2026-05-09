package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestListDirectory_DefaultsToWorkingDir(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "alpha.txt"), []byte("hello"), 0644)
	_ = os.Mkdir(filepath.Join(tmpDir, "beta"), 0755)

	tool := &ListDirectory{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var items []struct {
		Name string `json:"name"`
		Type string `json:"type"`
		Size int64  `json:"size"`
	}
	if err := json.Unmarshal([]byte(result), &items); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Name != "beta" || items[0].Type != "directory" {
		t.Errorf("expected first item to be beta directory, got %+v", items[0])
	}
	if items[1].Name != "alpha.txt" || items[1].Type != "file" || items[1].Size != 5 {
		t.Errorf("expected second item to be alpha.txt file with size 5, got %+v", items[1])
	}
}

func TestListDirectory_RespectsPathArg(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	_ = os.Mkdir(subDir, 0755)
	_ = os.WriteFile(filepath.Join(subDir, "inner.txt"), []byte("x"), 0644)

	tool := &ListDirectory{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"path":"sub"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var items []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(result), &items); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if len(items) != 1 || items[0].Name != "inner.txt" {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestListDirectory_BlocksTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	tool := &ListDirectory{WorkingDir: tmpDir}
	_, err := tool.GetAction()(context.Background(), `{"path":"../"}`)
	if err == nil {
		t.Fatal("expected error for directory traversal, got nil")
	}
}

func TestListDirectory_ResolvesSymlinks(t *testing.T) {
	tmpDir := t.TempDir()
	realDir := filepath.Join(tmpDir, "real")
	_ = os.Mkdir(realDir, 0755)
	_ = os.WriteFile(filepath.Join(realDir, "file.txt"), []byte("data"), 0644)
	linkDir := filepath.Join(tmpDir, "link")
	_ = os.Symlink(realDir, linkDir)

	tool := &ListDirectory{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"path":"link"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var items []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(result), &items); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if len(items) != 1 || items[0].Name != "file.txt" {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestListDirectory_DotPath(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "root.txt"), []byte("root"), 0644)

	tool := &ListDirectory{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"path":"."}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var items []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(result), &items); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if len(items) != 1 || items[0].Name != "root.txt" {
		t.Fatalf("unexpected items: %+v", items)
	}
}
