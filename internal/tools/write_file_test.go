package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFile_WritesNewFile(t *testing.T) {
	tmpDir := t.TempDir()
	tool := &WriteFile{WorkingDir: tmpDir}

	result, err := tool.GetAction()(context.Background(), `{"path":"new.txt","contents":"hello world"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "11" {
		t.Fatalf("expected bytes written '11', got %q", result)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "new.txt"))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(content) != "hello world" {
		t.Fatalf("expected 'hello world', got %q", string(content))
	}
}

func TestWriteFile_OverwritesExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "existing.txt"), []byte("old"), 0644)

	tool := &WriteFile{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"path":"existing.txt","contents":"new content"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "11" {
		t.Fatalf("expected bytes written '11', got %q", result)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "existing.txt"))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(content) != "new content" {
		t.Fatalf("expected 'new content', got %q", string(content))
	}
}

func TestWriteFile_CreatesMissingParentDirs(t *testing.T) {
	tmpDir := t.TempDir()
	tool := &WriteFile{WorkingDir: tmpDir}

	result, err := tool.GetAction()(context.Background(), `{"path":"a/b/c/deep.txt","contents":"deep"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "4" {
		t.Fatalf("expected bytes written '4', got %q", result)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "a", "b", "c", "deep.txt"))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(content) != "deep" {
		t.Fatalf("expected 'deep', got %q", string(content))
	}
}

func TestWriteFile_BlocksTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	tool := &WriteFile{WorkingDir: tmpDir}

	_, err := tool.GetAction()(context.Background(), `{"path":"../secret.txt","contents":"bad"}`)
	if err == nil {
		t.Fatal("expected error for directory traversal, got nil")
	}
}

func TestWriteFile_BlocksSymlinkTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	outsideDir := t.TempDir()
	_ = os.Symlink(outsideDir, filepath.Join(tmpDir, "badlink"))

	tool := &WriteFile{WorkingDir: tmpDir}
	_, err := tool.GetAction()(context.Background(), `{"path":"badlink/secret.txt","contents":"bad"}`)
	if err == nil {
		t.Fatal("expected error for symlink traversal outside working dir, got nil")
	}
}

func TestWriteFile_ResolvesValidSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	realDir := filepath.Join(tmpDir, "real")
	_ = os.Mkdir(realDir, 0755)
	_ = os.Symlink(realDir, filepath.Join(tmpDir, "link"))

	tool := &WriteFile{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"path":"link/file.txt","contents":"via symlink"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "11" {
		t.Fatalf("expected bytes written '11', got %q", result)
	}

	content, err := os.ReadFile(filepath.Join(realDir, "file.txt"))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(content) != "via symlink" {
		t.Fatalf("expected 'via symlink', got %q", string(content))
	}
}

func TestWriteFile_EmptyContents(t *testing.T) {
	tmpDir := t.TempDir()
	tool := &WriteFile{WorkingDir: tmpDir}

	result, err := tool.GetAction()(context.Background(), `{"path":"empty.txt","contents":""}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "0" {
		t.Fatalf("expected bytes written '0', got %q", result)
	}

	info, err := os.Stat(filepath.Join(tmpDir, "empty.txt"))
	if err != nil {
		t.Fatalf("failed to stat written file: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("expected empty file, got size %d", info.Size())
	}
}

func TestWriteFile_RequiresPath(t *testing.T) {
	tmpDir := t.TempDir()
	tool := &WriteFile{WorkingDir: tmpDir}

	_, err := tool.GetAction()(context.Background(), `{"contents":"no path"}`)
	if err == nil {
		t.Fatal("expected error when path is missing, got nil")
	}
}

func TestWriteFile_RejectsAbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	tool := &WriteFile{WorkingDir: tmpDir}

	_, err := tool.GetAction()(context.Background(), `{"path":"/etc/passwd","contents":"bad"}`)
	if err == nil {
		t.Fatal("expected error for absolute path, got nil")
	}
}
