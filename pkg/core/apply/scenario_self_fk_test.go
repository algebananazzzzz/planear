package apply_test

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/algebananazzzzz/planear/pkg/core/apply"
	"github.com/algebananazzzzz/planear/pkg/core/plan"
	"github.com/algebananazzzzz/planear/pkg/types"
	"github.com/algebananazzzzz/planear/testutils"
	"github.com/stretchr/testify/require"
)

type Position struct {
	Key         string
	CCA         string
	Name        string
	ReportingTo string // key of parent Position, or "" for root
}

type fakeDB struct {
	mu   sync.Mutex
	rows map[string]Position
}

func newFakeDB() *fakeDB { return &fakeDB{rows: map[string]Position{}} }

func (d *fakeDB) add(p Position) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if p.ReportingTo != "" {
		if _, ok := d.rows[p.ReportingTo]; !ok {
			return fmt.Errorf("FK violation: %q references missing %q", p.Key, p.ReportingTo)
		}
	}
	d.rows[p.Key] = p
	return nil
}

func (d *fakeDB) delete(key string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, row := range d.rows {
		if row.ReportingTo == key {
			return fmt.Errorf("FK violation: cannot delete %q, %q still references it", key, row.Key)
		}
	}
	delete(d.rows, key)
	return nil
}

func posDeps(p Position) []string {
	if p.ReportingTo == "" {
		return nil
	}
	return []string{p.ReportingTo}
}

func TestScenario_CCAPositions_LayeredSucceeds(t *testing.T) {
	db := newFakeDB()

	additions := []types.RecordAddition[Position]{
		{Key: "JCRC.President", New: Position{Key: "JCRC.President", CCA: "JCRC", Name: "President"}},
		{Key: "JCRC.Welfare-Head", New: Position{Key: "JCRC.Welfare-Head", CCA: "JCRC", Name: "Welfare Head", ReportingTo: "JCRC.President"}},
		{Key: "JCRC.Welfare-Member", New: Position{Key: "JCRC.Welfare-Member", CCA: "JCRC", Name: "Welfare Member", ReportingTo: "JCRC.Welfare-Head"}},
	}
	p := types.Plan[Position]{Additions: additions}

	layers, err := plan.ComputeLayers(p, posDeps)
	require.NoError(t, err)
	p.Layers = layers

	parallelism := 4
	report, err := apply.ExecuteOperations(apply.ExecuteOperationsParams[Position]{
		Plan:            p,
		FormatRecord:    func(pos Position) string { return pos.Key },
		FormatKey:       func(k string) string { return k },
		Parallelization: &parallelism,
		OnAdd:           func(r types.RecordAddition[Position]) error { return db.add(r.New) },
		OnUpdate:        func(types.RecordUpdate[Position]) error { return nil },
		OnDelete:        func(r types.RecordDeletion[Position]) error { return db.delete(r.Old.Key) },
	})
	require.NoError(t, err)
	require.Len(t, report.Success.Additions, 3)
	require.Empty(t, report.Failure.Additions)
	require.Empty(t, report.Skipped.Additions)

	// Verify FK ordering held: parent rows present before children.
	require.Contains(t, db.rows, "JCRC.President")
	require.Contains(t, db.rows, "JCRC.Welfare-Head")
	require.Contains(t, db.rows, "JCRC.Welfare-Member")
}

func TestScenario_CCAPositions_DeletionInverts(t *testing.T) {
	// Setup: parent, child rows in DB. Plan: delete parent, update child to clear FK.
	// Without inversion, parent-delete would fail because child still references it.
	// With ComputeLayers' deletion inversion, child-update lands first.
	db := newFakeDB()
	require.NoError(t, db.add(Position{Key: "parent", Name: "P"}))
	require.NoError(t, db.add(Position{Key: "child", Name: "C", ReportingTo: "parent"}))

	p := types.Plan[Position]{
		Updates: []types.RecordUpdate[Position]{
			{
				Key: "child",
				Old: Position{Key: "child", Name: "C", ReportingTo: "parent"},
				New: Position{Key: "child", Name: "C", ReportingTo: ""},
			},
		},
		Deletions: []types.RecordDeletion[Position]{
			{Key: "parent", Old: Position{Key: "parent", Name: "P"}},
		},
	}

	layers, err := plan.ComputeLayers(p, posDeps)
	require.NoError(t, err)
	require.Equal(t, [][]types.LayerOp{
		{{Kind: types.LayerOpUpdate, Key: "child"}},
		{{Kind: types.LayerOpDelete, Key: "parent"}},
	}, layers)
	p.Layers = layers

	parallelism := 4
	report, err := apply.ExecuteOperations(apply.ExecuteOperationsParams[Position]{
		Plan:            p,
		FormatRecord:    func(pos Position) string { return pos.Key },
		FormatKey:       func(k string) string { return k },
		Parallelization: &parallelism,
		OnAdd:           func(r types.RecordAddition[Position]) error { return db.add(r.New) },
		OnUpdate: func(r types.RecordUpdate[Position]) error {
			db.mu.Lock()
			defer db.mu.Unlock()
			db.rows[r.New.Key] = r.New
			return nil
		},
		OnDelete: func(r types.RecordDeletion[Position]) error { return db.delete(r.Old.Key) },
	})
	require.NoError(t, err)
	require.Len(t, report.Success.Updates, 1)
	require.Len(t, report.Success.Deletions, 1)
	require.Empty(t, report.Failure.Deletions)
	require.NotContains(t, db.rows, "parent")
	require.Equal(t, "", db.rows["child"].ReportingTo)
}

