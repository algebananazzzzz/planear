package apply_test

// integration_cca_scenario_test.go — elaborate multi-CCA integration tests for
// the dependency-aware-layering feature. Each subtest exercises a distinct code
// path of plan.ComputeLayers + apply.ExecuteOperations / apply.Run.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/algebananazzzzz/planear/pkg/core/apply"
	"github.com/algebananazzzzz/planear/pkg/core/plan"
	"github.com/algebananazzzzz/planear/pkg/types"
	"github.com/algebananazzzzz/planear/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// CCA domain types
// ---------------------------------------------------------------------------

// CCAPosition is the CSV record type for the integration tests. We define it
// separately from the simpler Position in scenario_self_fk_test.go so the two
// files coexist without field conflicts.
type CCAPosition struct {
	CCA         string `csv:"cca"`
	Name        string `csv:"name"`
	ReportingTo string `csv:"reporting_to"`
}

// Key returns the canonical "<cca>.<name>" identifier.
func (p CCAPosition) Key() string { return p.CCA + "." + p.Name }

// ---------------------------------------------------------------------------
// Fake in-memory DB with FK enforcement
// ---------------------------------------------------------------------------

type ccaDB struct {
	mu   sync.Mutex
	rows map[string]CCAPosition
	// callback counters (for multiset-mismatch test)
	addCalls int
	updCalls int
	delCalls int
}

func newCCADB() *ccaDB { return &ccaDB{rows: map[string]CCAPosition{}} }

func (d *ccaDB) seed(rows []CCAPosition) {
	for _, r := range rows {
		d.rows[r.Key()] = r
	}
}

func (d *ccaDB) add(p CCAPosition) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.addCalls++
	if p.ReportingTo != "" {
		if _, ok := d.rows[p.ReportingTo]; !ok {
			return fmt.Errorf("FK violation: %q references missing %q", p.Key(), p.ReportingTo)
		}
	}
	d.rows[p.Key()] = p
	return nil
}

func (d *ccaDB) update(upd types.RecordUpdate[CCAPosition]) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.updCalls++
	newP := upd.New
	if newP.ReportingTo != "" {
		if _, ok := d.rows[newP.ReportingTo]; !ok {
			return fmt.Errorf("FK violation on update: %q references missing %q", newP.Key(), newP.ReportingTo)
		}
	}
	d.rows[newP.Key()] = newP
	return nil
}

