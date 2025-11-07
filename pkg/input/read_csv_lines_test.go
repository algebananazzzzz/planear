package input_test

import (
	"testing"

	"github.com/algebananazzzzz/planear/pkg/input"
	"github.com/algebananazzzzz/planear/testutils"
	"github.com/stretchr/testify/require"
)

func TestReadCSVLines_ValidCSV(t *testing.T) {
	dir := testutils.NewTestDir(t)
	content := []byte("id,name,age\n1,Alice,30\n2,Bob,25\n")
	path := testutils.CreateMockFile(t, dir, "valid.csv", content)

	records, err := input.ReadCSVLines(path)
	require.NoError(t, err)
	require.Len(t, records, 3)
	require.Equal(t, []string{"id", "name", "age"}, records[0])
	require.Equal(t, []string{"1", "Alice", "30"}, records[1])
	require.Equal(t, []string{"2", "Bob", "25"}, records[2])
}

func TestReadCSVLines_EmptyFile(t *testing.T) {
	dir := testutils.NewTestDir(t)
	content := []byte("")
	path := testutils.CreateMockFile(t, dir, "empty.csv", content)

	records, err := input.ReadCSVLines(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty CSV file")
	require.Nil(t, records)
}

func TestReadCSVLines_NonExistentFile(t *testing.T) {
	path := "nonexistent.csv"

	records, err := input.ReadCSVLines(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to open CSV file")
	require.Nil(t, records)
}

func TestReadCSVLines_InvalidCSVFormat(t *testing.T) {
	dir := testutils.NewTestDir(t)
	// Malformed CSV (unescaped quote)
	content := []byte("id,name\n1,\"Alice\n2,Bob\n")
	path := testutils.CreateMockFile(t, dir, "malformed.csv", content)

	records, err := input.ReadCSVLines(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse CSV file")
	require.Nil(t, records)
}
