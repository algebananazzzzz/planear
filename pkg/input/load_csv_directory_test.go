package input_test

import (
	"testing"

	"github.com/algebananazzzzz/planear/pkg/input"
	"github.com/algebananazzzzz/planear/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestUser struct {
	ID    string `csv:"id"`
	Email string `csv:"email"`
	Score int    `csv:"score"`
}

func TestLoadCSVDirectoryToMap_SingleFile(t *testing.T) {
	dir := testutils.NewTestDir(t)

	// Write single CSV file
	testutils.CreateMockFile(t, dir, "users.csv", []byte("id,email,score\n1,alice@example.com,100\n2,bob@example.com,80\n"))

	records, err := input.LoadCSVDirectoryToMap[TestUser, string](dir, func(u TestUser) string {
		return u.ID
	})
	require.NoError(t, err)
	require.Len(t, records, 2)

	assert.Equal(t, "alice@example.com", records["1"].Email)
	assert.Equal(t, 80, records["2"].Score)
}

func TestLoadCSVDirectoryToMap_CompositeKey(t *testing.T) {
	dir := testutils.NewTestDir(t)

	testutils.CreateMockFile(t, dir, "records.csv", []byte("id,email,score\n1,alice@example.com,100\n2,bob@example.com,80\n"))

	// Composite key: id + email
	records, err := input.LoadCSVDirectoryToMap[TestUser, string](dir, func(u TestUser) string {
		return u.ID + ":" + u.Email
	})
	require.NoError(t, err)
	require.Len(t, records, 2)

	_, ok1 := records["1:alice@example.com"]
	_, ok2 := records["2:bob@example.com"]

	assert.True(t, ok1, "expected composite key '1:alice@example.com' to exist")
	assert.True(t, ok2, "expected composite key '2:bob@example.com' to exist")
}

func TestLoadCSVDirectoryToMap_MultipleFiles(t *testing.T) {
	dir := testutils.NewTestDir(t)

	testutils.CreateMockFile(t, dir, "a.csv", []byte("id,email,score\n1,a@ex.com,10\n"))
	testutils.CreateMockFile(t, dir, "b.csv", []byte("id,email,score\n2,b@ex.com,20\n"))

	records, err := input.LoadCSVDirectoryToMap[TestUser, string](dir, func(u TestUser) string {
		return u.ID
	})
	require.NoError(t, err)
	require.Len(t, records, 2)
	assert.Equal(t, "b@ex.com", records["2"].Email)
}

func TestLoadCSVDirectoryToMap_OverwriteKey(t *testing.T) {
	dir := testutils.NewTestDir(t)

	testutils.CreateMockFile(t, dir, "file1.csv", []byte("id,email,score\n1,a@x.com,100\n"))
	testutils.CreateMockFile(t, dir, "file2.csv", []byte("id,email,score\n1,override@x.com,999\n"))

	records, err := input.LoadCSVDirectoryToMap[TestUser, string](dir, func(u TestUser) string {
		return u.ID
	})
	require.NoError(t, err)
	assert.Equal(t, "override@x.com", records["1"].Email)
	assert.Equal(t, 999, records["1"].Score)
}

func TestLoadCSVDirectoryToMap_IgnoresNonCSV(t *testing.T) {
	dir := testutils.NewTestDir(t)

	testutils.CreateMockFile(t, dir, "a.csv", []byte("id,email,score\n1,x@x.com,1\n"))
	testutils.CreateMockFile(t, dir, "ignore.txt", []byte("not csv"))

	records, err := input.LoadCSVDirectoryToMap[TestUser, string](dir, func(u TestUser) string {
		return u.ID
	})
	require.NoError(t, err)
	require.Len(t, records, 1)
}

func TestLoadCSVDirectoryToMap_InvalidCSVFile(t *testing.T) {
	dir := testutils.NewTestDir(t)

	// Missing required 'score' field
	testutils.CreateMockFile(t, dir, "bad.csv", []byte("id,email\n1,x@x.com\n"))

	_, err := input.LoadCSVDirectoryToMap[TestUser, string](dir, func(u TestUser) string {
		return u.ID
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required column: score")
}

func TestLoadCSVDirectoryToMap_WalkDirError(t *testing.T) {
	invalidDir := "/path/does/not/exist"

	keyFunc := func(r struct{}) string {
		return "key"
	}

	_, err := input.LoadCSVDirectoryToMap[struct{}, string](invalidDir, keyFunc)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no such file or directory")
}
