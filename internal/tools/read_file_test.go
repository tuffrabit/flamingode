package tools

import (
	"context"
	"fmt"
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

func TestReadFile_TruncatesOversized(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "huge.txt"), []byte(strings.Repeat("x", DefaultMaxFileSize+1)), 0644)

	tool := &ReadFile{WorkingDir: tmpDir}
	result, err := tool.GetAction()(context.Background(), `{"path":"huge.txt"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "[File truncated:") {
		t.Fatalf("expected truncation notice, got: %q", result)
	}
	// Content should be DefaultMaxFileSize bytes (all "x" on one line plus newline) plus the notice.
	expectedContentLen := DefaultMaxFileSize + 1 // "x" * maxSize + "\n"
	if len(result) != expectedContentLen+len(fmt.Sprintf("[File truncated: exceeded max size of %d bytes]\n", DefaultMaxFileSize)) {
		t.Fatalf("unexpected result length: got %d, expected around %d", len(result), expectedContentLen)
	}
}

func TestReadFile_CustomMaxSizeTruncates(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "medium.txt"), []byte(strings.Repeat("x", 500)), 0644)

	tool := &ReadFile{WorkingDir: tmpDir, MaxSize: 400}
	result, err := tool.GetAction()(context.Background(), `{"path":"medium.txt"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "[File truncated:") {
		t.Fatalf("expected truncation notice, got: %q", result)
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