func (d *ccaDB) delete(key string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.delCalls++
	for _, row := range d.rows {
		if row.ReportingTo == key {
			return fmt.Errorf("FK violation: cannot delete %q, %q still references it", key, row.Key())
		}
	}
	delete(d.rows, key)
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// ccaDependsOn returns the reporting-to key of a CCAPosition (used as DependsOn).
func ccaDependsOn(p CCAPosition) []string {
	if p.ReportingTo == "" {
		return nil
	}
	return []string{p.ReportingTo}
}

// canonicalCSV is the 11-row baseline.
var canonicalCSV = []CCAPosition{
	{CCA: "JCRC", Name: "President", ReportingTo: ""},
	{CCA: "JCRC", Name: "Vice-President", ReportingTo: "JCRC.President"},
	{CCA: "JCRC", Name: "Welfare-Head", ReportingTo: "JCRC.Vice-President"},
	{CCA: "JCRC", Name: "Welfare-Member-1", ReportingTo: "JCRC.Welfare-Head"},
	{CCA: "JCRC", Name: "Welfare-Member-2", ReportingTo: "JCRC.Welfare-Head"},
	{CCA: "JCRC", Name: "Events-Head", ReportingTo: "JCRC.Vice-President"},
	{CCA: "JCRC", Name: "Events-Member-1", ReportingTo: "JCRC.Events-Head"},
	{CCA: "DEBATE", Name: "President", ReportingTo: ""},
	{CCA: "DEBATE", Name: "Secretary", ReportingTo: "DEBATE.President"},
	{CCA: "DEBATE", Name: "Member-1", ReportingTo: "DEBATE.Secretary"},
	{CCA: "DEBATE", Name: "Member-2", ReportingTo: "DEBATE.Secretary"},
}

// makeGenerateParams builds a GenerateParams for the given CSV directory and
// remote DB snapshot.
func makeGenerateParams(t *testing.T, csvDir, planPath string, remote map[string]CCAPosition, withDeps bool) plan.GenerateParams[CCAPosition] {
	t.Helper()
	p := plan.GenerateParams[CCAPosition]{
		CSVPath:          csvDir,
		OutputFilePath:   planPath,
		FormatRecordFunc: func(p CCAPosition) string { return p.Key() },
		FormatKeyFunc:    func(k string) string { return k },
		ExtractKeyFunc:   func(p CCAPosition) string { return p.Key() },
		LoadRemoteRecords: func() (map[string]CCAPosition, error) {
			// return a copy so tests don't share state
			cp := make(map[string]CCAPosition, len(remote))
			for k, v := range remote {
				cp[k] = v
			}
			return cp, nil
		},
		ValidateRecord: testutils.NoopValidator[CCAPosition](),
	}
	if withDeps {
		p.DependsOn = ccaDependsOn
	}
	return p
}

// sortLayerOps sorts a layer's ops so test assertions are deterministic.
func sortLayerOps(layer []types.LayerOp) []types.LayerOp {
	cp := make([]types.LayerOp, len(layer))
	copy(cp, layer)
	sort.Slice(cp, func(i, j int) bool {
		if cp[i].Kind != cp[j].Kind {
			return cp[i].Kind < cp[j].Kind
		}
		return cp[i].Key < cp[j].Key
	})
	return cp
}

func sortedKeys(ops []types.LayerOp) []string {
	keys := make([]string, len(ops))
	for i, o := range ops {
		keys[i] = o.Key
	}
	sort.Strings(keys)
	return keys
}

func additionKeys(adds []types.RecordAddition[CCAPosition]) []string {
	out := make([]string, len(adds))
	for i, a := range adds {
		out[i] = a.Key
	}
	sort.Strings(out)
	return out
}

func updateKeys(upds []types.RecordUpdate[CCAPosition]) []string {
	out := make([]string, len(upds))
	for i, u := range upds {
		out[i] = u.Key
	}
	sort.Strings(out)
	return out
}

func deletionKeys(dels []types.RecordDeletion[CCAPosition]) []string {
	out := make([]string, len(dels))
	for i, d := range dels {
		out[i] = d.Key
	}
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------------------
// Test 1: Greenfield_LayeredApply_AllSucceed
// ---------------------------------------------------------------------------

func TestIntegration_Greenfield_LayeredApply_AllSucceed(t *testing.T) {
	t.Parallel()

	dir := testutils.NewTestDir(t)
	planPath := filepath.Join(dir, "plan.json")
	testutils.WriteCSVFile(t, dir, "positions.csv", canonicalCSV)

	result, err := plan.Generate(makeGenerateParams(t, dir, planPath, nil, true))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Layers, "DependsOn was set — Layers must be populated")

	// Expected layers (manually computed):
	//   Layer 0: roots — JCRC.President, DEBATE.President
	//   Layer 1: direct children of roots — JCRC.Vice-President, DEBATE.Secretary
	//   Layer 2: children of VP/Secretary — JCRC.Welfare-Head, JCRC.Events-Head,
	//             DEBATE.Member-1, DEBATE.Member-2
	//   Layer 3: leaves under heads — JCRC.Welfare-Member-1, JCRC.Welfare-Member-2,
	//             JCRC.Events-Member-1
	require.Len(t, result.Layers, 4, "expected exactly 4 layers for the canonical 11-row CSV")

	assert.ElementsMatch(t, []string{"JCRC.President", "DEBATE.President"},
		sortedKeys(result.Layers[0]), "layer 0 must contain the two roots")
	assert.ElementsMatch(t, []string{"JCRC.Vice-President", "DEBATE.Secretary"},
		sortedKeys(result.Layers[1]), "layer 1 must contain VP and Secretary")
	assert.ElementsMatch(t,
		[]string{"JCRC.Welfare-Head", "JCRC.Events-Head", "DEBATE.Member-1", "DEBATE.Member-2"},
		sortedKeys(result.Layers[2]), "layer 2 must contain second-tier nodes")
	assert.ElementsMatch(t,
		[]string{"JCRC.Welfare-Member-1", "JCRC.Welfare-Member-2", "JCRC.Events-Member-1"},
		sortedKeys(result.Layers[3]), "layer 3 must contain the leaf members")

	// Apply via Run (round-trips through the plan file).
	db := newCCADB()
	finalizeCount := 0
	var finalizeMu sync.Mutex
	parallelism := 4

	err = apply.Run(apply.RunParams[CCAPosition]{
		PlanFilePath: planPath,
		FormatRecord: func(p CCAPosition) string { return p.Key() },
		FormatKey:    func(k string) string { return k },
		OnAdd: func(r types.RecordAddition[CCAPosition]) error {
			return db.add(r.New)
		},
		OnUpdate: func(r types.RecordUpdate[CCAPosition]) error {
			return db.update(r)
		},
		OnDelete: func(r types.RecordDeletion[CCAPosition]) error {
			return db.delete(r.Old.Key())
		},
		OnFinalize: func() error {
			finalizeMu.Lock()
			finalizeCount++
			finalizeMu.Unlock()
			return nil
		},
		Parallelization: &parallelism,
		FinalizeOn:      types.FinalizeOnAnySuccess,
	})
	require.NoError(t, err)

	// All 11 rows must be in the DB.
	db.mu.Lock()
	defer db.mu.Unlock()
	assert.Len(t, db.rows, 11, "all 11 positions must be in the DB")
	for _, row := range canonicalCSV {
		assert.Contains(t, db.rows, row.Key(), "DB must contain %s", row.Key())
	}
	assert.Equal(t, 1, finalizeCount, "OnFinalize must have been called exactly once")
}

// ---------------------------------------------------------------------------
// Test 2: Restructure_AddDeleteUpdateMix
// ---------------------------------------------------------------------------

func TestIntegration_Restructure_AddDeleteUpdateMix(t *testing.T) {
	t.Parallel()

	dir := testutils.NewTestDir(t)
	planPath := filepath.Join(dir, "plan.json")

	// Remote starts with the 11 canonical rows.
	remote := make(map[string]CCAPosition)
	for _, r := range canonicalCSV {
		remote[r.Key()] = r
	}

	// New local CSV:
	//   - keeps all unchanged rows except the ones below
	//   - ADDS JCRC.Tech-Head (under VP), JCRC.Tech-Member-1 (under Tech-Head)
	//   - DELETES JCRC.Welfare-Member-2 and JCRC.Events-Member-1
	//   - UPDATES JCRC.Events-Head: reporting_to JCRC.Vice-President → JCRC.President
	newCSV := []CCAPosition{
		{CCA: "JCRC", Name: "President", ReportingTo: ""},
		{CCA: "JCRC", Name: "Vice-President", ReportingTo: "JCRC.President"},
		{CCA: "JCRC", Name: "Welfare-Head", ReportingTo: "JCRC.Vice-President"},
		{CCA: "JCRC", Name: "Welfare-Member-1", ReportingTo: "JCRC.Welfare-Head"},
		// Welfare-Member-2 deleted (absent from new CSV)
		{CCA: "JCRC", Name: "Events-Head", ReportingTo: "JCRC.President"}, // updated
		// Events-Member-1 deleted (absent)
		{CCA: "JCRC", Name: "Tech-Head", ReportingTo: "JCRC.Vice-President"}, // new
		{CCA: "JCRC", Name: "Tech-Member-1", ReportingTo: "JCRC.Tech-Head"},  // new
		{CCA: "DEBATE", Name: "President", ReportingTo: ""},
		{CCA: "DEBATE", Name: "Secretary", ReportingTo: "DEBATE.President"},
		{CCA: "DEBATE", Name: "Member-1", ReportingTo: "DEBATE.Secretary"},
		{CCA: "DEBATE", Name: "Member-2", ReportingTo: "DEBATE.Secretary"},
	}

	testutils.WriteCSVFile(t, dir, "positions.csv", newCSV)

	result, err := plan.Generate(makeGenerateParams(t, dir, planPath, remote, true))
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify plan contents.
	require.Len(t, result.Additions, 2, "should add Tech-Head and Tech-Member-1")
	assert.ElementsMatch(t, []string{"JCRC.Tech-Head", "JCRC.Tech-Member-1"}, additionKeys(result.Additions))
	require.Len(t, result.Deletions, 2, "should delete Welfare-Member-2 and Events-Member-1")
	assert.ElementsMatch(t, []string{"JCRC.Welfare-Member-2", "JCRC.Events-Member-1"}, deletionKeys(result.Deletions))
	require.Len(t, result.Updates, 1, "should update Events-Head")
	assert.Equal(t, "JCRC.Events-Head", result.Updates[0].Key)

	// Additions must be layered: Tech-Head before Tech-Member-1.
	require.NotNil(t, result.Layers)
	// Find layers containing Tech-Head and Tech-Member-1.
	var techHeadLayer, techMemberLayer int = -1, -1
	for i, layer := range result.Layers {
		for _, op := range layer {
			if op.Key == "JCRC.Tech-Head" {
				techHeadLayer = i
			}
			if op.Key == "JCRC.Tech-Member-1" {
				techMemberLayer = i
			}
		}
	}
	assert.True(t, techHeadLayer >= 0, "Tech-Head must appear in a layer")
	assert.True(t, techMemberLayer >= 0, "Tech-Member-1 must appear in a layer")
	assert.Less(t, techHeadLayer, techMemberLayer,
		"Tech-Head (layer %d) must precede Tech-Member-1 (layer %d)", techHeadLayer, techMemberLayer)

	// Apply using the DB seeded with the 11 original rows.
	db := newCCADB()
	db.seed(canonicalCSV)
	parallelism := 4

	err = apply.Run(apply.RunParams[CCAPosition]{
		PlanFilePath: planPath,
		FormatRecord: func(p CCAPosition) string { return p.Key() },
		FormatKey:    func(k string) string { return k },
		OnAdd: func(r types.RecordAddition[CCAPosition]) error {
			return db.add(r.New)
		},
		OnUpdate: func(r types.RecordUpdate[CCAPosition]) error {
			return db.update(r)
		},
		OnDelete: func(r types.RecordDeletion[CCAPosition]) error {
			return db.delete(r.Old.Key())
		},
		Parallelization: &parallelism,
		FinalizeOn:      types.FinalizeAlways,
	})
	// Run returns an error when failures > 0; we expect no failures.
	require.NoError(t, err)

	// Verify final DB state.
	db.mu.Lock()
	defer db.mu.Unlock()
	assert.NotContains(t, db.rows, "JCRC.Welfare-Member-2", "must be deleted")
	assert.NotContains(t, db.rows, "JCRC.Events-Member-1", "must be deleted")
	assert.Contains(t, db.rows, "JCRC.Tech-Head")
	assert.Contains(t, db.rows, "JCRC.Tech-Member-1")
	assert.Equal(t, "JCRC.President", db.rows["JCRC.Events-Head"].ReportingTo,
		"Events-Head.reporting_to must now point to President")

	// Verify callback counts.
	assert.Equal(t, 2, db.addCalls, "exactly 2 additions")
	assert.Equal(t, 1, db.updCalls, "exactly 1 update")
	assert.Equal(t, 2, db.delCalls, "exactly 2 deletions")
}

// ---------------------------------------------------------------------------
// Test 3: CycleInCSV_FailsBeforeFileWrite
// ---------------------------------------------------------------------------

func TestIntegration_CycleInCSV_FailsBeforeFileWrite(t *testing.T) {
	t.Parallel()

	dir := testutils.NewTestDir(t)
	planPath := filepath.Join(dir, "plan.json")

	cycleCSV := []CCAPosition{
		{CCA: "X", Name: "A", ReportingTo: "X.B"},
		{CCA: "X", Name: "B", ReportingTo: "X.A"},
	}
	testutils.WriteCSVFile(t, dir, "positions.csv", cycleCSV)

	_, err := plan.Generate(makeGenerateParams(t, dir, planPath, nil, true))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle detected",
		"error must mention cycle detection")

	_, statErr := os.Stat(planPath)
	assert.True(t, os.IsNotExist(statErr),
		"plan file must NOT exist after cycle error (got: %v)", statErr)
}

