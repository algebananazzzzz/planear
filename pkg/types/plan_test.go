package types_test

import (
	"encoding/json"
	"testing"

	"github.com/algebananazzzzz/planear/pkg/types"
	"github.com/stretchr/testify/require"
)

type rec struct {
	ID string `json:"id"`
}

func TestPlan_LayersJSONRoundTrip(t *testing.T) {
	p := types.Plan[rec]{
		Additions: []types.RecordAddition[rec]{{Key: "A", New: rec{ID: "A"}}},
		Layers: [][]types.LayerOp{
			{{Kind: types.LayerOpAdd, Key: "A"}},
		},
	}

	raw, err := json.Marshal(p)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"layers"`)

	var back types.Plan[rec]
	require.NoError(t, json.Unmarshal(raw, &back))
	require.Equal(t, p.Layers, back.Layers)
}

func TestPlan_NilLayersOmittedFromJSON(t *testing.T) {
	p := types.Plan[rec]{
		Additions: []types.RecordAddition[rec]{{Key: "A", New: rec{ID: "A"}}},
	}
	raw, err := json.Marshal(p)
	require.NoError(t, err)
	require.NotContains(t, string(raw), `"layers"`)
}

func TestExecutionReport_SkippedField(t *testing.T) {
	report := types.ExecutionReport[rec]{
		Skipped: types.Plan[rec]{Additions: []types.RecordAddition[rec]{{Key: "S", New: rec{ID: "S"}}}},
	}
	raw, err := json.Marshal(report)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"skipped"`)
}

func TestLayerOpConstants(t *testing.T) {
	require.Equal(t, types.LayerOpKind("add"), types.LayerOpAdd)
	require.Equal(t, types.LayerOpKind("update"), types.LayerOpUpdate)
	require.Equal(t, types.LayerOpKind("delete"), types.LayerOpDelete)
}