func TestScenario_CCAPositions_CascadingDeleteSucceedsWithLayering(t *testing.T) {
	// Real-DB analogue: parent and child both deleted in one plan, where
	// child.Old references parent. Layering must order child-delete before
	// parent-delete to satisfy the FK-enforcing fakeDB; without layering the
	// two deletions race.
	db := newFakeDB()
	require.NoError(t, db.add(Position{Key: "parent", Name: "P"}))
	require.NoError(t, db.add(Position{Key: "child", Name: "C", ReportingTo: "parent"}))

	p := types.Plan[Position]{
		Deletions: []types.RecordDeletion[Position]{
			{Key: "parent", Old: Position{Key: "parent", Name: "P"}},
			{Key: "child", Old: Position{Key: "child", Name: "C", ReportingTo: "parent"}},
		},
	}
	layers, err := plan.ComputeLayers(p, posDeps)
	require.NoError(t, err)
	require.Equal(t, [][]types.LayerOp{
		{{Kind: types.LayerOpDelete, Key: "child"}},
		{{Kind: types.LayerOpDelete, Key: "parent"}},
	}, layers)
	p.Layers = layers

	parallelism := 4
	report, err := apply.ExecuteOperations(apply.ExecuteOperationsParams[Position]{
		Plan:            p,
		FormatRecord:    func(pos Position) string { return pos.Key },
		FormatKey:       func(k string) string { return k },
		Parallelization: &parallelism,
		OnAdd:           func(r types.RecordAddition[Position]) error { return db.add(r.New) },
		OnUpdate:        func(types.RecordUpdate[Position]) error { return nil },
		OnDelete:        func(r types.RecordDeletion[Position]) error { return db.delete(r.Old.Key) },
	})
	require.NoError(t, err)
	require.Len(t, report.Success.Deletions, 2)
	require.Empty(t, report.Failure.Deletions)
	require.NotContains(t, db.rows, "parent")
	require.NotContains(t, db.rows, "child")
}

func TestScenario_CCAPositions_CascadingDeleteFlatPathCanRace(t *testing.T) {
	// Companion to the layered cascading-delete test: with Layers == nil, the
	// flat path dispatches both deletes concurrently. Whether the race
	// manifests depends on goroutine scheduling, so this test is informational
	// — it just asserts the failure mode the layered version eliminates is
	// real enough to be worth fixing.
	saw := 0
	for i := 0; i < 20; i++ {
		db := newFakeDB()
		require.NoError(t, db.add(Position{Key: "parent", Name: "P"}))
		require.NoError(t, db.add(Position{Key: "child", Name: "C", ReportingTo: "parent"}))

		p := types.Plan[Position]{
			Deletions: []types.RecordDeletion[Position]{
				{Key: "parent", Old: Position{Key: "parent", Name: "P"}},
				{Key: "child", Old: Position{Key: "child", Name: "C", ReportingTo: "parent"}},
			},
		}
		parallelism := 4
		report, _ := apply.ExecuteOperations(apply.ExecuteOperationsParams[Position]{
			Plan:            p,
			FormatRecord:    func(pos Position) string { return pos.Key },
			FormatKey:       func(k string) string { return k },
			Parallelization: &parallelism,
			OnAdd:           func(r types.RecordAddition[Position]) error { return db.add(r.New) },
			OnUpdate:        func(types.RecordUpdate[Position]) error { return nil },
			OnDelete:        func(r types.RecordDeletion[Position]) error { return db.delete(r.Old.Key) },
		})
		if len(report.Failure.Deletions) > 0 {
			saw++
		}
	}
	t.Logf("flat-path cascading-delete failure observed in %d/20 iterations (documentation only)", saw)
}