// ---------------------------------------------------------------------------
// Test 4: MidLayerFailure_CascadesToSkipped
// ---------------------------------------------------------------------------

// Expected layers for canonical CSV (same computation as test 1):
//   Layer 0: {JCRC.President, DEBATE.President}
//   Layer 1: {JCRC.Vice-President, DEBATE.Secretary}
//   Layer 2: {JCRC.Welfare-Head, JCRC.Events-Head, DEBATE.Member-1, DEBATE.Member-2}
//   Layer 3: {JCRC.Welfare-Member-1, JCRC.Welfare-Member-2, JCRC.Events-Member-1}
//
// Failure injected at JCRC.Welfare-Head (layer 2).
// Layer 2 still attempts ALL ops in the layer (Welfare-Head, Events-Head, DEBATE.Member-1/2).
// Events-Head succeeds. Because Welfare-Head FAILS, the engine stops after layer 2
// and cascades layer 3 into Skipped.
// Layer 3 ops: {Welfare-Member-1, Welfare-Member-2, Events-Member-1} → all Skipped.
// Events-Member-1 is skipped even though Events-Head succeeded, because the
// cascade rule is layer-level: any failure in a layer stops subsequent layers.

func TestIntegration_MidLayerFailure_CascadesToSkipped(t *testing.T) {
	t.Parallel()

	dir := testutils.NewTestDir(t)
	planPath := filepath.Join(dir, "plan.json")
	testutils.WriteCSVFile(t, dir, "positions.csv", canonicalCSV)

	result, err := plan.Generate(makeGenerateParams(t, dir, planPath, nil, true))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Layers, 4)

	db := newCCADB()
	parallelism := 4
	finalizeCount := 0
	var finalizeMu sync.Mutex

	// Read plan from file (same as Run does).
	var loadedPlan types.Plan[CCAPosition]
	planBytes, err := os.ReadFile(planPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(planBytes, &loadedPlan))

	report, execErr := apply.ExecuteOperations(apply.ExecuteOperationsParams[CCAPosition]{
		Plan:         loadedPlan,
		FormatRecord: func(p CCAPosition) string { return p.Key() },
		FormatKey:    func(k string) string { return k },
		OnAdd: func(r types.RecordAddition[CCAPosition]) error {
			if r.Key == "JCRC.Welfare-Head" {
				return fmt.Errorf("injected failure for Welfare-Head")
			}
			return db.add(r.New)
		},
		OnUpdate: func(r types.RecordUpdate[CCAPosition]) error {
			return db.update(r)
		},
		OnDelete: func(r types.RecordDeletion[CCAPosition]) error {
			return db.delete(r.Old.Key())
		},
		OnFinalize: func() error {
			finalizeMu.Lock()
			finalizeCount++
			finalizeMu.Unlock()
			return nil
		},
		Parallelization: &parallelism,
		FinalizeOn:      types.FinalizeOnSuccess,
	})
	// ExecuteOperations itself should not error (failures are captured in report).
	require.NoError(t, execErr)
	require.NotNil(t, report)

	// Layers 0 and 1 must have succeeded completely.
	assert.ElementsMatch(t, []string{"JCRC.President", "DEBATE.President"},
		additionKeys(report.Success.Additions[:2]),
	)

	// Layer 2: Welfare-Head fails; Events-Head, DEBATE.Member-1/2 succeed.
	assert.Len(t, report.Failure.Additions, 1)
	assert.Equal(t, "JCRC.Welfare-Head", report.Failure.Additions[0].Key)

	// 7 total successes: President×2, VP, Secretary, Events-Head, DEBATE.Member-1, DEBATE.Member-2
	assert.Len(t, report.Success.Additions, 7)
	successKeys := additionKeys(report.Success.Additions)
	assert.Contains(t, successKeys, "JCRC.President")
	assert.Contains(t, successKeys, "DEBATE.President")
	assert.Contains(t, successKeys, "JCRC.Vice-President")
	assert.Contains(t, successKeys, "DEBATE.Secretary")
	assert.Contains(t, successKeys, "JCRC.Events-Head")
	assert.Contains(t, successKeys, "DEBATE.Member-1")
	assert.Contains(t, successKeys, "DEBATE.Member-2")
	assert.NotContains(t, successKeys, "JCRC.Welfare-Head")

	// Layer 3 must be fully skipped.
	assert.Len(t, report.Skipped.Additions, 3,
		"Welfare-Member-1, Welfare-Member-2, Events-Member-1 must all be skipped")
	skippedKeys := additionKeys(report.Skipped.Additions)
	assert.ElementsMatch(t, []string{"JCRC.Welfare-Member-1", "JCRC.Welfare-Member-2", "JCRC.Events-Member-1"},
		skippedKeys)

	// FinalizeOnSuccess must NOT be called when there are failures.
	assert.Equal(t, 0, finalizeCount, "OnFinalize must NOT be called when FinalizeOnSuccess and there are failures")

	// Verify DB state: Welfare-Head absent, Events-Head present, members absent.
	db.mu.Lock()
	defer db.mu.Unlock()
	assert.NotContains(t, db.rows, "JCRC.Welfare-Head")
	assert.Contains(t, db.rows, "JCRC.Events-Head")
	assert.NotContains(t, db.rows, "JCRC.Welfare-Member-1")
	assert.NotContains(t, db.rows, "JCRC.Welfare-Member-2")
	assert.NotContains(t, db.rows, "JCRC.Events-Member-1")
}

