package apply_test

import (
	"fmt"
	"testing"

	"github.com/algebananazzzzz/planear/pkg/core/apply"
	"github.com/algebananazzzzz/planear/pkg/types"
	"github.com/algebananazzzzz/planear/testutils"
	"github.com/stretchr/testify/assert"
)

type Dummy struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func TestRun_SuccessfulExecution(t *testing.T) {
	plan := types.Plan[Dummy]{
		Additions: []types.RecordAddition[Dummy]{
			{New: Dummy{ID: "1", Name: "AddMe"}},
		},
		Updates: []types.RecordUpdate[Dummy]{
			{Old: Dummy{ID: "2", Name: "Old"}, New: Dummy{ID: "2", Name: "New"}},
		},
		Deletions: []types.RecordDeletion[Dummy]{
			{Old: Dummy{ID: "3", Name: "DeleteMe"}},
		},
		Ignores: []types.RecordIgnored[Dummy]{
			{Record: Dummy{ID: "4", Name: "IgnoreMe"}, Reason: "some reason"},
		},
	}

	dir := testutils.NewTestDir(t)
	planFilePath := testutils.WriteJSONFile(t, dir, "plan.json", plan)

	var logs []string

	err := apply.Run(apply.RunParams[Dummy]{
		PlanFilePath: planFilePath,
		FormatRecord: func(d Dummy) string {
			return fmt.Sprintf("Dummy(ID=%s, Name=%s)", d.ID, d.Name)
		},
		FormatKey: func(key string) string {
			return fmt.Sprintf("Key[%s]", key)
		},
		OnAdd: func(add types.RecordAddition[Dummy]) error {
			logs = append(logs, "added:"+add.New.ID)
			return nil
		},
		OnUpdate: func(upd types.RecordUpdate[Dummy]) error {
			logs = append(logs, "updated:"+upd.New.ID)
			return nil
		},
		OnDelete: func(del types.RecordDeletion[Dummy]) error {
			logs = append(logs, "deleted:"+del.Old.ID)
			return nil
		},
		OnFinalize: func() error {
			logs = append(logs, "finalized")
			return nil
		},
	})
	assert.NoError(t, err)

	assert.ElementsMatch(t, []string{
		"added:1", "updated:2", "deleted:3", "finalized",
	}, logs)
}

func TestRun_NoChanges(t *testing.T) {
	dir := testutils.NewTestDir(t)
	planFilePath := testutils.WriteJSONFile(t, dir, "empty_plan.json", types.Plan[Dummy]{}) // empty plan

	err := apply.Run(apply.RunParams[Dummy]{
		PlanFilePath: planFilePath,
		FormatRecord: func(d Dummy) string { return d.ID },
		FormatKey:    func(key string) string { return key },
		OnAdd:        func(_ types.RecordAddition[Dummy]) error { t.Fatal("should not be called"); return nil },
		OnUpdate:     func(_ types.RecordUpdate[Dummy]) error { t.Fatal("should not be called"); return nil },
		OnDelete:     func(_ types.RecordDeletion[Dummy]) error { t.Fatal("should not be called"); return nil },
	})
	assert.NoError(t, err)
}

