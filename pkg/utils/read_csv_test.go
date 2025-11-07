package utils_test

import (
	"testing"

	"github.com/algebananazzzzz/planear/pkg/utils"
	"github.com/algebananazzzzz/planear/testutils"
)

func TestReadCSVFile_Success(t *testing.T) {
	dir := testutils.NewTestDir(t)

	content := []byte("name,email\nAlice,alice@example.com\nBob,bob@example.com\n")
	path := testutils.CreateMockFile(t, dir, "valid.csv", content)

	records, err := utils.ReadCSVFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := [][]string{
		{"name", "email"},
		{"Alice", "alice@example.com"},
		{"Bob", "bob@example.com"},
	}
	if len(records) != len(expected) {
		t.Fatalf("expected %d rows, got %d", len(expected), len(records))
	}
	for i := range expected {
		for j := range expected[i] {
			if records[i][j] != expected[i][j] {
				t.Errorf("row %d col %d: expected %s, got %s", i, j, expected[i][j], records[i][j])
			}
		}
	}
}

func TestReadCSVFile_EmptyFile(t *testing.T) {
	dir := testutils.NewTestDir(t)
	path := testutils.CreateMockFile(t, dir, "empty.csv", []byte(""))

	records, err := utils.ReadCSVFile(path)
	if err != nil {
		t.Fatalf("unexpected error for empty file: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected no records, got %d", len(records))
	}
}

func TestReadCSVFile_FileNotFound(t *testing.T) {
	_, err := utils.ReadCSVFile("nonexistent.csv")
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
}

func TestReadCSVFile_InvalidCSV(t *testing.T) {
	dir := testutils.NewTestDir(t)
	// missing comma between values (not well-formed CSV)
	content := []byte("name,email\nAlice alice@example.com\n")
	path := testutils.CreateMockFile(t, dir, "invalid.csv", content)

	_, err := utils.ReadCSVFile(path)
	if err == nil {
		t.Fatal("expected error for invalid CSV, got nil")
	}
}