// ---------------------------------------------------------------------------
// Test 5: FinalizeOnAnySuccess_RunsOnPartialFailure
// ---------------------------------------------------------------------------

func TestIntegration_FinalizeOnAnySuccess_RunsOnPartialFailure(t *testing.T) {
	t.Parallel()

	dir := testutils.NewTestDir(t)
	planPath := filepath.Join(dir, "plan.json")
	testutils.WriteCSVFile(t, dir, "positions.csv", canonicalCSV)

	_, err := plan.Generate(makeGenerateParams(t, dir, planPath, nil, true))
	require.NoError(t, err)

	var loadedPlan types.Plan[CCAPosition]
	planBytes, err := os.ReadFile(planPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(planBytes, &loadedPlan))

	db := newCCADB()
	finalizeCount := 0
	var finalizeMu sync.Mutex
	parallelism := 4

	report, execErr := apply.ExecuteOperations(apply.ExecuteOperationsParams[CCAPosition]{
		Plan:         loadedPlan,
		FormatRecord: func(p CCAPosition) string { return p.Key() },
		FormatKey:    func(k string) string { return k },
		OnAdd: func(r types.RecordAddition[CCAPosition]) error {
			if r.Key == "JCRC.Welfare-Head" {
				return fmt.Errorf("injected failure for Welfare-Head")
			}
			return db.add(r.New)
		},
		OnUpdate: func(r types.RecordUpdate[CCAPosition]) error {
			return db.update(r)
		},
		OnDelete: func(r types.RecordDeletion[CCAPosition]) error {
			return db.delete(r.Old.Key())
		},
		OnFinalize: func() error {
			finalizeMu.Lock()
			finalizeCount++
			finalizeMu.Unlock()
			return nil
		},
		Parallelization: &parallelism,
		FinalizeOn:      types.FinalizeOnAnySuccess,
	})
	require.NoError(t, execErr)
	require.NotNil(t, report)

	// Some successes and some failures/skips.
	assert.NotEmpty(t, report.Success.Additions, "at least President/VP must have succeeded")
	assert.NotEmpty(t, report.Failure.Additions, "Welfare-Head must have failed")

	// FinalizeOnAnySuccess must run because President + VP etc. succeeded.
	assert.Equal(t, 1, finalizeCount, "OnFinalize must be called once under FinalizeOnAnySuccess")
}

