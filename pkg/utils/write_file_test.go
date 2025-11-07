package utils_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/algebananazzzzz/planear/pkg/utils"
	"github.com/algebananazzzzz/planear/testutils"
)

func TestWriteToFile_Success(t *testing.T) {
	tempDir := testutils.NewTestDir(t)
	filePath := filepath.Join(tempDir, "nested", "file.txt")
	content := []byte("hello test")
	desc := "test file"

	err := utils.WriteToFile(filePath, desc, content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Validate file exists and content is correct
	if !testutils.FileEqualsContent(t, filePath, content) {
		t.Fatal("expected file to exist")
	}

	actual := testutils.ReadFile(t, filePath)
	if string(actual) != string(content) {
		t.Errorf("expected %s, got %s", content, actual)
	}
}

func TestWriteToFile_CreateDirError(t *testing.T) {
	// On Unix-like systems, /dev/null is a file, so making a subdirectory under it will fail
	invalidPath := filepath.Join("/dev/null", "subdir", "file.txt")
	content := []byte("should fail")
	desc := "invalid path test"

	err := utils.WriteToFile(invalidPath, desc, content)
	if err == nil {
		t.Fatal("expected error due to directory creation failure, got nil")
	}
}

func TestWriteToFile_EmptyContent(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "skip.txt")

	err := utils.WriteToFile(filePath, "empty", []byte{})
	if err != nil {
		t.Fatalf("unexpected error on empty content: %v", err)
	}

	if testutils.FileExists(t, filePath) {
		t.Errorf("file should not have been written")
	}
}

func TestWriteToFile_InvalidPath(t *testing.T) {
	// Invalid path using an illegal character on most OSes
	invalidPath := string([]byte{0x00}) // NUL character
	err := utils.WriteToFile(invalidPath, "bad path", []byte("test"))
	if err == nil {
		t.Fatal("expected error for invalid file path, got nil")
	}
}

// TestCreateFileWriter_Success verifies a file is created successfully and is writable.
func TestCreateFileWriter_Success(t *testing.T) {
	dir := testutils.NewTestDir(t)
	filePath := filepath.Join(dir, "nested", "test.txt")

	writer, err := utils.CreateFileWriter(filePath)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	data := "hello world"
	if _, err := writer.Write([]byte(data)); err != nil {
		t.Fatalf("failed to write to file: %v", err)
	}

	content := testutils.ReadFile(t, filePath)
	if string(content) != data {
		t.Errorf("expected content %q, got %q", data, string(content))
	}
}

// TestCreateFileWriter_CreateDirError simulates an error during directory creation.
func TestCreateFileWriter_CreateDirError(t *testing.T) {
	// Using an invalid directory path to force a permission error (on Unix-like systems)
	filePath := "/root/forbidden/test.txt"

	_, err := utils.CreateFileWriter(filePath)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "create directory") {
		t.Errorf("expected error message to contain 'create directory', got: %v", err)
	}
}

// TestCreateFileWriter_CreateFileError simulates an error during file creation.
func TestCreateFileWriter_CreateFileError(t *testing.T) {
	dir := testutils.NewTestDir(t)

	// Simulate file creation failure by creating a read-only directory
	readonlyDir := filepath.Join(dir, "readonly")
	if err := os.MkdirAll(readonlyDir, 0400); err != nil {
		t.Fatalf("failed to create readonly dir: %v", err)
	}
	defer func() {
		// Reset permissions to allow cleanup
		_ = os.Chmod(readonlyDir, 0755)
	}()

	filePath := filepath.Join(readonlyDir, "test.txt")
	_, err := utils.CreateFileWriter(filePath)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "create file") {
		t.Errorf("expected error message to contain 'create file', got: %v", err)
	}
}
