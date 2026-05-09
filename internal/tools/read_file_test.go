package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFile_ReadsEntireFile(t *testing.T) {
	tmpDir := t.TempDir()
	content := "line one\nline two\nline three\n"
	_ = os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte(content), 0644)

	tool := &ReadFile{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"path":"test.txt"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != content {
		t.Fatalf("expected %q, got %q", content, result)
	}
}

func TestReadFile_LineOffsetAndLimit(t *testing.T) {
	tmpDir := t.TempDir()
	content := "one\ntwo\nthree\nfour\nfive\n"
	_ = os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte(content), 0644)

	tool := &ReadFile{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"path":"test.txt","line_offset":2,"limit":2}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "two\nthree\n"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestReadFile_BlocksTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	tool := &ReadFile{WorkingDir: tmpDir}
	_, err := tool.GetAction()(context.Background(), `{"path":"../secret.txt"}`)
	if err == nil {
		t.Fatal("expected error for directory traversal, got nil")
	}
}

func TestReadFile_RejectsBinary(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "binary.bin"), []byte{0x00, 0x01, 0x02, 0xFF}, 0644)

	tool := &ReadFile{WorkingDir: tmpDir}
	_, err := tool.GetAction()(context.Background(), `{"path":"binary.bin"}`)
	if err == nil {
		t.Fatal("expected error for binary file, got nil")
	}
	if !strings.Contains(err.Error(), "binary") {
		t.Fatalf("expected binary error, got: %v", err)
	}
}

func TestReadFile_RejectsOversized(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "huge.txt"), []byte(strings.Repeat("x", maxFileSize+1)), 0644)

	tool := &ReadFile{WorkingDir: tmpDir}
	_, err := tool.GetAction()(context.Background(), `{"path":"huge.txt"}`)
	if err == nil {
		t.Fatal("expected error for oversized file, got nil")
	}
	if !strings.Contains(err.Error(), "maximum size") {
		t.Fatalf("expected size limit error, got: %v", err)
	}
}

func TestReadFile_RejectsDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)

	tool := &ReadFile{WorkingDir: tmpDir}
	_, err := tool.GetAction()(context.Background(), `{"path":"subdir"}`)
	if err == nil {
		t.Fatal("expected error for directory path, got nil")
	}
	if !strings.Contains(err.Error(), "directory") {
		t.Fatalf("expected directory error, got: %v", err)
	}
}

func TestReadFile_ResolvesSymlinks(t *testing.T) {
	tmpDir := t.TempDir()
	realFile := filepath.Join(tmpDir, "real.txt")
	_ = os.WriteFile(realFile, []byte("real content\n"), 0644)
	_ = os.Symlink(realFile, filepath.Join(tmpDir, "link.txt"))

	tool := &ReadFile{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"path":"link.txt"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "real content\n" {
		t.Fatalf("expected 'real content\\n', got %q", result)
	}
}

func TestReadFile_BlocksSymlinkTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	outsideDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(outsideDir, "secret.txt"), []byte("secret\n"), 0644)
	_ = os.Symlink(filepath.Join(outsideDir, "secret.txt"), filepath.Join(tmpDir, "badlink.txt"))

	tool := &ReadFile{WorkingDir: tmpDir}
	_, err := tool.GetAction()(context.Background(), `{"path":"badlink.txt"}`)
	if err == nil {
		t.Fatal("expected error for symlink traversal outside working dir, got nil")
	}
}

func TestReadFile_DefaultLineOffset(t *testing.T) {
	tmpDir := t.TempDir()
	content := "first\nsecond\nthird\n"
	_ = os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte(content), 0644)

	tool := &ReadFile{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"path":"test.txt","limit":1}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "first\n" {
		t.Fatalf("expected 'first\\n', got %q", result)
	}
}

func TestReadFile_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "empty.txt"), []byte{}, 0644)

	tool := &ReadFile{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"path":"empty.txt"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Fatalf("expected empty string, got %q", result)
	}
}