func TestRun_ErrorDuringFinalization(t *testing.T) {
	plan := types.Plan[Dummy]{
		Additions: []types.RecordAddition[Dummy]{
			{New: Dummy{ID: "1", Name: "Add"}},
		},
	}

	dir := testutils.NewTestDir(t)
	planFilePath := testutils.WriteJSONFile(t, dir, "plan_with_finalize_failure.json", plan)

	err := apply.Run(apply.RunParams[Dummy]{
		PlanFilePath: planFilePath,
		FormatRecord: func(d Dummy) string { return d.ID },
		FormatKey:    func(k string) string { return k },
		OnAdd: func(add types.RecordAddition[Dummy]) error {
			return nil
		},
		OnUpdate: func(_ types.RecordUpdate[Dummy]) error { return nil },
		OnDelete: func(_ types.RecordDeletion[Dummy]) error { return nil },
		OnFinalize: func() error {
			return fmt.Errorf("finalize failed")
		},
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "operation failed after 3 retries")
}

func TestRun_ErrorLoadingPlanFile(t *testing.T) {
	dir := testutils.NewTestDir(t)
	// Write invalid JSON that can't be parsed as a plan
	invalidPlanPath := testutils.CreateMockFile(t, dir, "invalid_plan.json", []byte("not valid json"))

	err := apply.Run(apply.RunParams[Dummy]{
		PlanFilePath: invalidPlanPath,
		FormatRecord: func(d Dummy) string { return d.ID },
		FormatKey:    func(k string) string { return k },
		OnAdd:        func(_ types.RecordAddition[Dummy]) error { return nil },
		OnUpdate:     func(_ types.RecordUpdate[Dummy]) error { return nil },
		OnDelete:     func(_ types.RecordDeletion[Dummy]) error { return nil },
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load plan file")
}

func TestRun_WithoutFinalization(t *testing.T) {
	plan := types.Plan[Dummy]{
		Additions: []types.RecordAddition[Dummy]{
			{New: Dummy{ID: "1", Name: "AddMe"}},
		},
	}

	dir := testutils.NewTestDir(t)
	planFilePath := testutils.WriteJSONFile(t, dir, "plan_no_finalize.json", plan)

	var logs []string

	// OnFinalize is nil (not provided)
	err := apply.Run(apply.RunParams[Dummy]{
		PlanFilePath: planFilePath,
		FormatRecord: func(d Dummy) string { return d.ID },
		FormatKey:    func(k string) string { return k },
		OnAdd: func(add types.RecordAddition[Dummy]) error {
			logs = append(logs, "added:"+add.New.ID)
			return nil
		},
		OnUpdate: func(_ types.RecordUpdate[Dummy]) error { return nil },
		OnDelete: func(_ types.RecordDeletion[Dummy]) error { return nil },
		// OnFinalize is nil - should not fail
	})

	assert.NoError(t, err)
	assert.Contains(t, logs, "added:1")
}

func TestRun_NilFormatRecord(t *testing.T) {
	dir := testutils.NewTestDir(t)
	planFilePath := testutils.WriteJSONFile(t, dir, "plan.json", types.Plan[Dummy]{})

	err := apply.Run(apply.RunParams[Dummy]{
		PlanFilePath: planFilePath,
		FormatRecord: nil, // Nil function
		FormatKey:    func(k string) string { return k },
		OnAdd:        func(_ types.RecordAddition[Dummy]) error { return nil },
		OnUpdate:     func(_ types.RecordUpdate[Dummy]) error { return nil },
		OnDelete:     func(_ types.RecordDeletion[Dummy]) error { return nil },
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "FormatRecord is required")
}

func TestRun_NilFormatKey(t *testing.T) {
	dir := testutils.NewTestDir(t)
	planFilePath := testutils.WriteJSONFile(t, dir, "plan.json", types.Plan[Dummy]{})

	err := apply.Run(apply.RunParams[Dummy]{
		PlanFilePath: planFilePath,
		FormatRecord: func(d Dummy) string { return d.ID },
		FormatKey:    nil, // Nil function
		OnAdd:        func(_ types.RecordAddition[Dummy]) error { return nil },
		OnUpdate:     func(_ types.RecordUpdate[Dummy]) error { return nil },
		OnDelete:     func(_ types.RecordDeletion[Dummy]) error { return nil },
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "FormatKey is required")
}

func TestRun_NilOnAdd(t *testing.T) {
	dir := testutils.NewTestDir(t)
	planFilePath := testutils.WriteJSONFile(t, dir, "plan.json", types.Plan[Dummy]{})

	err := apply.Run(apply.RunParams[Dummy]{
		PlanFilePath: planFilePath,
		FormatRecord: func(d Dummy) string { return d.ID },
		FormatKey:    func(k string) string { return k },
		OnAdd:        nil, // Nil function
		OnUpdate:     func(_ types.RecordUpdate[Dummy]) error { return nil },
		OnDelete:     func(_ types.RecordDeletion[Dummy]) error { return nil },
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OnAdd is required")
}

func TestRun_NilOnUpdate(t *testing.T) {
	dir := testutils.NewTestDir(t)
	planFilePath := testutils.WriteJSONFile(t, dir, "plan.json", types.Plan[Dummy]{})

	err := apply.Run(apply.RunParams[Dummy]{
		PlanFilePath: planFilePath,
		FormatRecord: func(d Dummy) string { return d.ID },
		FormatKey:    func(k string) string { return k },
		OnAdd:        func(_ types.RecordAddition[Dummy]) error { return nil },
		OnUpdate:     nil, // Nil function
		OnDelete:     func(_ types.RecordDeletion[Dummy]) error { return nil },
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OnUpdate is required")
}

func TestRun_NilOnDelete(t *testing.T) {
	dir := testutils.NewTestDir(t)
	planFilePath := testutils.WriteJSONFile(t, dir, "plan.json", types.Plan[Dummy]{})

	err := apply.Run(apply.RunParams[Dummy]{
		PlanFilePath: planFilePath,
		FormatRecord: func(d Dummy) string { return d.ID },
		FormatKey:    func(k string) string { return k },
		OnAdd:        func(_ types.RecordAddition[Dummy]) error { return nil },
		OnUpdate:     func(_ types.RecordUpdate[Dummy]) error { return nil },
		OnDelete:     nil, // Nil function
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OnDelete is required")
}
