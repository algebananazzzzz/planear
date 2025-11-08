package plan_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/algebananazzzzz/planear/pkg/core/plan"
	"github.com/algebananazzzzz/planear/testutils"
	"github.com/stretchr/testify/require"
)

// Mock record type
type Record struct {
	ID    string `csv:"id"`
	Value string `csv:"value"`
}

func formatRecord(r Record) string { return r.ID + "=" + r.Value }
func formatKey(k string) string    { return "[" + k + "]" }
func extractKey(r Record) string   { return r.ID }
func noopValidator(r Record) error { return nil }

func TestGeneratePlan_Success(t *testing.T) {
	tmpDir := testutils.NewTestDir(t)
	outputPlanFile := filepath.Join(tmpDir, "plan.json")

	local := []Record{
		{ID: "1", Value: "Updated"},
		{ID: "2", Value: "Same"},
	}
	remote := map[string]Record{
		"1": {ID: "1", Value: "Old"},
		"2": {ID: "2", Value: "Same"},
		"3": {ID: "3", Value: "Gone"},
	}

	testutils.WriteCSVFile(t, tmpDir, "plan.csv", local)

	params := plan.GenerateParams[Record]{
		CSVPath:           tmpDir,
		OutputFilePath:    outputPlanFile,
		FormatRecordFunc:  formatRecord,
		FormatKeyFunc:     formatKey,
		ExtractKeyFunc:    extractKey,
		LoadRemoteRecords: func() (map[string]Record, error) { return remote, nil },
		ValidateRecord:    noopValidator,
	}

	result, err := plan.Generate(params)
	require.NoError(t, err)
	require.NotNil(t, result)

	require.Len(t, result.Updates, 1)
	require.Equal(t, "1", result.Updates[0].Key)
	require.Len(t, result.Deletions, 1)
	require.Equal(t, "3", result.Deletions[0].Key)
	require.Empty(t, result.Additions)

	require.True(t, testutils.FileExists(t, outputPlanFile))
}

func TestGeneratePlan_EmptyPlan(t *testing.T) {
	tmpDir := testutils.NewTestDir(t)
	outputPlanFile := filepath.Join(tmpDir, "plan.json")

	records := []Record{
		{ID: "1", Value: "Same"},
		{ID: "2", Value: "SameToo"},
	}
	// Write local CSV input matching remote records exactly
	testutils.WriteCSVFile(t, tmpDir, "plan.csv", records)

	remote := map[string]Record{
		"1": {ID: "1", Value: "Same"},
		"2": {ID: "2", Value: "SameToo"},
	}

	params := plan.GenerateParams[Record]{
		CSVPath:          tmpDir,
		OutputFilePath:   outputPlanFile,
		FormatRecordFunc: func(r Record) string { return r.ID + "=" + r.Value },
		FormatKeyFunc:    func(k string) string { return "[" + k + "]" },
		ExtractKeyFunc:   func(r Record) string { return r.ID },
		LoadRemoteRecords: func() (map[string]Record, error) {
			return remote, nil
		},
		ValidateRecord: testutils.NoopValidator[Record](),
	}

	planResult, err := plan.Generate(params)
	require.NoError(t, err)
	require.NotNil(t, planResult)
	require.True(t, planResult.IsEmpty())

	require.False(t, testutils.FileExists(t, outputPlanFile))
}

func TestGeneratePlan_FailValidation(t *testing.T) {
	tmpDir := testutils.NewTestDir(t)
	testutils.WriteCSVFile(t, tmpDir, "plan.csv", []Record{{ID: "1", Value: "Bad"}})
	outputPlanFile := filepath.Join(tmpDir, "plan.json")

	params := plan.GenerateParams[Record]{
		CSVPath:          tmpDir,
		OutputFilePath:   outputPlanFile,
		FormatRecordFunc: formatRecord,
		FormatKeyFunc:    formatKey,
		ExtractKeyFunc:   extractKey,
		LoadRemoteRecords: func() (map[string]Record, error) {
			return map[string]Record{"1": {ID: "1", Value: "Old"}}, nil
		},
		ValidateRecord: func(r Record) error {
			return errors.New("invalid record")
		},
	}

	result, err := plan.Generate(params)
	require.NoError(t, err)
	require.NotNil(t, result)

	require.Len(t, result.Ignores, 1)
}

func TestGeneratePlan_FailLoadLocal(t *testing.T) {
	params := plan.GenerateParams[Record]{
		CSVPath:           "/invalid/path",
		OutputFilePath:    "plan.json",
		FormatRecordFunc:  formatRecord,
		FormatKeyFunc:     formatKey,
		ExtractKeyFunc:    extractKey,
		LoadRemoteRecords: func() (map[string]Record, error) { return nil, nil },
		ValidateRecord:    noopValidator,
	}

	_, err := plan.Generate(params)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to load local CSV records")
}

func TestGeneratePlan_FailLoadRemote(t *testing.T) {
	tmpDir := testutils.NewTestDir(t)
	testutils.WriteCSVFile(t, tmpDir, "plan.csv", []Record{{ID: "1", Value: "A"}})
	outputPlanFile := filepath.Join(tmpDir, "plan.json")

	params := plan.GenerateParams[Record]{
		CSVPath:           tmpDir,
		OutputFilePath:    outputPlanFile,
		FormatRecordFunc:  formatRecord,
		FormatKeyFunc:     formatKey,
		ExtractKeyFunc:    extractKey,
		LoadRemoteRecords: func() (map[string]Record, error) { return nil, errors.New("db error") },
		ValidateRecord:    noopValidator,
	}

	_, err := plan.Generate(params)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to load remote records")
	require.Contains(t, err.Error(), "db error")
}

