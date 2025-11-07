package utils_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/algebananazzzzz/planear/pkg/utils"
	"github.com/algebananazzzzz/planear/testutils"
)

type Sample struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func TestWriteJSONFileAndParseJSONFile_Success(t *testing.T) {
	dir := testutils.NewTestDir(t)
	filePath := filepath.Join(dir, "sample.json")

	original := Sample{Name: "Alice", Email: "alice@example.com"}

	err := utils.WriteJSONFile(filePath, "sample json", original)
	if err != nil {
		t.Fatalf("WriteJSONFile failed: %v", err)
	}

	var parsed Sample
	err = utils.ParseJSONFile(filePath, "sample json", &parsed)
	if err != nil {
		t.Fatalf("ParseJSONFile failed: %v", err)
	}

	if parsed != original {
		t.Errorf("parsed data mismatch: expected %+v, got %+v", original, parsed)
	}
}

func TestParseJSONFile_MissingFile(t *testing.T) {
	var parsed Sample
	err := utils.ParseJSONFile("nonexistent.json", "missing file", &parsed)
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
}

func TestParseJSONFile_ReadError(t *testing.T) {
	dir := testutils.NewTestDir(t)
	filePath := filepath.Join(dir, "not-a-file")

	// Create a directory instead of a file
	if err := os.Mkdir(filePath, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	var parsed Sample
	err := utils.ParseJSONFile(filePath, "invalid json", &parsed)

	if err == nil {
		t.Fatal("expected read error, got nil")
	}

	if !strings.Contains(err.Error(), "failed to read") {
		t.Errorf("expected error to contain 'failed to read', got: %v", err)
	}
}

func TestParseJSONFile_InvalidJSON(t *testing.T) {
	dir := testutils.NewTestDir(t)
	badJSON := []byte(`{name:Alice`)

	path := testutils.CreateMockFile(t, dir, "bad.json", badJSON)

	var parsed Sample
	err := utils.ParseJSONFile(path, "invalid json", &parsed)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestWriteJSONFile_MarshalError(t *testing.T) {
	dir := testutils.NewTestDir(t)
	filePath := filepath.Join(dir, "bad.json")

	// Channels can't be marshaled to JSON — will trigger marshal error
	badData := make(chan int)

	err := utils.WriteJSONFile(filePath, "bad json", badData)
	if err == nil {
		t.Fatal("expected marshal error, got nil")
	}
}

func TestWriteJSONFile_WriteError(t *testing.T) {
	dir := testutils.NewTestDir(t)

	// Create a read-only directory
	readonlyDir := filepath.Join(dir, "readonly")
	if err := os.MkdirAll(readonlyDir, 0400); err != nil {
		t.Fatalf("failed to create readonly dir: %v", err)
	}
	defer os.Chmod(readonlyDir, 0755) // Ensure cleanup is possible

	filePath := filepath.Join(readonlyDir, "out.json")
	data := Sample{Name: "ErrorCase", Email: "fail@example.com"}

	err := utils.WriteJSONFile(filePath, "sample json", data)
	if err == nil {
		t.Fatal("expected write error, got nil")
	}

	if !strings.Contains(err.Error(), "failed to write") {
		t.Errorf("expected error message to contain 'failed to write', got: %v", err)
	}
}
