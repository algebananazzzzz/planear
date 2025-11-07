package apply_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/algebananazzzzz/planear/pkg/core/apply"
	"github.com/algebananazzzzz/planear/pkg/types"
	"github.com/stretchr/testify/assert"
)

type MockRecord struct {
	ID   string
	Name string
}

func TestExecuteOperations(t *testing.T) {
	successAdd := types.RecordAddition[MockRecord]{
		New: MockRecord{ID: "1", Name: "Alice"},
	}
	failAdd := types.RecordAddition[MockRecord]{
		New: MockRecord{ID: "2", Name: "FailAdd"},
	}

	successUpdate := types.RecordUpdate[MockRecord]{
		Old: MockRecord{ID: "3", Name: "OldBob"},
		New: MockRecord{ID: "3", Name: "NewBob"},
	}
	failUpdate := types.RecordUpdate[MockRecord]{
		Old: MockRecord{ID: "4", Name: "OldFail"},
		New: MockRecord{ID: "4", Name: "NewFail"},
	}

	successDelete := types.RecordDeletion[MockRecord]{
		Old: MockRecord{ID: "5", Name: "Charlie"},
	}
	failDelete := types.RecordDeletion[MockRecord]{
		Old: MockRecord{ID: "6", Name: "FailDelete"},
	}

	plan := types.Plan[MockRecord]{
		Additions: []types.RecordAddition[MockRecord]{successAdd, failAdd},
		Updates:   []types.RecordUpdate[MockRecord]{successUpdate, failUpdate},
		Deletions: []types.RecordDeletion[MockRecord]{successDelete, failDelete},
		Ignores: []types.RecordIgnored[MockRecord]{
			{Reason: "Ignored reason", Record: MockRecord{ID: "7", Name: "Ignored"}},
		},
	}

	var executed []string
	params := apply.ExecuteOperationsParams[MockRecord]{
		Plan: plan,
		FormatRecord: func(r MockRecord) string {
			return fmt.Sprintf("ID: %s, Name: %s", r.ID, r.Name)
		},
		FormatKey: func(k string) string { return k },
		OnAdd: func(rec types.RecordAddition[MockRecord]) error {
			if rec.New.Name == "FailAdd" {
				return errors.New("simulated add error")
			}
			executed = append(executed, "add:"+rec.New.ID)
			return nil
		},
		OnUpdate: func(rec types.RecordUpdate[MockRecord]) error {
			if rec.New.Name == "NewFail" {
				return errors.New("simulated update error")
			}
			executed = append(executed, "update:"+rec.New.ID)
			return nil
		},
		OnDelete: func(rec types.RecordDeletion[MockRecord]) error {
			if rec.Old.Name == "FailDelete" {
				return errors.New("simulated delete error")
			}
			executed = append(executed, "delete:"+rec.Old.ID)
			return nil
		},
		OnFinalize: func() error {
			executed = append(executed, "finalize")
			return nil
		},
	}

	report, err := apply.ExecuteOperations(params)
	assert.NoError(t, err)

	assert.Len(t, report.Success.Additions, 1)
	assert.Len(t, report.Failure.Additions, 1)

	assert.Len(t, report.Success.Updates, 1)
	assert.Len(t, report.Failure.Updates, 1)

	assert.Len(t, report.Success.Deletions, 1)
	assert.Len(t, report.Failure.Deletions, 1)

	assert.Len(t, report.Ignores, 1)

	assert.ElementsMatch(t, executed, []string{
		"add:1", "update:3", "delete:5", "finalize",
	})
}