func TestScenario_CCAPositions_FlatPathCanRace(t *testing.T) {
	// Without Layers, the flat path may attempt to add the child before the parent.
	// The test isn't required to fail deterministically — it's a documentation test
	// asserting that the failure mode the feature solves exists in the no-Layers case.
	//
	// We loop several times to give the race a chance. If at least one iteration
	// produces a failure, the regression is locked in. If none fail, we don't fail
	// the test (flat-path race depends on goroutine scheduling).
	saw := 0
	for i := 0; i < 20; i++ {
		db := newFakeDB()
		p := types.Plan[Position]{
			Additions: []types.RecordAddition[Position]{
				{Key: "child", New: Position{Key: "child", ReportingTo: "parent"}},
				{Key: "parent", New: Position{Key: "parent"}},
			},
		}
		parallelism := 4
		report, _ := apply.ExecuteOperations(apply.ExecuteOperationsParams[Position]{
			Plan:            p,
			FormatRecord:    func(pos Position) string { return pos.Key },
			FormatKey:       func(k string) string { return k },
			Parallelization: &parallelism,
			OnAdd:           func(r types.RecordAddition[Position]) error { return db.add(r.New) },
			OnUpdate:        func(types.RecordUpdate[Position]) error { return nil },
			OnDelete:        func(r types.RecordDeletion[Position]) error { return db.delete(r.Old.Key) },
		})
		if len(report.Failure.Additions) > 0 {
			saw++
		}
	}
	t.Logf("flat-path failure observed in %d/20 iterations (documentation only)", saw)
}

func TestRun_LayeredPlan_RoundTripsThroughFile(t *testing.T) {
	tmpDir := testutils.NewTestDir(t)
	planPath := filepath.Join(tmpDir, "plan.json")

	p := types.Plan[Position]{
		Additions: []types.RecordAddition[Position]{
			{Key: "parent", New: Position{Key: "parent"}},
			{Key: "child", New: Position{Key: "child", ReportingTo: "parent"}},
		},
		Layers: [][]types.LayerOp{
			{{Kind: types.LayerOpAdd, Key: "parent"}},
			{{Kind: types.LayerOpAdd, Key: "child"}},
		},
	}
	testutils.WriteJSONFile(t, tmpDir, "plan.json", p)

	db := newFakeDB()
	err := apply.Run(apply.RunParams[Position]{
		PlanFilePath: planPath,
		FormatRecord: func(p Position) string { return p.Key },
		FormatKey:    func(k string) string { return k },
		OnAdd:        func(r types.RecordAddition[Position]) error { return db.add(r.New) },
		OnUpdate:     func(types.RecordUpdate[Position]) error { return nil },
		OnDelete:     func(r types.RecordDeletion[Position]) error { return db.delete(r.Old.Key) },
	})
	require.NoError(t, err)
	require.Contains(t, db.rows, "parent")
	require.Contains(t, db.rows, "child")
}

func TestRun_HandEditedPlan_RejectedByMultisetCheck(t *testing.T) {
	tmpDir := testutils.NewTestDir(t)
	planPath := filepath.Join(tmpDir, "plan.json")

	// Plan has TWO additions but Layers references only ONE — multiset mismatch.
	p := types.Plan[Position]{
		Additions: []types.RecordAddition[Position]{
			{Key: "A", New: Position{Key: "A"}},
			{Key: "B", New: Position{Key: "B"}},
		},
		Layers: [][]types.LayerOp{
			{{Kind: types.LayerOpAdd, Key: "A"}},
			// "B" intentionally missing
		},
	}
	testutils.WriteJSONFile(t, tmpDir, "plan.json", p)

	db := newFakeDB()
	added := 0
	var addedMu sync.Mutex
	err := apply.Run(apply.RunParams[Position]{
		PlanFilePath: planPath,
		FormatRecord: func(p Position) string { return p.Key },
		FormatKey:    func(k string) string { return k },
		OnAdd: func(r types.RecordAddition[Position]) error {
			addedMu.Lock()
			added++
			addedMu.Unlock()
			return db.add(r.New)
		},
		OnUpdate: func(types.RecordUpdate[Position]) error { return nil },
		OnDelete: func(r types.RecordDeletion[Position]) error { return db.delete(r.Old.Key) },
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "multiset")
	require.Equal(t, 0, added, "no DB writes must occur on multiset mismatch")
}
