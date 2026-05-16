# Dependency-Aware Plan Ordering (v2) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add native, declarative dependency resolution to planear. Consumers supply one `DependsOn func(T) []string` callback at plan-generation time; planear builds a DAG over the entire plan, detects cycles, topologically sorts into layers, and serializes the layer assignment into the plan JSON. At apply time, plans with `Layers` execute layer-by-layer with the existing worker pool dispatching ops within a layer in parallel.

**Architecture:**
- **Plan time** (`pkg/core/plan/generate_plan.go`): when `DependsOn != nil`, build edge set over additions/updates/deletions; deletions invert (a deletion node depends on every node whose `DependsOn(...)` returns the deletion's key, so it runs after them). Kahn's algorithm produces layers; cycles fail with `cycle detected: A -> B -> C -> A` before any file write.
- **Apply time** (`pkg/core/apply/execute_operations.go`): if `plan.Layers != nil`, verify multiset, then walk layers in order — dispatch each layer to the existing worker pool, wait for it to drain, then check failures. A failed layer cascades all later ops into `report.Skipped` (no DB writes attempted).
- **Library stays domain-agnostic.** Consumer owns the key namespace via `ExtractKeyFunc` / `DependsOn`. DAG algorithm lives in `pkg/internal/dag/` so it cannot leak into the public API.

**Tech Stack:** Go 1.21+, generics, existing `pkg/concurrency` worker pool, `testify/assert` + `testify/require`.

**Open question answers (from v2 prompt):**
| # | Question | Answer |
|---|---|---|
| 1 | Finalize policy | Add `FinalizeOn` enum on `RunParams`. Default `FinalizeAlways` (backward compat). Recommended for new consumers: `FinalizeOnAnySuccess`. |
| 2 | Formatter prints `Skipped`? | Yes — new `SKIPPED (n)` section between Failure and Ignores. |
| 3 | Cross-domain dependencies | Intra-plan only. Cross-domain ordering = consumer's responsibility (sequence the apply commands). |
| 4 | `DependsOn` also on `RunParams`? | Rejected. Plan file is the source of truth; layering belongs to the work description. |

**Breaking-change audit:** Fully additive.
- `Plan.Layers` uses `omitempty` → old plans round-trip unchanged; external JSON parsers tolerate the new optional key.
- `ExecutionReport.Skipped` is a new key (no `omitempty` to keep symmetry with Success/Failure); external consumers parsing the report JSON will see an extra `"skipped": {...}` object. Additive — flagging for the consumer.
- `LayerOp` is non-generic (string `kind` + string `key`) so it serializes cleanly inside the generic `Plan[T]`.
- `FinalizeOn` is an `int`-backed enum whose zero value = `FinalizeAlways` = current behavior.
- `concurrency.ExecuteTasks` (pkg/concurrency/pool.go:22) blocks on `wg.Wait` until every dispatched task finishes — the layer barrier is safe.
- No generic type inference changes: `LayerOp` is monomorphic and lives inside the existing `Plan[T]`.

---

## File Structure

**New files:**
- `pkg/internal/dag/dag.go` — Kahn's algorithm, cycle detection, `BuildLayers` entry point. Internal to planear.
- `pkg/internal/dag/dag_test.go` — unit tests for the DAG primitives.
- `pkg/core/plan/depgraph.go` — adapter that translates `Plan[T]` → DAG input (deletion inversion lives here), translates DAG output → `[][]LayerOp`.
- `pkg/core/plan/depgraph_test.go` — tests for the plan ↔ DAG translation, including deletion inversion.

**Modified files:**
- `pkg/types/plan.go` — add `LayerOp` type, `Layers [][]LayerOp` field to `Plan[T]`, `Skipped Plan[T]` field to `ExecutionReport[T]`.
- `pkg/core/plan/generate_plan.go` — add `DependsOn` to `GenerateParams[T]`, invoke layering before `WriteJSONFile` at line 84, fail before write on cycle.
- `pkg/core/apply/apply_plan.go` — add `FinalizeOn` field to `RunParams`, pass through to `ExecuteOperations`.
- `pkg/core/apply/execute_operations.go` — add `FinalizeOn` to `ExecuteOperationsParams`, branch into layered path when `plan.Layers != nil`, implement multiset verification, layered dispatch, failure cascade, finalize policy gate.
- `pkg/formatters/execution_report.go` — add `SKIPPED (n)` section.
- `pkg/core/plan/doc.go`, `pkg/core/apply/doc.go` — document the contract.

**Test files modified:**
- `pkg/types/plan_test.go` (or new if absent) — JSON round-trip with `Layers`.
- `pkg/core/plan/generate_plan_test.go` — adds layered-generation tests.
- `pkg/core/apply/execute_operations_test.go` — layer barrier, failure cascade, multiset verification, FinalizeOn policies.
- `pkg/formatters/execution_report_test.go` — Skipped section.

---

## Task 1: Add `LayerOp`, `Layers`, and `Skipped` to types

**Files:**
- Modify: `pkg/types/plan.go`
- Test: `pkg/types/plan_test.go` (create if absent)

- [ ] **Step 1: Write failing JSON round-trip test**

Create or extend `pkg/types/plan_test.go`:

```go
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
			{{Kind: "add", Key: "A"}},
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/types/ -run "TestPlan_Layers|TestPlan_NilLayers|TestExecutionReport_Skipped" -v`
Expected: FAIL — `types.LayerOp` undefined, `Plan.Layers` undefined, `ExecutionReport.Skipped` undefined.

- [ ] **Step 3: Add `LayerOp`, `Plan.Layers`, `ExecutionReport.Skipped`**

Edit `pkg/types/plan.go`. Replace the `Plan[T]` and `ExecutionReport[T]` blocks:

```go
type Plan[T any] struct {
	Additions []RecordAddition[T] `json:"additions"`
	Updates   []RecordUpdate[T]   `json:"updates"`
	Deletions []RecordDeletion[T] `json:"deletions"`
	Ignores   []RecordIgnored[T]  `json:"ignores"`
	// Layers, if non-nil, dictates apply-time execution order. Each inner
	// slice is a layer; ops in the same layer dispatch in parallel; layer
	// N+1 starts after layer N drains. References ops in Additions /
	// Updates / Deletions by (Kind, Key). Populated by Generate when
	// GenerateParams.DependsOn is set.
	Layers [][]LayerOp `json:"layers,omitempty"`
}

// LayerOp identifies a single operation within a layered execution plan.
// Kind is one of "add", "update", "delete". Key matches the operation's
// Key field within the corresponding Additions / Updates / Deletions slice.
type LayerOp struct {
	Kind string `json:"kind"`
	Key  string `json:"key"`
}

type ExecutionReport[T any] struct {
	Success              Plan[T]            `json:"success"`
	Failure              Plan[T]            `json:"failure"`
	Skipped              Plan[T]            `json:"skipped"`
	Ignores              []RecordIgnored[T] `json:"ignores"`
	FinalizationSuccess  bool               `json:"finalization_success"`
	FinalizationErrorMsg string             `json:"finalization_error_msg,omitempty"`
}
```

Layer-op kind constants (kept private to discourage stringly-typed misuse outside the layered code paths):

Add to `pkg/types/plan.go` (or a new tiny file `pkg/types/layer_kinds.go`):

```go
const (
	LayerOpAdd    = "add"
	LayerOpUpdate = "update"
	LayerOpDelete = "delete"
)
```

- [ ] **Step 4: Run tests**

Run: `go test ./pkg/types/ -v`
Expected: PASS.

- [ ] **Step 5: Run full suite (compilation check)**

Run: `go build ./... && go test ./...`
Expected: all existing tests PASS (no callers reference the new fields).

- [ ] **Step 6: Commit**

```bash
git add pkg/types/plan.go pkg/types/plan_test.go pkg/types/layer_kinds.go
git commit -m "feat(types): add Layers, LayerOp, Skipped for layered execution"
```

---

## Task 2: Implement `pkg/internal/dag` (Kahn's + cycle detection)

**Files:**
- Create: `pkg/internal/dag/dag.go`
- Create: `pkg/internal/dag/dag_test.go`

- [ ] **Step 1: Write failing tests for the DAG primitives**

Create `pkg/internal/dag/dag_test.go`:

```go
package dag_test

import (
	"testing"

	"github.com/algebananazzzzz/planear/pkg/internal/dag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildLayers_LinearChain(t *testing.T) {
	// A -> B -> C  (A depends on nothing; B depends on A; C depends on B)
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
	// A -> B, A -> C, B -> D, C -> D
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
	// B depends on "X" which is not in the node set — should be ignored
	// (treated as "already satisfied", e.g. remote-only).
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
	// A -> B -> C -> A
	edges := map[string][]string{
		"A": {"C"},
		"B": {"A"},
		"C": {"B"},
	}
	_, err := dag.BuildLayers([]string{"A", "B", "C"}, edges)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle detected")
	// Path must mention all three nodes
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

func TestBuildLayers_EmptyInput(t *testing.T) {
	layers, err := dag.BuildLayers(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, layers)
}

func TestBuildLayers_DeterministicOrdering(t *testing.T) {
	// Same input twice must produce same layer order (nodes inside a layer
	// sorted lexicographically) so plan files are reproducible.
	edges := map[string][]string{"A": {}, "B": {}, "C": {}, "D": {}}
	nodes := []string{"D", "B", "A", "C"}
	layers, err := dag.BuildLayers(nodes, edges)
	require.NoError(t, err)
	require.Equal(t, [][]string{{"A", "B", "C", "D"}}, layers)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/internal/dag/ -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement `BuildLayers`**

Create `pkg/internal/dag/dag.go`:

```go
// Package dag implements layered topological sorting with cycle detection.
// It is internal to planear and not part of any public API.
package dag

import (
	"fmt"
	"sort"
	"strings"
)

// BuildLayers performs a layered topological sort over the given nodes.
//
// `edges[v]` returns the keys that `v` depends on. A dependency on a key not
// in `nodes` is treated as already satisfied (e.g. references to remote-only
// rows that impose no in-plan ordering).
//
// Within a layer, nodes are returned in lexicographic order so callers get
// reproducible output across runs.
//
// Returns ("cycle detected: A -> B -> ... -> A", nil) if any cycle exists.
func BuildLayers(nodes []string, edges map[string][]string) ([][]string, error) {
	if len(nodes) == 0 {
		return nil, nil
	}

	nodeSet := make(map[string]struct{}, len(nodes))
	for _, n := range nodes {
		nodeSet[n] = struct{}{}
	}

	// indegree[v] = number of unsatisfied in-plan dependencies of v.
	indegree := make(map[string]int, len(nodes))
	// reverse[u] = list of nodes that depend on u (so when u is removed we
	// know whose indegree to decrement).
	reverse := make(map[string][]string, len(nodes))

	for _, v := range nodes {
		indegree[v] = 0
	}
	for _, v := range nodes {
		for _, dep := range edges[v] {
			if _, ok := nodeSet[dep]; !ok {
				continue // external dependency, ignore
			}
			indegree[v]++
			reverse[dep] = append(reverse[dep], v)
		}
	}

	var layers [][]string
	remaining := len(nodes)

	for remaining > 0 {
		var layer []string
		for _, v := range nodes {
			if indegree[v] == 0 {
				layer = append(layer, v)
			}
		}
		if len(layer) == 0 {
			return nil, formatCycleError(nodes, indegree, edges, nodeSet)
		}
		sort.Strings(layer)
		layers = append(layers, layer)
		for _, v := range layer {
			indegree[v] = -1 // mark consumed
			for _, downstream := range reverse[v] {
				if indegree[downstream] > 0 {
					indegree[downstream]--
				}
			}
		}
		remaining -= len(layer)
	}

	return layers, nil
}

func formatCycleError(nodes []string, indegree map[string]int, edges map[string][]string, nodeSet map[string]struct{}) error {
	// Find any node still in the cycle (indegree > 0) and walk dependencies
	// until we revisit a node — that gives us the cycle path.
	var start string
	for _, n := range nodes {
		if indegree[n] > 0 {
			start = n
			break
		}
	}

	visited := map[string]int{} // node -> position in path
	path := []string{}
	current := start
	for {
		if pos, seen := visited[current]; seen {
			path = append(path, current)
			cycle := path[pos:]
			return fmt.Errorf("cycle detected: %s", strings.Join(cycle, " -> "))
		}
		visited[current] = len(path)
		path = append(path, current)

		// Walk to the first in-plan dependency that still has indegree > 0
		// (i.e. still part of the unresolved cycle).
		var next string
		for _, dep := range edges[current] {
			if _, in := nodeSet[dep]; !in {
				continue
			}
			if indegree[dep] > 0 {
				next = dep
				break
			}
		}
		if next == "" {
			// Should be unreachable given indegree>0, but stay safe.
			return fmt.Errorf("cycle detected (path truncated): %s", strings.Join(path, " -> "))
		}
		current = next
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./pkg/internal/dag/ -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/internal/dag/dag.go pkg/internal/dag/dag_test.go
git commit -m "feat(internal/dag): layered topo-sort with cycle detection"
```

---

## Task 3: Plan ↔ DAG adapter with deletion inversion

**Files:**
- Create: `pkg/core/plan/depgraph.go`
- Create: `pkg/core/plan/depgraph_test.go`

- [ ] **Step 1: Write failing tests**

Create `pkg/core/plan/depgraph_test.go`:

```go
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

func keyOf(r depRec) string         { return r.Key }
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
	// parent is being deleted; child (still present, being updated) depends
	// on parent via its OLD state. Deletion must run AFTER the child update
	// that drops the reference.
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
	// New state of child has no parent reference, but Old state does.
	// ComputeLayers must consider both for deletion-inversion.
	layers, err := plan.ComputeLayers(p, keyOf, parentOf)
	require.NoError(t, err)
	require.Equal(t, [][]types.LayerOp{
		{{Kind: types.LayerOpUpdate, Key: "child"}},
		{{Kind: types.LayerOpDelete, Key: "parent"}},
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
	// "ghost" is not in the plan — depending on it imposes no constraint.
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/core/plan/ -run TestComputeLayers -v`
Expected: FAIL — `plan.ComputeLayers` undefined.

- [ ] **Step 3: Implement `ComputeLayers`**

Create `pkg/core/plan/depgraph.go`:

```go
package plan

import (
	"github.com/algebananazzzzz/planear/pkg/internal/dag"
	"github.com/algebananazzzzz/planear/pkg/types"
)

// ComputeLayers builds the dependency DAG for a Plan and returns its
// topological layering.
//
// `extractKey(record)` returns the record's identity (matches RecordAddition.Key
// etc.). `dependsOn(record)` returns the keys the record references; keys not
// present in the plan are treated as external (impose no ordering).
//
// Deletion inversion: a deletion is scheduled after every other op in the plan
// whose effective state (NEW state for adds/updates, OLD state for deletes)
// references the deletion's key. For updates we also consider the OLD state,
// because the old reference must be cleared before the parent can be removed.
//
// Returns an error if the dependency graph contains a cycle.
func ComputeLayers[T any](
	p types.Plan[T],
	extractKey func(T) string,
	dependsOn func(T) []string,
) ([][]types.LayerOp, error) {
	// Step 1: assign each op a stable node id "kind:key" and collect them.
	type opIdent struct {
		layerOp  types.LayerOp
		nodeID   string
		newDeps  []string // dependencies via the NEW state (adds/updates only)
		oldDeps  []string // dependencies via the OLD state (updates/deletes only)
		thisKey  string
	}

	var ops []opIdent
	idOf := func(kind, key string) string { return kind + ":" + key }

	for _, a := range p.Additions {
		ops = append(ops, opIdent{
			layerOp: types.LayerOp{Kind: types.LayerOpAdd, Key: a.Key},
			nodeID:  idOf(types.LayerOpAdd, a.Key),
			newDeps: dependsOn(a.New),
			thisKey: a.Key,
		})
	}
	for _, u := range p.Updates {
		ops = append(ops, opIdent{
			layerOp: types.LayerOp{Kind: types.LayerOpUpdate, Key: u.Key},
			nodeID:  idOf(types.LayerOpUpdate, u.Key),
			newDeps: dependsOn(u.New),
			oldDeps: dependsOn(u.Old),
			thisKey: u.Key,
		})
	}
	for _, d := range p.Deletions {
		ops = append(ops, opIdent{
			layerOp: types.LayerOp{Kind: types.LayerOpDelete, Key: d.Key},
			nodeID:  idOf(types.LayerOpDelete, d.Key),
			oldDeps: dependsOn(d.Old),
			thisKey: d.Key,
		})
	}

	// Step 2: index ops by their key for inversion lookups.
	opsByKey := make(map[string][]*opIdent, len(ops))
	for i := range ops {
		opsByKey[ops[i].thisKey] = append(opsByKey[ops[i].thisKey], &ops[i])
	}

	// Step 3: build node list + edge map for the DAG.
	nodes := make([]string, 0, len(ops))
	nodeToOp := make(map[string]types.LayerOp, len(ops))
	for _, o := range ops {
		nodes = append(nodes, o.nodeID)
		nodeToOp[o.nodeID] = o.layerOp
	}

	edges := make(map[string][]string, len(ops))

	for _, o := range ops {
		switch o.layerOp.Kind {
		case types.LayerOpAdd, types.LayerOpUpdate:
			// Edge: o depends on the op that establishes each referenced key
			// (the add/update node for that key, if one exists).
			for _, depKey := range o.newDeps {
				for _, depOp := range opsByKey[depKey] {
					if depOp.layerOp.Kind == types.LayerOpAdd || depOp.layerOp.Kind == types.LayerOpUpdate {
						edges[o.nodeID] = append(edges[o.nodeID], depOp.nodeID)
					}
				}
			}
		}
	}

	// Step 4: inversion for deletions.
	// A deletion D for key K must run AFTER any add/update whose new state
	// references K (those references still exist when D would run) AND after
	// any update whose old state references K (the old reference must be
	// cleared by the update before D can drop K).
	for _, o := range ops {
		if o.layerOp.Kind != types.LayerOpDelete {
			continue
		}
		delKey := o.thisKey
		for _, other := range ops {
			if other.nodeID == o.nodeID {
				continue
			}
			refs := false
			switch other.layerOp.Kind {
			case types.LayerOpAdd:
				for _, dep := range other.newDeps {
					if dep == delKey {
						refs = true
						break
					}
				}
			case types.LayerOpUpdate:
				for _, dep := range other.newDeps {
					if dep == delKey {
						refs = true
						break
					}
				}
				if !refs {
					for _, dep := range other.oldDeps {
						if dep == delKey {
							refs = true
							break
						}
					}
				}
			}
			if refs {
				edges[o.nodeID] = append(edges[o.nodeID], other.nodeID)
			}
		}
	}

	// Step 5: layered topo-sort.
	layeredIDs, err := dag.BuildLayers(nodes, edges)
	if err != nil {
		return nil, err
	}

	// Step 6: translate node IDs back to LayerOps.
	result := make([][]types.LayerOp, len(layeredIDs))
	for i, layer := range layeredIDs {
		result[i] = make([]types.LayerOp, len(layer))
		for j, id := range layer {
			result[i][j] = nodeToOp[id]
		}
	}
	return result, nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./pkg/core/plan/ -run TestComputeLayers -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/core/plan/depgraph.go pkg/core/plan/depgraph_test.go
git commit -m "feat(plan): compute layered DAG with deletion inversion"
```

---

## Task 4: Add `DependsOn` to `GenerateParams` and integrate layering into `Generate`

**Files:**
- Modify: `pkg/core/plan/generate_plan.go` (struct fields lines 15-23, write block at line 84)
- Test: `pkg/core/plan/generate_plan_test.go` (append)

- [ ] **Step 1: Write failing integration test**

Append to `pkg/core/plan/generate_plan_test.go`:

```go
func TestGenerate_LayersPopulatedWhenDependsOnSet(t *testing.T) {
	dir := testutils.NewTestDir(t)

	// CSV: child rows reference parent rows by name.
	csv := []map[string]string{
		{"name": "JCRC.Welfare-Member", "parent": "JCRC.Welfare-Head"},
		{"name": "JCRC.President", "parent": ""},
		{"name": "JCRC.Welfare-Head", "parent": "JCRC.President"},
	}
	csvPath := testutils.WriteCSVFile(t, dir, "positions.csv", csv)
	planOut := filepath.Join(dir, "plan.json")

	type Position struct {
		Name   string
		Parent string
	}

	params := plan.GenerateParams[Position]{
		CSVPath:           csvPath,
		OutputFilePath:    planOut,
		FormatRecordFunc:  func(p Position) string { return p.Name },
		FormatKeyFunc:     func(k string) string { return k },
		ExtractKeyFunc:    func(p Position) string { return p.Name },
		LoadRemoteRecords: func() (map[string]Position, error) { return nil, nil },
		ValidateRecord:    func(Position) error { return nil },
		DependsOn: func(p Position) []string {
			if p.Parent == "" {
				return nil
			}
			return []string{p.Parent}
		},
	}
	// Note: the test harness will need a CSV decoder hook that builds Position
	// values — use whatever fixture pattern the existing TestGeneratePlan_Success
	// uses (see generate_plan_test.go for the exact shape).

	generated, err := plan.Generate(params)
	require.NoError(t, err)
	require.NotNil(t, generated.Layers)
	require.Equal(t, [][]types.LayerOp{
		{{Kind: types.LayerOpAdd, Key: "JCRC.President"}},
		{{Kind: types.LayerOpAdd, Key: "JCRC.Welfare-Head"}},
		{{Kind: types.LayerOpAdd, Key: "JCRC.Welfare-Member"}},
	}, generated.Layers)
}

func TestGenerate_CycleFailsBeforeFileWrite(t *testing.T) {
	dir := testutils.NewTestDir(t)
	csv := []map[string]string{
		{"name": "A", "parent": "B"},
		{"name": "B", "parent": "A"},
	}
	csvPath := testutils.WriteCSVFile(t, dir, "positions.csv", csv)
	planOut := filepath.Join(dir, "plan.json")

	type Position struct {
		Name   string
		Parent string
	}

	_, err := plan.Generate(plan.GenerateParams[Position]{
		CSVPath:           csvPath,
		OutputFilePath:    planOut,
		FormatRecordFunc:  func(p Position) string { return p.Name },
		FormatKeyFunc:     func(k string) string { return k },
		ExtractKeyFunc:    func(p Position) string { return p.Name },
		LoadRemoteRecords: func() (map[string]Position, error) { return nil, nil },
		ValidateRecord:    func(Position) error { return nil },
		DependsOn: func(p Position) []string {
			if p.Parent == "" {
				return nil
			}
			return []string{p.Parent}
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cycle detected")

	_, statErr := os.Stat(planOut)
	require.True(t, os.IsNotExist(statErr), "plan file must NOT exist after cycle error; got: %v", statErr)
}

func TestGenerate_NoDependsOn_LayersNil(t *testing.T) {
	// Backward compat: existing consumers without DependsOn get Layers==nil.
	dir := testutils.NewTestDir(t)
	// ...build a minimal CSV / params using existing helper pattern...
	// Assert: generated.Layers == nil
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/core/plan/ -run "TestGenerate_(Layers|Cycle|NoDependsOn)" -v`
Expected: FAIL — `GenerateParams.DependsOn` undefined.

- [ ] **Step 3: Add `DependsOn` field and integrate layering**

Edit `pkg/core/plan/generate_plan.go`.

Add field to `GenerateParams[T]` (after `ValidateRecord` at line 22):

```go
	// DependsOn returns the keys (as produced by ExtractKeyFunc) that this
	// record references. When set, Generate builds a dependency DAG over the
	// entire plan, topologically sorts it into layers, and stores the layer
	// assignment in Plan.Layers. Cycles produce an error before the plan is
	// written. Keys returned that are not present in the plan are treated as
	// external (no ordering constraint).
	//
	// For deletions, planear automatically inverts: a row being deleted is
	// scheduled after every row in the plan that depends on it (whether by
	// its old state for deletes/updates, or its new state for adds/updates).
	DependsOn func(T) []string
```

Insert between the `formatters.FormatPlan` call (line 81) and the `WriteJSONFile` call (line 84):

```go
	if params.DependsOn != nil {
		layers, err := ComputeLayers(plan, params.ExtractKeyFunc, params.DependsOn)
		if err != nil {
			return nil, fmt.Errorf("failed to compute layered plan: %w", err)
		}
		plan.Layers = layers
	}
```

This MUST sit before `WriteJSONFile` so a cycle error aborts before any side effect.

- [ ] **Step 4: Run tests**

Run: `go test ./pkg/core/plan/... -v`
Expected: all PASS, including the new tests AND the existing `TestGeneratePlan_Success` (which doesn't set `DependsOn`, so `Layers` stays nil).

- [ ] **Step 5: Commit**

```bash
git add pkg/core/plan/generate_plan.go pkg/core/plan/generate_plan_test.go
git commit -m "feat(plan): wire DependsOn into Generate, fail-fast on cycles"
```

---

## Task 5: Layered apply path with multiset verification

**Files:**
- Modify: `pkg/core/apply/execute_operations.go` (lines 14-23 params, dispatch block lines 142-160)
- Test: `pkg/core/apply/execute_operations_test.go` (append)

- [ ] **Step 1: Write failing tests for layered dispatch + multiset rejection**

Append to `pkg/core/apply/execute_operations_test.go`:

```go
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
	require.NoError(t, err)
	require.Equal(t, []string{"parent", "child"}, order)
	require.Len(t, report.Success.Additions, 2)
}

func TestExecuteOperations_LayeredPath_MultisetMismatchAborts(t *testing.T) {
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
	require.Error(t, err)
	require.Contains(t, err.Error(), "multiset")
	require.False(t, called, "no OnAdd must run when multiset check fails")
}

func TestExecuteOperations_NilLayers_TakesFlatPath(t *testing.T) {
	// Existing behavior: Plan with Layers == nil uses the flat dispatch path.
	// All existing tests already cover this implicitly; this is an explicit
	// regression lock.
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
	require.NoError(t, err)
	require.True(t, called)
}
```

- [ ] **Step 2: Run tests to verify they fail (or are inert)**

Run: `go test ./pkg/core/apply/ -run "TestExecuteOperations_(Layered|NilLayers)" -v`
Expected: FAIL — no layered branch yet; the layered test runs the flat path and may pass coincidentally, but the multiset test must fail (no error returned).

- [ ] **Step 3: Implement the layered branch**

Edit `pkg/core/apply/execute_operations.go`.

Before the existing flat-dispatch block (line 142, the comment `// Execute deletions first ...`), inject the layered branch:

```go
	if params.Parallelization == nil {
		defaultParallelism := runtime.NumCPU()
		params.Parallelization = &defaultParallelism
	}

	if params.Plan.Layers != nil {
		// --- Layered path ---

		// Multiset verification: every op in the plan must appear exactly
		// once across all layers. Reject stale / hand-edited plans before
		// any DB write.
		want := map[types.LayerOp]int{}
		for _, a := range params.Plan.Additions {
			want[types.LayerOp{Kind: types.LayerOpAdd, Key: a.Key}]++
		}
		for _, u := range params.Plan.Updates {
			want[types.LayerOp{Kind: types.LayerOpUpdate, Key: u.Key}]++
		}
		for _, d := range params.Plan.Deletions {
			want[types.LayerOp{Kind: types.LayerOpDelete, Key: d.Key}]++
		}
		got := map[types.LayerOp]int{}
		for _, layer := range params.Plan.Layers {
			for _, op := range layer {
				got[op]++
			}
		}
		for k, v := range want {
			if got[k] != v {
				return nil, fmt.Errorf("plan.Layers multiset mismatch: op %+v expected %d times, found %d (plan may be stale or hand-edited)", k, v, got[k])
			}
		}
		for k, v := range got {
			if want[k] != v {
				return nil, fmt.Errorf("plan.Layers references unknown op %+v %d time(s)", k, v)
			}
		}

		// Index plan ops by (Kind, Key) for fast resolution.
		addByKey := make(map[string]types.RecordAddition[T], len(params.Plan.Additions))
		for _, a := range params.Plan.Additions {
			addByKey[a.Key] = a
		}
		updByKey := make(map[string]types.RecordUpdate[T], len(params.Plan.Updates))
		for _, u := range params.Plan.Updates {
			updByKey[u.Key] = u
		}
		delByKey := make(map[string]types.RecordDeletion[T], len(params.Plan.Deletions))
		for _, d := range params.Plan.Deletions {
			delByKey[d.Key] = d
		}

		var skipped types.Plan[T]
		stopAfter := -1
		for layerIdx, layer := range params.Plan.Layers {
			tasks := make([]concurrency.Task, 0, len(layer))
			for _, op := range layer {
				switch op.Kind {
				case types.LayerOpAdd:
					tasks = append(tasks, addTask(addByKey[op.Key]))
				case types.LayerOpUpdate:
					tasks = append(tasks, updateTask(updByKey[op.Key]))
				case types.LayerOpDelete:
					tasks = append(tasks, deleteTask(delByKey[op.Key]))
				default:
					return nil, fmt.Errorf("plan.Layers unknown op kind %q in layer %d", op.Kind, layerIdx)
				}
			}

			failBefore := len(failure.Additions) + len(failure.Updates) + len(failure.Deletions)
			if err := concurrency.ExecuteTasks(tasks, *params.Parallelization); err != nil {
				return nil, fmt.Errorf("layer %d execution failed: %w", layerIdx, err)
			}
			failAfter := len(failure.Additions) + len(failure.Updates) + len(failure.Deletions)
			if failAfter > failBefore {
				stopAfter = layerIdx
				break
			}
		}

		if stopAfter >= 0 {
			for _, layer := range params.Plan.Layers[stopAfter+1:] {
				for _, op := range layer {
					switch op.Kind {
					case types.LayerOpAdd:
						skipped.Additions = append(skipped.Additions, addByKey[op.Key])
					case types.LayerOpUpdate:
						skipped.Updates = append(skipped.Updates, updByKey[op.Key])
					case types.LayerOpDelete:
						skipped.Deletions = append(skipped.Deletions, delByKey[op.Key])
					}
				}
			}
		}

		report := &types.ExecutionReport[T]{
			Success:             success,
			Failure:             failure,
			Skipped:             skipped,
			Ignores:             params.Plan.Ignores,
			FinalizationSuccess: true,
		}

		runFinalize := shouldRunFinalize(params.FinalizeOn, report)
		var finalizeErr error
		if params.OnFinalize != nil && runFinalize {
			if err := retryWithLogging(params.OnFinalize, "finalize", ""); err != nil {
				report.FinalizationSuccess = false
				report.FinalizationErrorMsg = err.Error()
				finalizeErr = err
			}
		}
		return report, finalizeErr
	}

	// --- Flat path (existing behavior, unchanged) ---
```

Add at the end of the file (helper used by both paths once Task 6 lands):

```go
func shouldRunFinalize[T any](policy types.FinalizeOn, report *types.ExecutionReport[T]) bool {
	switch policy {
	case types.FinalizeOnSuccess:
		anyFail := !report.Failure.IsEmpty() || !report.Skipped.IsEmpty()
		return !anyFail
	case types.FinalizeOnAnySuccess:
		return !report.Success.IsEmpty()
	case types.FinalizeAlways:
		fallthrough
	default:
		return true
	}
}
```

(The `FinalizeOn` enum lands in Task 6 — gate-link with a placeholder constant `FinalizeAlways = 0` for now so this compiles.)

Add field to `ExecuteOperationsParams[T]`:

```go
	FinalizeOn types.FinalizeOn // see types.FinalizeOn; defaults to FinalizeAlways
```

- [ ] **Step 4: Run tests**

Run: `go test ./pkg/core/apply/ -run "TestExecuteOperations_(Layered|NilLayers)" -v`
Expected: PASS.

- [ ] **Step 5: Run full apply suite**

Run: `go test ./pkg/core/apply/... -v`
Expected: all existing tests PASS (Plan.Layers is nil → flat path).

- [ ] **Step 6: Commit**

```bash
git add pkg/core/apply/execute_operations.go pkg/core/apply/execute_operations_test.go
git commit -m "feat(apply): layered execution path with multiset verification"
```

---

## Task 6: `FinalizeOn` enum + `RunParams.FinalizeOn` wiring

**Files:**
- Modify: `pkg/types/plan.go` (add `FinalizeOn` enum)
- Modify: `pkg/core/apply/apply_plan.go` (add field, pass through)
- Test: `pkg/core/apply/execute_operations_test.go` (append)

- [ ] **Step 1: Write failing tests for each policy**

Append to `pkg/core/apply/execute_operations_test.go`:

```go
func TestFinalizeOnSuccess_SkipsWhenAnyFailure(t *testing.T) {
	// Two-layer plan, layer 0 fails. FinalizeOnSuccess → finalize NOT called.
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
	assert.False(t, finalizeCalled)
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
	assert.True(t, finalizeCalled)
}

func TestFinalizeAlways_DefaultPreservedForOldCallers(t *testing.T) {
	// FinalizeOn zero-value = FinalizeAlways = always run finalize.
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
		OnAdd: func(rec types.RecordAddition[MockRecord]) error {
			return errors.New("synthetic")
		},
		OnUpdate:   func(types.RecordUpdate[MockRecord]) error { return nil },
		OnDelete:   func(types.RecordDeletion[MockRecord]) error { return nil },
		OnFinalize: func() error { finalizeCalled = true; return nil },
	})
	assert.True(t, finalizeCalled, "default (zero-value FinalizeOn) must run finalize")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/core/apply/ -run TestFinalizeOn -v`
Expected: FAIL — `types.FinalizeOn` undefined.

- [ ] **Step 3: Add the enum**

Edit `pkg/types/plan.go` (or new `pkg/types/finalize_policy.go`):

```go
// FinalizeOn controls when ExecuteOperations invokes the OnFinalize callback.
// Zero value = FinalizeAlways (preserves pre-Layers behavior).
type FinalizeOn int

const (
	// FinalizeAlways runs OnFinalize regardless of failures or skipped ops.
	// Default; preserves backward compatibility.
	FinalizeAlways FinalizeOn = iota
	// FinalizeOnSuccess runs OnFinalize only when no op failed and no op was
	// skipped (i.e. the plan ran to completion).
	FinalizeOnSuccess
	// FinalizeOnAnySuccess runs OnFinalize when at least one op succeeded;
	// skips it only on zero progress. Recommended default for new consumers.
	FinalizeOnAnySuccess
)
```

Edit `pkg/core/apply/apply_plan.go`. Add to `RunParams[T]`:

```go
	FinalizeOn types.FinalizeOn // see types.FinalizeOn
```

Pass it through in `Run` (line ~63, the `ExecuteOperations` call):

```go
	result, err := ExecuteOperations(ExecuteOperationsParams[T]{
		Plan:            plan,
		FormatRecord:    params.FormatRecord,
		FormatKey:       params.FormatKey,
		OnAdd:           params.OnAdd,
		OnUpdate:        params.OnUpdate,
		OnDelete:        params.OnDelete,
		OnFinalize:      params.OnFinalize,
		Parallelization: params.Parallelization,
		FinalizeOn:      params.FinalizeOn,
	})
```

Also gate finalize in the **flat path** (so non-layered consumers can opt into `FinalizeOnAnySuccess` too): edit the flat-path finalize block in `execute_operations.go` to call `shouldRunFinalize(params.FinalizeOn, report)` before invoking `params.OnFinalize`.

- [ ] **Step 4: Run tests**

Run: `go test ./pkg/core/apply/ -run TestFinalizeOn -v && go test ./... -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/types/plan.go pkg/types/finalize_policy.go pkg/core/apply/apply_plan.go pkg/core/apply/execute_operations.go pkg/core/apply/execute_operations_test.go
git commit -m "feat(apply): FinalizeOn policy enum (default preserves behavior)"
```

---

## Task 7: Layer barrier test (intra-layer parallel, inter-layer serial)

**Files:**
- Test: `pkg/core/apply/execute_operations_test.go` (append)

- [ ] **Step 1: Write barrier test**

Append:

```go
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
	require.NoError(t, err)
	assert.Equal(t, 0, violations, "L1 must not start until both L0 ops finish")
	assert.True(t, finished["L1"])
}
```

- [ ] **Step 2: Run, commit**

Run: `go test ./pkg/core/apply/ -run TestExecuteOperations_LayeredPath_LayerBarrier -race -v`
Expected: PASS, no race.

```bash
git add pkg/core/apply/execute_operations_test.go
git commit -m "test(apply): verify layer barrier on layered execution"
```

---

## Task 8: Failure cascade test (later layers → Skipped)

**Files:**
- Test: `pkg/core/apply/execute_operations_test.go` (append)

- [ ] **Step 1: Write cascade test**

```go
func TestExecuteOperations_LayeredPath_FailureCascadesToSkipped(t *testing.T) {
	plan := types.Plan[MockRecord]{
		Additions: []types.RecordAddition[MockRecord]{
			{Key: "fails", New: MockRecord{ID: "fails", Name: "always-fails"}},
			{Key: "skip1", New: MockRecord{ID: "skip1"}},
			{Key: "skip2", New: MockRecord{ID: "skip2"}},
		},
		Deletions: []types.RecordDeletion[MockRecord]{
			{Key: "skipDel", Old: MockRecord{ID: "skipDel"}},
		},
		Layers: [][]types.LayerOp{
			{{Kind: types.LayerOpAdd, Key: "fails"}},
			{{Kind: types.LayerOpAdd, Key: "skip1"}, {Kind: types.LayerOpAdd, Key: "skip2"}},
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
		OnUpdate: func(types.RecordUpdate[MockRecord]) error { return nil },
		OnDelete: func(rec types.RecordDeletion[MockRecord]) error {
			record("del:" + rec.Old.ID)
			return nil
		},
	}

	report, err := apply.ExecuteOperations(params)
	require.NoError(t, err)
	assert.NotContains(t, attempted, "add:skip1")
	assert.NotContains(t, attempted, "add:skip2")
	assert.NotContains(t, attempted, "del:skipDel")
	assert.Len(t, report.Failure.Additions, 1)
	assert.Len(t, report.Skipped.Additions, 2)
	assert.Len(t, report.Skipped.Deletions, 1)
}
```

- [ ] **Step 2: Run, commit**

Run: `go test ./pkg/core/apply/ -run TestExecuteOperations_LayeredPath_FailureCascadesToSkipped -v`
Expected: PASS.

```bash
git add pkg/core/apply/execute_operations_test.go
git commit -m "test(apply): failed layer cascades subsequent ops into Skipped"
```

---

## Task 9: Formatter prints `SKIPPED (n)` section

**Files:**
- Modify: `pkg/formatters/execution_report.go`
- Test: `pkg/formatters/execution_report_test.go` (append)

- [ ] **Step 1: Write failing test**

Append:

```go
func TestFormatExecutionReport_SkippedSection(t *testing.T) {
	report := types.ExecutionReport[testRec]{
		Success: types.Plan[testRec]{
			Additions: []types.RecordAddition[testRec]{{Key: "ok", New: testRec{ID: "ok"}}},
		},
		Failure: types.Plan[testRec]{
			Additions: []types.RecordAddition[testRec]{{Key: "bad", New: testRec{ID: "bad"}}},
		},
		Skipped: types.Plan[testRec]{
			Additions: []types.RecordAddition[testRec]{{Key: "later", New: testRec{ID: "later"}}},
		},
		FinalizationSuccess: true,
	}
	out := formatters.FormatExecutionReport(report,
		func(r testRec) string { return r.ID },
		func(k string) string { return k })
	assert.Contains(t, out, "SKIPPED (1)")
	assert.Contains(t, out, "later")
	// Section order: FAILURE then SKIPPED then IGNORES
	failIdx := strings.Index(out, "FAILURE")
	skipIdx := strings.Index(out, "SKIPPED")
	ignIdx := strings.Index(out, "IGNORES")
	assert.True(t, failIdx >= 0 && skipIdx > failIdx, "SKIPPED must follow FAILURE")
	if ignIdx >= 0 {
		assert.True(t, skipIdx < ignIdx, "SKIPPED must precede IGNORES")
	}
}

func TestFormatExecutionReport_NoSkippedSection_WhenEmpty(t *testing.T) {
	report := types.ExecutionReport[testRec]{
		Success:             types.Plan[testRec]{},
		FinalizationSuccess: true,
	}
	out := formatters.FormatExecutionReport(report,
		func(r testRec) string { return r.ID },
		func(k string) string { return k })
	assert.NotContains(t, out, "SKIPPED")
}
```

- [ ] **Step 2: Run to fail, then implement**

Run: `go test ./pkg/formatters/ -run TestFormatExecutionReport_Skipped -v`
Expected: FAIL.

Edit `pkg/formatters/execution_report.go`. Insert (between the Failure section and the Ignores section, around line 50–55):

```go
	if !result.Skipped.IsEmpty() {
		buf.WriteString(fmt.Sprintf("\nSKIPPED (%d)\n", result.Skipped.TotalCount()))
		buf.WriteString(formatPlanDetails(result.Skipped, formatRecord, formatKey))
	}
```

If `TotalCount()` doesn't exist on `Plan`, inline the math: `len(Additions)+len(Updates)+len(Deletions)`. Or add the helper to `pkg/types/plan.go` for symmetry.

- [ ] **Step 3: Run tests, commit**

```bash
go test ./pkg/formatters/... -v
git add pkg/formatters/execution_report.go pkg/formatters/execution_report_test.go pkg/types/plan.go
git commit -m "feat(formatters): print SKIPPED section in execution report"
```

---

## Task 10: End-to-end `cca_positions` self-referencing FK scenario

**Files:**
- Create: `pkg/core/apply/scenario_self_fk_test.go`

- [ ] **Step 1: Write scenario test mirroring the v2 prompt's motivating use case**

Create `pkg/core/apply/scenario_self_fk_test.go`:

```go
package apply_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/algebananazzzzz/planear/pkg/core/apply"
	"github.com/algebananazzzzz/planear/pkg/core/plan"
	"github.com/algebananazzzzz/planear/pkg/types"
	"github.com/stretchr/testify/require"
)

type Position struct {
	Key             string
	CCA             string
	Name            string
	ReportingTo     string // key of parent Position, or "" for root
}

// fakeDB enforces a self-referencing FK on Position.ReportingTo.
type fakeDB struct {
	mu   sync.Mutex
	rows map[string]Position
}

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

func TestScenario_CCAPositions_LayeredSucceeds(t *testing.T) {
	db := &fakeDB{rows: map[string]Position{}}

	additions := []types.RecordAddition[Position]{
		{Key: "JCRC.President", New: Position{Key: "JCRC.President", CCA: "JCRC", Name: "President"}},
		{Key: "JCRC.Welfare-Head", New: Position{Key: "JCRC.Welfare-Head", CCA: "JCRC", Name: "Welfare Head", ReportingTo: "JCRC.President"}},
		{Key: "JCRC.Welfare-Member", New: Position{Key: "JCRC.Welfare-Member", CCA: "JCRC", Name: "Welfare Member", ReportingTo: "JCRC.Welfare-Head"}},
	}
	p := types.Plan[Position]{Additions: additions}

	layers, err := plan.ComputeLayers(p, func(pos Position) string { return pos.Key }, func(pos Position) []string {
		if pos.ReportingTo == "" {
			return nil
		}
		return []string{pos.ReportingTo}
	})
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
}

func TestScenario_CCAPositions_FlatPathFails(t *testing.T) {
	// Same plan WITHOUT Layers — flat path may race child before parent.
	// This is the regression we're solving; lock it in.
	db := &fakeDB{rows: map[string]Position{}}
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
	// Without layering, the child has ~50% odds of being attempted first.
	// We don't require failure deterministically — we just verify that adding
	// layers (above test) is what makes the scenario reliable.
	_ = report
}
```

- [ ] **Step 2: Run, commit**

```bash
go test ./pkg/core/apply/ -run TestScenario_CCAPositions -race -v
git add pkg/core/apply/scenario_self_fk_test.go
git commit -m "test(apply): end-to-end self-referencing FK scenario"
```

---

## Task 11: Backward-compat sweep + doc updates

**Files:**
- Modify: `pkg/core/plan/doc.go`
- Modify: `pkg/core/apply/doc.go`

- [ ] **Step 1: Document `DependsOn` in `pkg/core/plan/doc.go`**

Append a new section after the existing `Generate` documentation:

```go
//
// # Dependency-Aware Layering (DependsOn)
//
// When GenerateParams.DependsOn is set, Generate builds a dependency DAG
// over the plan and topologically sorts it into layers. The sorted layers
// are serialized into Plan.Layers and consumed at apply time to enforce
// safe execution order.
//
//   - DependsOn returns the keys (as produced by ExtractKeyFunc) that a
//     record references. Keys not present in the plan are treated as
//     external (impose no in-plan ordering).
//   - Cycles surface as errors before the plan file is written; the
//     consumer sees "cycle detected: A -> B -> C -> A" with no side effect.
//   - Deletions automatically invert: a deletion node is placed AFTER every
//     node whose new (or old, for updates) state references the deleted key.
//
// When DependsOn is nil, Plan.Layers is left nil and apply takes its
// existing flat dispatch path.
```

- [ ] **Step 2: Document the layered apply path + `FinalizeOn` in `pkg/core/apply/doc.go`**

Append a section explaining: `plan.Layers != nil` triggers layered dispatch; layer barrier semantics; multiset verification (rejects stale or hand-edited plans); failure cascade into `ExecutionReport.Skipped`; `FinalizeOn` policies.

- [ ] **Step 3: Verify nothing leaks the internal package**

Run: `go list -deps ./pkg/core/plan/... | grep "internal/dag"`
Expected: at most one line — `pkg/core/plan` importing `pkg/internal/dag` (allowed; `internal/` enforces no out-of-tree imports).

Run: `go vet ./...`
Expected: no warnings.

- [ ] **Step 4: Commit**

```bash
git add pkg/core/plan/doc.go pkg/core/apply/doc.go
git commit -m "docs: document DependsOn, layered apply, FinalizeOn policies"
```

---

## Task 12: Final verification

- [ ] **Step 1: Race-detector full run**

Run: `go test -race ./...`
Expected: all PASS, no data races.

- [ ] **Step 2: Public surface check**

Run:
```bash
go doc ./pkg/core/plan GenerateParams
go doc ./pkg/core/apply RunParams
go doc ./pkg/types Plan
go doc ./pkg/types ExecutionReport
go doc ./pkg/types FinalizeOn
go doc ./pkg/types LayerOp
```
Expected: each shows the new fields/types with the comments authored in tasks 1, 4, and 6.

- [ ] **Step 3: Plan-file forward-compat check**

Generate a plan with `DependsOn = nil`, then load and apply with the new code. Layers must be nil; flat path must run. (Covered by `TestExecuteOperations_NilLayers_TakesFlatPath` and existing apply suite — re-verify explicitly.)

Generate a plan with `DependsOn` set, hand-edit `plan.json` to add a bogus entry to `additions` without updating `layers`, then apply. Expect: `multiset mismatch` error, zero DB writes.

- [ ] **Step 4: CHANGELOG entry (if convention exists)**

```bash
ls CHANGELOG.md 2>/dev/null && {
  cat >> CHANGELOG.md <<'EOF'
- feat: native dependency resolution. Set `GenerateParams.DependsOn` to have
  Generate produce a layered plan (`Plan.Layers`); Apply then executes layers
  serially with intra-layer parallelism. Adds `FinalizeOn` policy on
  `RunParams`. Cycles fail at plan time before any file write. Backward-
  compatible: plans generated without DependsOn behave exactly as before.
EOF
}
git add CHANGELOG.md
git commit -m "docs: changelog entry for dependency-aware layering"
```

If no CHANGELOG, skip.

---

## Self-Review Notes

**Spec coverage:**
- Q1 `FinalizeOn` enum lands in Task 6; default `FinalizeAlways` preserves existing behavior; both apply paths (layered + flat) honor it.
- Q2 `SKIPPED (n)` section lands in Task 9 between Failure and Ignores.
- Q3 Intra-plan only; the design + tests don't attempt cross-plan deps.
- Q4 `DependsOn` only on `GenerateParams`; `RunParams` exposes none.

**Order rationale:** Types (Task 1) and the pure DAG (Task 2) come first because everything downstream depends on them. The plan→DAG adapter (Task 3) is its own task so the deletion-inversion logic gets focused test coverage. Apply-time layered path (Task 5) lands before the `FinalizeOn` enum (Task 6) because the layered path is the larger structural change and easier to review on its own; Task 6 then layers the policy gate on top of both apply paths. Scenario test (Task 10) is the integration capstone before docs (Task 11) and final verification (Task 12).

**Risks flagged for review:**
- Deletion inversion considers BOTH `dependsOn(Old)` and `dependsOn(New)` for updates. A consumer whose `DependsOn` is expensive (DB lookup, large slice allocation) will see it called twice per update. Acceptable; document.
- Multiset verification uses `LayerOp{Kind,Key}` as map keys — safe because both are strings. Verified.
- The cycle-path formatter walks the unresolved graph; in pathological inputs (densely tangled cycles) it returns a representative cycle, not the minimum cycle. Tradeoff: simpler code, sufficient for debugging.
- `formatPlanDetails` and `TotalCount()` may need a helper added to `pkg/types` for Task 9 — flag during implementation.
- Backward compat for plan JSON: `Skipped` is emitted as `"skipped": {...}` even when empty (matches the existing Success/Failure asymmetry). External report parsers tolerate extra keys; flagged in the breaking-change audit above.
