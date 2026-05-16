package apply_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
	"time"

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
	parallelism := 1 // Sequential execution to verify order
	params := apply.ExecuteOperationsParams[MockRecord]{
		Plan: plan,
		FormatRecord: func(r MockRecord) string {
			return fmt.Sprintf("ID: %s, Name: %s", r.ID, r.Name)
		},
		FormatKey:       func(k string) string { return k },
		Parallelization: &parallelism,
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

	// Verify execution order: deletions first, then additions and updates, then finalize
	assert.Equal(t, []string{
		"delete:5", "add:1", "update:3", "finalize",
	}, executed)
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
		OnUpdate: func(types.RecordUpdate[MockRecord]) error { return nil },
		OnDelete: func(types.RecordDeletion[MockRecord]) error { return nil },
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
		OnUpdate: func(types.RecordUpdate[MockRecord]) error { return nil },
		OnDelete: func(types.RecordDeletion[MockRecord]) error { return nil },
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

func TestExecuteOperations_NilFormatRecord(t *testing.T) {
	plan := types.Plan[MockRecord]{}
	params := apply.ExecuteOperationsParams[MockRecord]{
		Plan:         plan,
		FormatRecord: nil, // Nil function
		FormatKey:    func(k string) string { return k },
		OnAdd:        func(types.RecordAddition[MockRecord]) error { return nil },
		OnUpdate:     func(types.RecordUpdate[MockRecord]) error { return nil },
		OnDelete:     func(types.RecordDeletion[MockRecord]) error { return nil },
	}

	_, err := apply.ExecuteOperations(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "FormatRecord is required")
}

func TestExecuteOperations_NilFormatKey(t *testing.T) {
	plan := types.Plan[MockRecord]{}
	params := apply.ExecuteOperationsParams[MockRecord]{
		Plan:         plan,
		FormatRecord: func(r MockRecord) string { return r.Name },
		FormatKey:    nil, // Nil function
		OnAdd:        func(types.RecordAddition[MockRecord]) error { return nil },
		OnUpdate:     func(types.RecordUpdate[MockRecord]) error { return nil },
		OnDelete:     func(types.RecordDeletion[MockRecord]) error { return nil },
	}

	_, err := apply.ExecuteOperations(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "FormatKey is required")
}

func TestExecuteOperations_NilOnAdd(t *testing.T) {
	plan := types.Plan[MockRecord]{}
	params := apply.ExecuteOperationsParams[MockRecord]{
		Plan:         plan,
		FormatRecord: func(r MockRecord) string { return r.Name },
		FormatKey:    func(k string) string { return k },
		OnAdd:        nil, // Nil function
		OnUpdate:     func(types.RecordUpdate[MockRecord]) error { return nil },
		OnDelete:     func(types.RecordDeletion[MockRecord]) error { return nil },
	}

	_, err := apply.ExecuteOperations(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OnAdd is required")
}

func TestExecuteOperations_NilOnUpdate(t *testing.T) {
	plan := types.Plan[MockRecord]{}
	params := apply.ExecuteOperationsParams[MockRecord]{
		Plan:         plan,
		FormatRecord: func(r MockRecord) string { return r.Name },
		FormatKey:    func(k string) string { return k },
		OnAdd:        func(types.RecordAddition[MockRecord]) error { return nil },
		OnUpdate:     nil, // Nil function
		OnDelete:     func(types.RecordDeletion[MockRecord]) error { return nil },
	}

	_, err := apply.ExecuteOperations(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OnUpdate is required")
}

func TestExecuteOperations_NilOnDelete(t *testing.T) {
	plan := types.Plan[MockRecord]{}
	params := apply.ExecuteOperationsParams[MockRecord]{
		Plan:         plan,
		FormatRecord: func(r MockRecord) string { return r.Name },
		FormatKey:    func(k string) string { return k },
		OnAdd:        func(types.RecordAddition[MockRecord]) error { return nil },
		OnUpdate:     func(types.RecordUpdate[MockRecord]) error { return nil },
		OnDelete:     nil, // Nil function
	}

	_, err := apply.ExecuteOperations(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OnDelete is required")
}

func TestExecuteOperations_LayeredPath_RespectsOrder(t *testing.T) {
	plan := types.Plan[MockRecord]{
		Additions: []types.RecordAddition[MockRecord]{
			{Key: "parent", New: MockRecord{ID: "parent"}},
			{Key: "child", New: MockRecord{ID: "child"}},
		},
		Layers: [][]types.LayerOp{
			{{Kind: types.LayerOpAdd, Key: "parent"}},
			{{Kind: types.LayerOpAdd, Key: "child"}},
		},
	}

	var mu sync.Mutex
	var order []string
	params := apply.ExecuteOperationsParams[MockRecord]{
		Plan:         plan,
		FormatRecord: func(r MockRecord) string { return r.ID },
		FormatKey:    func(k string) string { return k },
		OnAdd: func(rec types.RecordAddition[MockRecord]) error {
			mu.Lock()
			order = append(order, rec.New.ID)
			mu.Unlock()
			return nil
		},
		OnUpdate: func(types.RecordUpdate[MockRecord]) error { return nil },
		OnDelete: func(types.RecordDeletion[MockRecord]) error { return nil },
	}

	report, err := apply.ExecuteOperations(params)
	assert.NoError(t, err)
	assert.Equal(t, []string{"parent", "child"}, order)
	assert.Len(t, report.Success.Additions, 2)
}

func TestExecuteOperations_LayeredPath_MultisetMismatch_MissingOp(t *testing.T) {
	plan := types.Plan[MockRecord]{
		Additions: []types.RecordAddition[MockRecord]{
			{Key: "A", New: MockRecord{ID: "A"}},
			{Key: "B", New: MockRecord{ID: "B"}},
		},
		Layers: [][]types.LayerOp{
			// Missing "B" — multiset violation.
			{{Kind: types.LayerOpAdd, Key: "A"}},
		},
	}

	called := false
	params := apply.ExecuteOperationsParams[MockRecord]{
		Plan:         plan,
		FormatRecord: func(r MockRecord) string { return r.ID },
		FormatKey:    func(k string) string { return k },
		OnAdd: func(types.RecordAddition[MockRecord]) error {
			called = true
			return nil
		},
		OnUpdate: func(types.RecordUpdate[MockRecord]) error { return nil },
		OnDelete: func(types.RecordDeletion[MockRecord]) error { return nil },
	}

	_, err := apply.ExecuteOperations(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "multiset")
	assert.False(t, called, "no OnAdd must run when multiset check fails")
}

func TestExecuteOperations_LayeredPath_MultisetMismatch_UnknownOp(t *testing.T) {
	plan := types.Plan[MockRecord]{
		Additions: []types.RecordAddition[MockRecord]{
			{Key: "A", New: MockRecord{ID: "A"}},
		},
		Layers: [][]types.LayerOp{
			{{Kind: types.LayerOpAdd, Key: "A"}, {Kind: types.LayerOpAdd, Key: "phantom"}},
		},
	}

	called := false
	params := apply.ExecuteOperationsParams[MockRecord]{
		Plan:         plan,
		FormatRecord: func(r MockRecord) string { return r.ID },
		FormatKey:    func(k string) string { return k },
		OnAdd: func(types.RecordAddition[MockRecord]) error {
			called = true
			return nil
		},
		OnUpdate: func(types.RecordUpdate[MockRecord]) error { return nil },
		OnDelete: func(types.RecordDeletion[MockRecord]) error { return nil },
	}

	_, err := apply.ExecuteOperations(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown op")
	assert.False(t, called)
}

func TestExecuteOperations_NilLayers_TakesFlatPath(t *testing.T) {
	plan := types.Plan[MockRecord]{
		Additions: []types.RecordAddition[MockRecord]{
			{Key: "X", New: MockRecord{ID: "X"}},
		},
	}
	called := false
	params := apply.ExecuteOperationsParams[MockRecord]{
		Plan:         plan,
		FormatRecord: func(r MockRecord) string { return r.ID },
		FormatKey:    func(k string) string { return k },
		OnAdd: func(types.RecordAddition[MockRecord]) error {
			called = true
			return nil
		},
		OnUpdate: func(types.RecordUpdate[MockRecord]) error { return nil },
		OnDelete: func(types.RecordDeletion[MockRecord]) error { return nil },
	}
	_, err := apply.ExecuteOperations(params)
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestFinalizeOnSuccess_SkipsWhenAnyFailure(t *testing.T) {
	finalizeCalled := false
	plan := types.Plan[MockRecord]{
		Additions: []types.RecordAddition[MockRecord]{
			{Key: "fail", New: MockRecord{ID: "fail", Name: "always-fails"}},
			{Key: "skipped", New: MockRecord{ID: "skipped"}},
		},
		Layers: [][]types.LayerOp{
			{{Kind: types.LayerOpAdd, Key: "fail"}},
			{{Kind: types.LayerOpAdd, Key: "skipped"}},
		},
	}
	_, _ = apply.ExecuteOperations(apply.ExecuteOperationsParams[MockRecord]{
		Plan:         plan,
		FormatRecord: func(r MockRecord) string { return r.ID },
		FormatKey:    func(k string) string { return k },
		FinalizeOn:   types.FinalizeOnSuccess,
		OnAdd: func(rec types.RecordAddition[MockRecord]) error {
			if rec.New.Name == "always-fails" {
				return errors.New("synthetic")
			}
			return nil
		},
		OnUpdate:   func(types.RecordUpdate[MockRecord]) error { return nil },
		OnDelete:   func(types.RecordDeletion[MockRecord]) error { return nil },
		OnFinalize: func() error { finalizeCalled = true; return nil },
	})
	assert.False(t, finalizeCalled, "FinalizeOnSuccess must skip finalize when any op failed")
}

func TestFinalizeOnAnySuccess_RunsWhenAtLeastOneSucceeds(t *testing.T) {
	finalizeCalled := false
	plan := types.Plan[MockRecord]{
		Additions: []types.RecordAddition[MockRecord]{
			{Key: "ok", New: MockRecord{ID: "ok"}},
			{Key: "fail", New: MockRecord{ID: "fail", Name: "always-fails"}},
		},
		Layers: [][]types.LayerOp{
			{{Kind: types.LayerOpAdd, Key: "ok"}},
			{{Kind: types.LayerOpAdd, Key: "fail"}},
		},
	}
	_, _ = apply.ExecuteOperations(apply.ExecuteOperationsParams[MockRecord]{
		Plan:         plan,
		FormatRecord: func(r MockRecord) string { return r.ID },
		FormatKey:    func(k string) string { return k },
		FinalizeOn:   types.FinalizeOnAnySuccess,
		OnAdd: func(rec types.RecordAddition[MockRecord]) error {
			if rec.New.Name == "always-fails" {
				return errors.New("synthetic")
			}
			return nil
		},
		OnUpdate:   func(types.RecordUpdate[MockRecord]) error { return nil },
		OnDelete:   func(types.RecordDeletion[MockRecord]) error { return nil },
		OnFinalize: func() error { finalizeCalled = true; return nil },
	})
	assert.True(t, finalizeCalled, "FinalizeOnAnySuccess must run finalize when at least one op succeeded")
}

func TestFinalizeOnAnySuccess_SkipsWhenZeroProgress(t *testing.T) {
	finalizeCalled := false
	plan := types.Plan[MockRecord]{
		Additions: []types.RecordAddition[MockRecord]{
			{Key: "fail", New: MockRecord{ID: "fail", Name: "always-fails"}},
		},
		Layers: [][]types.LayerOp{
			{{Kind: types.LayerOpAdd, Key: "fail"}},
		},
	}
	_, _ = apply.ExecuteOperations(apply.ExecuteOperationsParams[MockRecord]{
		Plan:         plan,
		FormatRecord: func(r MockRecord) string { return r.ID },
		FormatKey:    func(k string) string { return k },
		FinalizeOn:   types.FinalizeOnAnySuccess,
		OnAdd:        func(rec types.RecordAddition[MockRecord]) error { return errors.New("synthetic") },
		OnUpdate:     func(types.RecordUpdate[MockRecord]) error { return nil },
		OnDelete:     func(types.RecordDeletion[MockRecord]) error { return nil },
		OnFinalize:   func() error { finalizeCalled = true; return nil },
	})
	assert.False(t, finalizeCalled, "FinalizeOnAnySuccess must skip finalize when zero ops succeeded")
}

func TestFinalizeAlways_DefaultRunsEvenOnFailure(t *testing.T) {
	finalizeCalled := false
	plan := types.Plan[MockRecord]{
		Additions: []types.RecordAddition[MockRecord]{
			{Key: "fail", New: MockRecord{ID: "fail", Name: "always-fails"}},
		},
		Layers: [][]types.LayerOp{
			{{Kind: types.LayerOpAdd, Key: "fail"}},
		},
		// FinalizeOn intentionally unset (zero value = FinalizeAlways)
	}
	_, _ = apply.ExecuteOperations(apply.ExecuteOperationsParams[MockRecord]{
		Plan:         plan,
		FormatRecord: func(r MockRecord) string { return r.ID },
		FormatKey:    func(k string) string { return k },
		OnAdd:        func(rec types.RecordAddition[MockRecord]) error { return errors.New("synthetic") },
		OnUpdate:     func(types.RecordUpdate[MockRecord]) error { return nil },
		OnDelete:     func(types.RecordDeletion[MockRecord]) error { return nil },
		OnFinalize:   func() error { finalizeCalled = true; return nil },
	})
	assert.True(t, finalizeCalled, "zero-value FinalizeOn (= FinalizeAlways) must always run finalize")
}

func TestFinalizeOn_AppliesToFlatPath(t *testing.T) {
	// Flat path (Layers == nil) must also honor FinalizeOn.
	finalizeCalled := false
	plan := types.Plan[MockRecord]{
		Additions: []types.RecordAddition[MockRecord]{
			{Key: "fail", New: MockRecord{ID: "fail", Name: "always-fails"}},
		},
	}
	_, _ = apply.ExecuteOperations(apply.ExecuteOperationsParams[MockRecord]{
		Plan:         plan,
		FormatRecord: func(r MockRecord) string { return r.ID },
		FormatKey:    func(k string) string { return k },
		FinalizeOn:   types.FinalizeOnSuccess,
		OnAdd:        func(rec types.RecordAddition[MockRecord]) error { return errors.New("synthetic") },
		OnUpdate:     func(types.RecordUpdate[MockRecord]) error { return nil },
		OnDelete:     func(types.RecordDeletion[MockRecord]) error { return nil },
		OnFinalize:   func() error { finalizeCalled = true; return nil },
	})
	assert.False(t, finalizeCalled, "FinalizeOnSuccess must gate finalize on the flat path as well")
}

func TestExecuteOperations_LayeredPath_LayerBarrier(t *testing.T) {
	plan := types.Plan[MockRecord]{
		Additions: []types.RecordAddition[MockRecord]{
			{Key: "L0a", New: MockRecord{ID: "L0a"}},
			{Key: "L0b", New: MockRecord{ID: "L0b"}},
			{Key: "L1", New: MockRecord{ID: "L1"}},
		},
		Layers: [][]types.LayerOp{
			{{Kind: types.LayerOpAdd, Key: "L0a"}, {Kind: types.LayerOpAdd, Key: "L0b"}},
			{{Kind: types.LayerOpAdd, Key: "L1"}},
		},
	}

	var mu sync.Mutex
	inFlight := map[string]bool{}
	finished := map[string]bool{}
	violations := 0

	parallelism := 4
	params := apply.ExecuteOperationsParams[MockRecord]{
		Plan:            plan,
		FormatRecord:    func(r MockRecord) string { return r.ID },
		FormatKey:       func(k string) string { return k },
		Parallelization: &parallelism,
		OnAdd: func(rec types.RecordAddition[MockRecord]) error {
			mu.Lock()
			inFlight[rec.New.ID] = true
			if rec.New.ID == "L1" {
				if inFlight["L0a"] && !finished["L0a"] {
					violations++
				}
				if inFlight["L0b"] && !finished["L0b"] {
					violations++
				}
			}
			mu.Unlock()

			if rec.New.ID == "L0a" || rec.New.ID == "L0b" {
				time.Sleep(50 * time.Millisecond)
			}

			mu.Lock()
			finished[rec.New.ID] = true
			mu.Unlock()
			return nil
		},
		OnUpdate: func(types.RecordUpdate[MockRecord]) error { return nil },
		OnDelete: func(types.RecordDeletion[MockRecord]) error { return nil },
	}

	_, err := apply.ExecuteOperations(params)
	assert.NoError(t, err)
	assert.Equal(t, 0, violations, "L1 must not start until both L0 ops finish")
	assert.True(t, finished["L1"])
}

func TestExecuteOperations_LayeredPath_FailureCascadesToSkipped(t *testing.T) {
	plan := types.Plan[MockRecord]{
		Additions: []types.RecordAddition[MockRecord]{
			{Key: "fails", New: MockRecord{ID: "fails", Name: "always-fails"}},
			{Key: "skip1", New: MockRecord{ID: "skip1"}},
			{Key: "skip2", New: MockRecord{ID: "skip2"}},
		},
		Updates: []types.RecordUpdate[MockRecord]{
			{Key: "skipUpd", Old: MockRecord{ID: "skipUpd"}, New: MockRecord{ID: "skipUpd", Name: "new"}},
		},
		Deletions: []types.RecordDeletion[MockRecord]{
			{Key: "skipDel", Old: MockRecord{ID: "skipDel"}},
		},
		Layers: [][]types.LayerOp{
			{{Kind: types.LayerOpAdd, Key: "fails"}},
			{{Kind: types.LayerOpAdd, Key: "skip1"}, {Kind: types.LayerOpAdd, Key: "skip2"}},
			{{Kind: types.LayerOpUpdate, Key: "skipUpd"}},
			{{Kind: types.LayerOpDelete, Key: "skipDel"}},
		},
	}

	var mu sync.Mutex
	var attempted []string
	record := func(s string) { mu.Lock(); attempted = append(attempted, s); mu.Unlock() }

	params := apply.ExecuteOperationsParams[MockRecord]{
		Plan:         plan,
		FormatRecord: func(r MockRecord) string { return r.ID },
		FormatKey:    func(k string) string { return k },
		OnAdd: func(rec types.RecordAddition[MockRecord]) error {
			record("add:" + rec.New.ID)
			if rec.New.Name == "always-fails" {
				return errors.New("synthetic")
			}
			return nil
		},
		OnUpdate: func(rec types.RecordUpdate[MockRecord]) error {
			record("upd:" + rec.New.ID)
			return nil
		},
		OnDelete: func(rec types.RecordDeletion[MockRecord]) error {
			record("del:" + rec.Old.ID)
			return nil
		},
	}

	report, err := apply.ExecuteOperations(params)
	assert.NoError(t, err)
	assert.NotContains(t, attempted, "add:skip1")
	assert.NotContains(t, attempted, "add:skip2")
	assert.NotContains(t, attempted, "upd:skipUpd")
	assert.NotContains(t, attempted, "del:skipDel")
	assert.Len(t, report.Failure.Additions, 1)
	assert.Equal(t, "fails", report.Failure.Additions[0].Key)
	assert.Len(t, report.Skipped.Additions, 2)
	assert.Len(t, report.Skipped.Updates, 1)
	assert.Equal(t, "skipUpd", report.Skipped.Updates[0].Key)
	assert.Len(t, report.Skipped.Deletions, 1)
	assert.Equal(t, "skipDel", report.Skipped.Deletions[0].Key)
}