// ---------------------------------------------------------------------------
// Test 6: MultisetMismatch_HandEditedPlan_RejectsBeforeAnyDBWrite
// ---------------------------------------------------------------------------

func TestIntegration_MultisetMismatch_HandEditedPlan_RejectsBeforeAnyDBWrite(t *testing.T) {
	t.Parallel()

	dir := testutils.NewTestDir(t)
	planPath := filepath.Join(dir, "plan.json")
	testutils.WriteCSVFile(t, dir, "positions.csv", canonicalCSV)

	_, err := plan.Generate(makeGenerateParams(t, dir, planPath, nil, true))
	require.NoError(t, err)

	// Read back and tamper: remove one op from Layers without removing it from Additions.
	planBytes, err := os.ReadFile(planPath)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(planBytes, &raw))

	var layers [][]types.LayerOp
	require.NoError(t, json.Unmarshal(raw["layers"], &layers))

	// Remove the first op from layer 0 (e.g. JCRC.President or DEBATE.President).
	require.NotEmpty(t, layers[0], "layer 0 must be non-empty")
	layers[0] = layers[0][1:] // drop first op

	layersJSON, err := json.Marshal(layers)
	require.NoError(t, err)
	raw["layers"] = layersJSON

	tamperedBytes, err := json.Marshal(raw)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(planPath, tamperedBytes, 0644))

	// Now apply.Run the tampered plan.
	db := newCCADB()
	addCalled := 0
	updCalled := 0
	delCalled := 0
	var callMu sync.Mutex

	err = apply.Run(apply.RunParams[CCAPosition]{
		PlanFilePath: planPath,
		FormatRecord: func(p CCAPosition) string { return p.Key() },
		FormatKey:    func(k string) string { return k },
		OnAdd: func(r types.RecordAddition[CCAPosition]) error {
			callMu.Lock()
			addCalled++
			callMu.Unlock()
			return db.add(r.New)
		},
		OnUpdate: func(r types.RecordUpdate[CCAPosition]) error {
			callMu.Lock()
			updCalled++
			callMu.Unlock()
			return db.update(r)
		},
		OnDelete: func(r types.RecordDeletion[CCAPosition]) error {
			callMu.Lock()
			delCalled++
			callMu.Unlock()
			return db.delete(r.Old.Key())
		},
	})
	require.Error(t, err, "tampered plan must be rejected")
	assert.Contains(t, err.Error(), "multiset",
		"error must mention multiset mismatch")

	// Zero DB writes must have happened.
	assert.Equal(t, 0, addCalled, "no OnAdd calls on multiset mismatch")
	assert.Equal(t, 0, updCalled, "no OnUpdate calls on multiset mismatch")
	assert.Equal(t, 0, delCalled, "no OnDelete calls on multiset mismatch")
}