func TestExecuteOperations_FinalizeFailure(t *testing.T) {
	plan := types.Plan[MockRecord]{
		Additions: []types.RecordAddition[MockRecord]{
			{New: MockRecord{ID: "ok", Name: "Should Pass"}},
		},
	}

	params := apply.ExecuteOperationsParams[MockRecord]{
		Plan:         plan,
		FormatRecord: func(r MockRecord) string { return r.Name },
		FormatKey:    func(k string) string { return k },
		OnAdd:        func(types.RecordAddition[MockRecord]) error { return nil },
		OnUpdate:     func(types.RecordUpdate[MockRecord]) error { return nil },
		OnDelete:     func(types.RecordDeletion[MockRecord]) error { return nil },
		OnFinalize: func() error {
			return errors.New("finalize step failed")
		},
	}

	_, err := apply.ExecuteOperations(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "operation failed after 3 retries")
}

func TestExecuteOperations_RetryLoggingWithFormatter(t *testing.T) {
	// Test that retry failures are logged with the formatter in red with attempt count
	plan := types.Plan[MockRecord]{
		Additions: []types.RecordAddition[MockRecord]{
			{New: MockRecord{ID: "1", Name: "FailOnce"}},
		},
	}

	// Capture stdout to verify log output
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	attempts := 0
	params := apply.ExecuteOperationsParams[MockRecord]{
		Plan: plan,
		FormatRecord: func(r MockRecord) string {
			return fmt.Sprintf("[%s:%s]", r.ID, r.Name)
		},
		FormatKey: func(k string) string { return k },
		OnAdd: func(rec types.RecordAddition[MockRecord]) error {
			attempts++
			// Fail on first 2 attempts, succeed on 3rd
			if attempts <= 2 {
				return errors.New("temporary failure")
			}
			return nil
		},
		OnUpdate:     func(types.RecordUpdate[MockRecord]) error { return nil },
		OnDelete:     func(types.RecordDeletion[MockRecord]) error { return nil },
	}

	_, err := apply.ExecuteOperations(params)

	// Restore stdout and read captured output
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify no error (all retries succeeded)
	assert.NoError(t, err)

	// Verify the operation was attempted 3 times
	assert.Equal(t, 3, attempts)

	// Verify retry logs are in the output
	assert.Contains(t, output, "RETRY")
	assert.Contains(t, output, "add")
	assert.Contains(t, output, "[1:FailOnce]")
	assert.Contains(t, output, "attempt 1")
	assert.Contains(t, output, "attempt 2")
	assert.Contains(t, output, "temporary failure")

	// Verify it used red color codes
	assert.Contains(t, output, "\033[31m") // Red color code
}

func TestExecuteOperations_RetryLoggingAllFailures(t *testing.T) {
	// Test that when all retries fail, we get retry attempt logs
	plan := types.Plan[MockRecord]{
		Additions: []types.RecordAddition[MockRecord]{
			{New: MockRecord{ID: "fail", Name: "AlwaysFail"}},
		},
	}

	// Capture stdout
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	params := apply.ExecuteOperationsParams[MockRecord]{
		Plan: plan,
		FormatRecord: func(r MockRecord) string {
			return fmt.Sprintf("Record(%s)", r.ID)
		},
		FormatKey: func(k string) string { return k },
		OnAdd: func(rec types.RecordAddition[MockRecord]) error {
			return errors.New("always fails")
		},
		OnUpdate:     func(types.RecordUpdate[MockRecord]) error { return nil },
		OnDelete:     func(types.RecordDeletion[MockRecord]) error { return nil },
	}

	report, err := apply.ExecuteOperations(params)

	// Restore stdout and read captured output
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify no system error (operation failed but was tracked in report)
	assert.NoError(t, err)

	// Verify the operation was marked as failed in the report
	assert.Len(t, report.Failure.Additions, 1)

	// Verify retry logs are present with attempts and error message
	assert.Contains(t, output, "RETRY")
	assert.Contains(t, output, "add")
	assert.Contains(t, output, "Record(fail)")
	assert.Contains(t, output, "always fails")
	assert.Contains(t, output, "attempt 1")
	assert.Contains(t, output, "attempt 2")
	assert.Contains(t, output, "attempt 3")
	// Verify red color codes were used
	assert.Contains(t, output, "\033[31m") // Red color code
}