func TestGeneratePlan_FailToWritePlanFile_ReadOnlyDir(t *testing.T) {
	tmpDir := testutils.NewTestDir(t)

	readonlyDir := filepath.Join(tmpDir, "readonly")
	err := os.Mkdir(readonlyDir, 0500) // owner: read & execute, no write
	require.NoError(t, err)

	badOutputPath := filepath.Join(readonlyDir, "plan.json")

	testutils.WriteCSVFile(t, tmpDir, "plan.csv", []Record{
		{ID: "1", Value: "Updated"},
	})

	params := plan.GenerateParams[Record]{
		CSVPath:          tmpDir,
		OutputFilePath:   badOutputPath,
		FormatRecordFunc: func(r Record) string { return r.ID + "=" + r.Value },
		FormatKeyFunc:    func(k string) string { return "[" + k + "]" },
		ExtractKeyFunc:   func(r Record) string { return r.ID },
		LoadRemoteRecords: func() (map[string]Record, error) {
			return map[string]Record{"1": {ID: "1", Value: "Old"}}, nil
		},
		ValidateRecord: testutils.NoopValidator[Record](),
	}

	_, err = plan.Generate(params)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to write plan to file")
}

func TestGeneratePlan_EmptyCSVDirectory(t *testing.T) {
	// Test case where CSV directory exists but is empty
	tmpDir := testutils.NewTestDir(t)
	outputPlanFile := filepath.Join(tmpDir, "plan.json")

	// Directory exists but no CSV files
	remote := map[string]Record{
		"1": {ID: "1", Value: "Gone"},
	}

	params := plan.GenerateParams[Record]{
		CSVPath:           tmpDir,
		OutputFilePath:    outputPlanFile,
		FormatRecordFunc:  formatRecord,
		FormatKeyFunc:     formatKey,
		ExtractKeyFunc:    extractKey,
		LoadRemoteRecords: func() (map[string]Record, error) { return remote, nil },
		ValidateRecord:    noopValidator,
	}

	result, err := plan.Generate(params)
	require.NoError(t, err)
	require.NotNil(t, result)
	// Only deletion (remote record with no local record)
	require.Len(t, result.Deletions, 1)
	require.Empty(t, result.Additions)
	require.Empty(t, result.Updates)
}

func TestGeneratePlan_NilExtractKeyFunc(t *testing.T) {
	tmpDir := testutils.NewTestDir(t)
	params := plan.GenerateParams[Record]{
		CSVPath:           tmpDir,
		OutputFilePath:    "plan.json",
		FormatRecordFunc:  formatRecord,
		FormatKeyFunc:     formatKey,
		ExtractKeyFunc:    nil, // Nil function
		LoadRemoteRecords: func() (map[string]Record, error) { return nil, nil },
		ValidateRecord:    noopValidator,
	}

	_, err := plan.Generate(params)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ExtractKeyFunc is required")
}

func TestGeneratePlan_NilLoadRemoteRecords(t *testing.T) {
	tmpDir := testutils.NewTestDir(t)
	params := plan.GenerateParams[Record]{
		CSVPath:           tmpDir,
		OutputFilePath:    "plan.json",
		FormatRecordFunc:  formatRecord,
		FormatKeyFunc:     formatKey,
		ExtractKeyFunc:    extractKey,
		LoadRemoteRecords: nil, // Nil function
		ValidateRecord:    noopValidator,
	}

	_, err := plan.Generate(params)
	require.Error(t, err)
	require.Contains(t, err.Error(), "LoadRemoteRecords is required")
}

func TestGeneratePlan_NilValidateRecord(t *testing.T) {
	tmpDir := testutils.NewTestDir(t)
	params := plan.GenerateParams[Record]{
		CSVPath:           tmpDir,
		OutputFilePath:    "plan.json",
		FormatRecordFunc:  formatRecord,
		FormatKeyFunc:     formatKey,
		ExtractKeyFunc:    extractKey,
		LoadRemoteRecords: func() (map[string]Record, error) { return nil, nil },
		ValidateRecord:    nil, // Nil function
	}

	_, err := plan.Generate(params)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ValidateRecord is required")
}

func TestGeneratePlan_NilFormatRecordFunc(t *testing.T) {
	tmpDir := testutils.NewTestDir(t)
	params := plan.GenerateParams[Record]{
		CSVPath:           tmpDir,
		OutputFilePath:    "plan.json",
		FormatRecordFunc:  nil, // Nil function
		FormatKeyFunc:     formatKey,
		ExtractKeyFunc:    extractKey,
		LoadRemoteRecords: func() (map[string]Record, error) { return nil, nil },
		ValidateRecord:    noopValidator,
	}

	_, err := plan.Generate(params)
	require.Error(t, err)
	require.Contains(t, err.Error(), "FormatRecordFunc is required")
}

func TestGeneratePlan_NilFormatKeyFunc(t *testing.T) {
	tmpDir := testutils.NewTestDir(t)
	params := plan.GenerateParams[Record]{
		CSVPath:           tmpDir,
		OutputFilePath:    "plan.json",
		FormatRecordFunc:  formatRecord,
		FormatKeyFunc:     nil, // Nil function
		ExtractKeyFunc:    extractKey,
		LoadRemoteRecords: func() (map[string]Record, error) { return nil, nil },
		ValidateRecord:    noopValidator,
	}

	_, err := plan.Generate(params)
	require.Error(t, err)
	require.Contains(t, err.Error(), "FormatKeyFunc is required")
}