// ---------------------------------------------------------------------------
// Test 7: DeletionInversion_ChildUpdateClearsBeforeParentDelete
// ---------------------------------------------------------------------------

func TestIntegration_DeletionInversion_ChildUpdateClearsBeforeParentDelete(t *testing.T) {
	t.Parallel()

	// Pre-populate DB with parent/child rows.
	db := newCCADB()
	parent := CCAPosition{CCA: "ORG", Name: "Parent", ReportingTo: ""}
	child := CCAPosition{CCA: "ORG", Name: "Child", ReportingTo: "ORG.Parent"}
	db.seed([]CCAPosition{parent, child})

	// Plan: update child to clear FK (reporting_to → ""), delete parent.
	childNew := CCAPosition{CCA: "ORG", Name: "Child", ReportingTo: ""}
	p := types.Plan[CCAPosition]{
		Updates: []types.RecordUpdate[CCAPosition]{
			{
				Key: child.Key(),
				Old: child,
				New: childNew,
			},
		},
		Deletions: []types.RecordDeletion[CCAPosition]{
			{Key: parent.Key(), Old: parent},
		},
	}

	layers, err := plan.ComputeLayers(p, ccaDependsOn)
	require.NoError(t, err)

	// Deletion inversion: child-update must land in an earlier layer than parent-delete.
	require.Len(t, layers, 2, "expected exactly 2 layers after inversion")
	require.Len(t, layers[0], 1)
	require.Len(t, layers[1], 1)

	assert.Equal(t, types.LayerOp{Kind: types.LayerOpUpdate, Key: child.Key()}, layers[0][0],
		"layer 0 must be the child update")
	assert.Equal(t, types.LayerOp{Kind: types.LayerOpDelete, Key: parent.Key()}, layers[1][0],
		"layer 1 must be the parent delete")

	// Apply: verify both ops succeed in the FK-enforcing DB.
	p.Layers = layers
	parallelism := 4
	report, err := apply.ExecuteOperations(apply.ExecuteOperationsParams[CCAPosition]{
		Plan:         p,
		FormatRecord: func(pos CCAPosition) string { return pos.Key() },
		FormatKey:    func(k string) string { return k },
		OnAdd: func(r types.RecordAddition[CCAPosition]) error {
			return db.add(r.New)
		},
		OnUpdate: func(r types.RecordUpdate[CCAPosition]) error {
			return db.update(r)
		},
		OnDelete: func(r types.RecordDeletion[CCAPosition]) error {
			return db.delete(r.Old.Key())
		},
		Parallelization: &parallelism,
	})
	require.NoError(t, err)
	require.NotNil(t, report)

	assert.Len(t, report.Success.Updates, 1)
	assert.Len(t, report.Success.Deletions, 1)
	assert.Empty(t, report.Failure.Updates)
	assert.Empty(t, report.Failure.Deletions)

	db.mu.Lock()
	defer db.mu.Unlock()
	assert.NotContains(t, db.rows, parent.Key(), "parent must be deleted")
	assert.Equal(t, "", db.rows[child.Key()].ReportingTo, "child FK must be cleared")
}

