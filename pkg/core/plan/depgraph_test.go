package plan_test

import (
	"testing"

	"github.com/algebananazzzzz/planear/pkg/core/plan"
	"github.com/algebananazzzzz/planear/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type depRec struct {
	Key    string
	Parent string
}

func keyOf(r depRec) string { return r.Key }
func parentOf(r depRec) []string {
	if r.Parent == "" {
		return nil
	}
	return []string{r.Parent}
}

func TestComputeLayers_AdditionDependsOnAddition(t *testing.T) {
	p := types.Plan[depRec]{
		Additions: []types.RecordAddition[depRec]{
			{Key: "child", New: depRec{Key: "child", Parent: "parent"}},
			{Key: "parent", New: depRec{Key: "parent"}},
		},
	}
	layers, err := plan.ComputeLayers(p, keyOf, parentOf)
	require.NoError(t, err)
	require.Equal(t, [][]types.LayerOp{
		{{Kind: types.LayerOpAdd, Key: "parent"}},
		{{Kind: types.LayerOpAdd, Key: "child"}},
	}, layers)
}

func TestComputeLayers_DeletionInverts(t *testing.T) {
	// Child is updated to drop its reference to parent; parent is deleted.
	// Delete must run AFTER child update so the FK reference is gone first.
	p := types.Plan[depRec]{
		Updates: []types.RecordUpdate[depRec]{
			{
				Key: "child",
				Old: depRec{Key: "child", Parent: "parent"},
				New: depRec{Key: "child", Parent: ""},
			},
		},
		Deletions: []types.RecordDeletion[depRec]{
			{Key: "parent", Old: depRec{Key: "parent"}},
		},
	}
	layers, err := plan.ComputeLayers(p, keyOf, parentOf)
	require.NoError(t, err)
	require.Equal(t, [][]types.LayerOp{
		{{Kind: types.LayerOpUpdate, Key: "child"}},
		{{Kind: types.LayerOpDelete, Key: "parent"}},
	}, layers)
}

func TestComputeLayers_DeletionAfterAdditionThatReferencesIt(t *testing.T) {
	// Edge case: an Addition's NEW state references something being deleted.
	// (Unusual but possible: adding a row that points to a soon-to-be-deleted row.)
	// The deletion must run after the addition so the addition can still see the parent.
	p := types.Plan[depRec]{
		Additions: []types.RecordAddition[depRec]{
			{Key: "newchild", New: depRec{Key: "newchild", Parent: "doomed"}},
		},
		Deletions: []types.RecordDeletion[depRec]{
			{Key: "doomed", Old: depRec{Key: "doomed"}},
		},
	}
	layers, err := plan.ComputeLayers(p, keyOf, parentOf)
	require.NoError(t, err)
	require.Equal(t, [][]types.LayerOp{
		{{Kind: types.LayerOpAdd, Key: "newchild"}},
		{{Kind: types.LayerOpDelete, Key: "doomed"}},
	}, layers)
}

func TestComputeLayers_CycleReturnsError(t *testing.T) {
	p := types.Plan[depRec]{
		Additions: []types.RecordAddition[depRec]{
			{Key: "A", New: depRec{Key: "A", Parent: "B"}},
			{Key: "B", New: depRec{Key: "B", Parent: "A"}},
		},
	}
	_, err := plan.ComputeLayers(p, keyOf, parentOf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle detected")
}

func TestComputeLayers_DependencyOutsidePlanIsExternal(t *testing.T) {
	p := types.Plan[depRec]{
		Additions: []types.RecordAddition[depRec]{
			{Key: "child", New: depRec{Key: "child", Parent: "ghost"}},
		},
	}
	layers, err := plan.ComputeLayers(p, keyOf, parentOf)
	require.NoError(t, err)
	require.Equal(t, [][]types.LayerOp{
		{{Kind: types.LayerOpAdd, Key: "child"}},
	}, layers)
}

func TestComputeLayers_EmptyPlanReturnsNoLayers(t *testing.T) {
	layers, err := plan.ComputeLayers(types.Plan[depRec]{}, keyOf, parentOf)
	require.NoError(t, err)
	require.Empty(t, layers)
}

func TestComputeLayers_DiamondDependencies(t *testing.T) {
	// A is root; B and C both depend on A; D depends on both B and C.
	// Test with multi-dep parents.
	type multiRec struct {
		Key     string
		Parents []string
	}
	mKey := func(r multiRec) string { return r.Key }
	mDeps := func(r multiRec) []string { return r.Parents }

	p := types.Plan[multiRec]{
		Additions: []types.RecordAddition[multiRec]{
			{Key: "A", New: multiRec{Key: "A"}},
			{Key: "B", New: multiRec{Key: "B", Parents: []string{"A"}}},
			{Key: "C", New: multiRec{Key: "C", Parents: []string{"A"}}},
			{Key: "D", New: multiRec{Key: "D", Parents: []string{"B", "C"}}},
		},
	}
	layers, err := plan.ComputeLayers(p, mKey, mDeps)
	require.NoError(t, err)
	require.Len(t, layers, 3)
	assert.Equal(t, []types.LayerOp{{Kind: types.LayerOpAdd, Key: "A"}}, layers[0])
	assert.ElementsMatch(t, []types.LayerOp{
		{Kind: types.LayerOpAdd, Key: "B"},
		{Kind: types.LayerOpAdd, Key: "C"},
	}, layers[1])
	assert.Equal(t, []types.LayerOp{{Kind: types.LayerOpAdd, Key: "D"}}, layers[2])
}
