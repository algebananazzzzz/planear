package testutils

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/gocarina/gocsv"
)

// NewTestDir creates a temporary directory that is automatically cleaned up by the test framework.
// It returns the absolute path to the temporary test directory.
func NewTestDir(t *testing.T) string {
	t.Helper()

	tempDir := t.TempDir()
	abs, err := filepath.Abs(tempDir)
	if err != nil {
		t.Fatalf("failed to get absolute path for temp dir: %v", err)
	}
	return abs
}

// CreateMockFile creates a file with content, including all parent directories.
func CreateMockFile(t *testing.T, baseDir, relPath string, content []byte) string {
	t.Helper()

	fullPath := filepath.Join(baseDir, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), os.ModePerm); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	if err := os.WriteFile(fullPath, content, 0644); err != nil {
		t.Fatalf("failed to write mock file: %v", err)
	}

	return fullPath
}

// WriteCSVFile writes a slice of structs to a CSV file using gocsv and returns the full file path.
func WriteCSVFile[T any](t *testing.T, baseDir, filename string, records []T) string {
	t.Helper()

	fullPath := filepath.Join(baseDir, filename)
	file, err := os.Create(fullPath)
	if err != nil {
		t.Fatalf("failed to create CSV file: %v", err)
	}
	defer file.Close()

	if err := gocsv.MarshalFile(&records, file); err != nil {
		t.Fatalf("failed to marshal records to CSV: %v", err)
	}

	return fullPath
}

// WriteJSONFile writes a JSON struct to a file and returns the full file path.
func WriteJSONFile[T any](t *testing.T, baseDir, filename string, records T) string {
	t.Helper()

	fullPath := filepath.Join(baseDir, filename)
	file, err := os.Create(fullPath)
	if err != nil {
		t.Fatalf("failed to create JSON file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(records); err != nil {
		t.Fatalf("failed to encode records to JSON: %v", err)
	}

	return fullPath
}

// ReadFile reads a file and returns its contents.
func ReadFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file %s: %v", path, err)
	}
	return data
}

// FilesEqual checks if two files contain the same content.
func FilesEqual(t *testing.T, path1, path2 string) bool {
	t.Helper()

	data1 := ReadFile(t, path1)
	data2 := ReadFile(t, path2)

	return string(data1) == string(data2)
}

// FileEqualsContent checks if the file at path contains exactly the given content.
func FileEqualsContent(t *testing.T, path string, expected []byte) bool {
	t.Helper()

	actual, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file %s: %v", path, err)
	}

	return string(actual) == string(expected)
}

// FileExists checks if the file exists and is not a directory.
func FileExists(t *testing.T, path string) bool {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// RemoveAll cleans up a directory tree.
func RemoveAll(t *testing.T, path string) {
	t.Helper()

	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("failed to remove path %s: %v", path, err)
	}
}

// ListFiles lists all files under a directory (recursive).
func ListFiles(t *testing.T, root string) []string {
	t.Helper()

	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			t.Fatalf("walk error: %v", err)
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk dir: %v", err)
	}
	return files
}