// ---------------------------------------------------------------------------
// Test 8: BackwardCompat_PlanWithoutLayers_TakesFlatPath
// ---------------------------------------------------------------------------

func TestIntegration_BackwardCompat_PlanWithoutLayers_TakesFlatPath(t *testing.T) {
	t.Parallel()

	dir := testutils.NewTestDir(t)
	planPath := filepath.Join(dir, "plan.json")

	// Manually construct a plan WITHOUT Layers (flat path).
	// Use a simple non-hierarchical set so FK order doesn't matter.
	flatPlan := types.Plan[CCAPosition]{
		Additions: []types.RecordAddition[CCAPosition]{
			{Key: "ORG.Alpha", New: CCAPosition{CCA: "ORG", Name: "Alpha", ReportingTo: ""}},
			{Key: "ORG.Beta", New: CCAPosition{CCA: "ORG", Name: "Beta", ReportingTo: ""}},
			{Key: "ORG.Gamma", New: CCAPosition{CCA: "ORG", Name: "Gamma", ReportingTo: ""}},
		},
		// Layers is intentionally nil — simulates a pre-feature plan file.
	}
	require.Nil(t, flatPlan.Layers, "must start with no Layers field")

	testutils.WriteJSONFile(t, dir, "plan.json", flatPlan)

	db := newCCADB()
	parallelism := 2

	err := apply.Run(apply.RunParams[CCAPosition]{
		PlanFilePath: planPath,
		FormatRecord: func(p CCAPosition) string { return p.Key() },
		FormatKey:    func(k string) string { return k },
		OnAdd: func(r types.RecordAddition[CCAPosition]) error {
			return db.add(r.New)
		},
		OnUpdate: func(r types.RecordUpdate[CCAPosition]) error {
			return db.update(r)
		},
		OnDelete: func(r types.RecordDeletion[CCAPosition]) error {
			return db.delete(r.Old.Key())
		},
		Parallelization: &parallelism,
	})
	require.NoError(t, err)

	db.mu.Lock()
	defer db.mu.Unlock()
	assert.Len(t, db.rows, 3)
	assert.Contains(t, db.rows, "ORG.Alpha")
	assert.Contains(t, db.rows, "ORG.Beta")
	assert.Contains(t, db.rows, "ORG.Gamma")
}
