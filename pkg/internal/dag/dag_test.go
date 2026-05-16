package dag_test

import (
	"testing"

	"github.com/algebananazzzzz/planear/pkg/internal/dag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildLayers_LinearChain(t *testing.T) {
	edges := map[string][]string{
		"A": {},
		"B": {"A"},
		"C": {"B"},
	}
	layers, err := dag.BuildLayers([]string{"A", "B", "C"}, edges)
	require.NoError(t, err)
	require.Equal(t, [][]string{{"A"}, {"B"}, {"C"}}, layers)
}

func TestBuildLayers_DiamondShape(t *testing.T) {
	edges := map[string][]string{
		"A": {},
		"B": {"A"},
		"C": {"A"},
		"D": {"B", "C"},
	}
	layers, err := dag.BuildLayers([]string{"A", "B", "C", "D"}, edges)
	require.NoError(t, err)
	require.Len(t, layers, 3)
	assert.ElementsMatch(t, []string{"A"}, layers[0])
	assert.ElementsMatch(t, []string{"B", "C"}, layers[1])
	assert.ElementsMatch(t, []string{"D"}, layers[2])
}

func TestBuildLayers_UnknownDependencyTreatedAsExternal(t *testing.T) {
	edges := map[string][]string{
		"A": {},
		"B": {"X"},
	}
	layers, err := dag.BuildLayers([]string{"A", "B"}, edges)
	require.NoError(t, err)
	require.Len(t, layers, 1)
	assert.ElementsMatch(t, []string{"A", "B"}, layers[0])
}

func TestBuildLayers_CycleReportsPath(t *testing.T) {
	edges := map[string][]string{
		"A": {"C"},
		"B": {"A"},
		"C": {"B"},
	}
	_, err := dag.BuildLayers([]string{"A", "B", "C"}, edges)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle detected")
	for _, n := range []string{"A", "B", "C"} {
		assert.Contains(t, err.Error(), n)
	}
}

func TestBuildLayers_SelfLoopReportsCycle(t *testing.T) {
	edges := map[string][]string{"A": {"A"}}
	_, err := dag.BuildLayers([]string{"A"}, edges)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle detected")
}

// TestBuildLayers_CycleWithExternalDeps ensures formatCycleError correctly
// skips dependencies that are outside the node set when walking the cycle.
func TestBuildLayers_CycleWithExternalDeps(t *testing.T) {
	edges := map[string][]string{
		"A": {"EXTERNAL_1", "B"},
		"B": {"EXTERNAL_2", "A"},
	}
	_, err := dag.BuildLayers([]string{"A", "B"}, edges)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle detected")
	assert.NotContains(t, err.Error(), "EXTERNAL")
}

func TestBuildLayers_EmptyInput(t *testing.T) {
	layers, err := dag.BuildLayers(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, layers)
}

func TestBuildLayers_DeterministicOrdering(t *testing.T) {
	edges := map[string][]string{"A": {}, "B": {}, "C": {}, "D": {}}
	nodes := []string{"D", "B", "A", "C"}
	layers, err := dag.BuildLayers(nodes, edges)
	require.NoError(t, err)
	require.Equal(t, [][]string{{"A", "B", "C", "D"}}, layers)
}
