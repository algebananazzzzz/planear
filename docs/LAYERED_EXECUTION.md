# Dependency-Aware Layered Execution

Planear can topologically sort a plan into **layers** so that operations with
foreign-key (or any user-defined) dependencies execute in a safe order, while
independent operations inside a layer still run in parallel. This document
describes how it works end-to-end: what you wire up, how the DAG is built, how
deletions are inverted, and what apply does with the result.

If you only want the quick version, see the *Quick Start* section at the bottom.

---

## Why layers?

The default (flat) apply path dispatches every operation to a worker pool with
no ordering guarantee. That is fine when records are independent, but breaks
the moment one record references another:

- An `INSERT` for a child row racing an `INSERT` for its parent → FK violation.
- A `DELETE` for a parent row racing a `DELETE` for its child → FK violation,
  or worse, an orphan if the child somehow lands second on a non-enforcing
  store.
- An `UPDATE` that re-points a child away from a parent must land *before* the
  parent is deleted, otherwise the parent-delete fails.

Layering solves this by:

1. Building a directed acyclic graph (DAG) over the plan's operations from a
   user-supplied `DependsOn` function.
2. Sorting that DAG into layers via Kahn's algorithm.
3. Executing layers sequentially, with full intra-layer parallelism and a hard
   barrier between layers.

When any layer fails, subsequent layers are **not attempted** — their ops land
in `ExecutionReport.Skipped` instead of `Failure`. This keeps a partial apply
from leaving the system in an inconsistent state.

---

## Wiring it up

You opt in at plan-generation time by setting `GenerateParams.DependsOn`:

```go
plan, err := plan.Generate(plan.GenerateParams[Position]{
    CSVPath:           "./data",
    OutputFilePath:    "plan.json",
    ExtractKeyFunc:    func(p Position) string { return p.ID },
    LoadRemoteRecords: loadFromDB,
    ValidateRecord:    validate,
    FormatRecordFunc:  formatRecord,
    FormatKeyFunc:     formatKey,

    // Tell planear which keys this record references.
    DependsOn: func(p Position) []string {
        if p.ReportingTo == "" {
            return nil
        }
        return []string{p.ReportingTo}
    },
})
```

Contract for `DependsOn`:

- Return the keys (as produced by `ExtractKeyFunc`) that this record
  references. Order does not matter.
- Return `nil` for records with no dependencies.
- Keys not present anywhere in the plan are treated as **external** — they
  impose no in-plan ordering. This is what lets you reference rows that
  already exist in the remote system without forcing them into the plan.

When `DependsOn` is `nil`, `Plan.Layers` is left `nil` and apply falls back to
the flat dispatch path. Existing code paths are unchanged.

`Generate` calls `ComputeLayers` after the diff is built, populates
`plan.Layers`, and only then writes the plan file. If the dependency graph
contains a cycle, you get an error of the form `cycle detected: A -> B -> C -> A`
**before** the file is written — no partial state escapes.

At apply time, no extra configuration is needed: `apply.Run` notices that
`plan.Layers` is non-nil and switches to the layered dispatch path
automatically.

---

## How `ComputeLayers` builds the DAG

Source: `pkg/core/plan/depgraph.go`.

### Node identity

Each operation in the plan becomes a node. A node ID encodes both the kind and
the key, e.g. `add:user-42`, `update:user-42`, `delete:user-42`. Kinds are
typed as `LayerOpKind` (string-backed so the JSON wire format stays
`"add"`/`"update"`/`"delete"`).

Plan invariant: a given key appears in at most one of Additions or Updates
(but a key in Deletions is in a separate namespace from any add/update of a
matching key — see the deletion-inversion rules below).

### Edges for additions and updates

For each addition or update node, planear walks `DependsOn(record.New)` and
adds an edge `node -> dep_node` for every dep key that is itself an add or
update in the plan. The edge means "this node must run after the dep node."

```
add:child  --depends-on-->  add:parent
update:x   --depends-on-->  update:y    // x.New references y
```

External deps (keys not present as add/update in the plan) are silently
skipped.

### Deletion inversion

Deletions reverse the direction of the FK arrow. If a row references something
that is being deleted, that reference must be removed *first*, then the
deletion can run. `ComputeLayers` encodes this by adding edges **from the
deletion node to every other op whose effective state references the deleted
key**:

| Other op kind | What is checked              | Why                                           |
| ------------- | ---------------------------- | --------------------------------------------- |
| `Add`         | `DependsOn(other.New)`       | New child must be inserted before parent dies |
| `Update`      | `DependsOn(other.New)` **and** `DependsOn(other.Old)` | Old reference must be cleared by the update before the parent-delete lands |
| `Delete`      | `DependsOn(other.Old)`       | Child-delete must precede parent-delete       |

That last row (delete-to-delete) is critical for cascading deletes. Without
it, deleting both a parent and a child that references it would race, and
an FK-enforcing DB would reject the parent-delete.

The edge points `delete -> other`, meaning the *deletion* is placed after the
*other* op in the topological order. This is the "inversion" — the deletion
sinks to the bottom of any dependency chain that touches it.

### Topological sort (Kahn's algorithm, layered)

Source: `pkg/internal/dag/dag.go`. Algorithm:

1. Compute indegrees from the edge map.
2. Repeat:
   - Collect every node with indegree 0 → this is the next layer.
   - If no such node exists and unprocessed nodes remain, the graph has a
     cycle. Walk the indegree-positive subgraph to recover an actual cycle
     path for the error message.
   - Sort the layer lexicographically by node ID (so apply output is
     reproducible across runs).
   - Mark layer nodes as consumed (indegree = -1) and decrement indegrees of
     their downstream neighbors.
3. Return the layers in order.

The result is the *minimum* number of layers needed to satisfy the
dependencies. Every op that *can* run in parallel does run in parallel.

Edges to nodes outside the plan are dropped during indegree computation, so
external dependencies cost nothing.

### Cycle reporting

When `BuildLayers` cannot make progress, `formatCycleError` walks from any
indegree-positive node, following in-plan deps that still have positive
indegree, until it revisits a node. The slice from first visit to the repeat
is the reported cycle:

```
cycle detected: add:a -> add:b -> add:c -> add:a
```

`Generate` propagates this error up; no plan file is written.

---

## What apply does with the layers

Source: `pkg/core/apply/execute_operations.go`.

### Stale-plan guard: `verifyLayersMultiset`

Before the first layer dispatches, apply rebuilds the expected multiset of ops
from `Additions`/`Updates`/`Deletions` and compares it to what
`plan.Layers` actually references. Mismatches fail fast with a descriptive
error:

```
plan.Layers multiset mismatch: op {Kind:add Key:user-42} expected 1 times, found 0 (plan may be stale or hand-edited)
plan.Layers references unknown op {Kind:delete Key:ghost} 1 time(s)
```

This catches the most common operator footguns: editing a plan file by hand,
or shipping a plan generated against a stale schema.

### Layer-by-layer dispatch

For each layer:

1. Resolve each `LayerOp{Kind, Key}` back to its full
   `RecordAddition`/`RecordUpdate`/`RecordDeletion` via per-kind maps.
2. Build a slice of tasks and pass it to `concurrency.ExecuteTasks` with the
   configured `Parallelization`. Up to N ops in the layer run concurrently.
3. `ExecuteTasks` blocks until every task in the slice has either succeeded
   or exhausted its retries.
4. Read the failure counters (safe to read post-wait because the worker pool
   has fully drained — there is also a mutex protecting append in the
   `OnSuccess`/`OnFailure` task callbacks, since they run on worker
   goroutines).
5. If any new failures landed in this layer, **stop**. Record the layer index
   and break.

### Cascading skips

When the loop breaks at layer `stopAfter`, every op in layers
`stopAfter+1 ...` is appended to `ExecutionReport.Skipped` (split into
Additions / Updates / Deletions per its kind). `OnAdd`/`OnUpdate`/`OnDelete`
are **never invoked** for skipped ops.

The execution report prints a dedicated `# N operation(s) were skipped` block,
and `apply.Run` returns an error of the form
`plan execution incomplete: K failed (...), M skipped (...)` so the operator
sees both numbers.

### Layer barrier guarantee

Because `concurrency.ExecuteTasks` does `wg.Wait` before returning, no op
from layer N+1 can possibly start while any op from layer N is still in
flight, even with high parallelism. There is a test
(`TestExecuteOperations_LayeredPath_LayerBarrier`) that verifies this under
`parallelism = 4`.

---

## Finalize policy (`FinalizeOn`)

Layered apply changes when finalization should run, so `RunParams.FinalizeOn`
lets you pick a policy. Source: `pkg/types/finalize_policy.go`.

| Value                  | When `OnFinalize` runs                                 | Use when                                  |
| ---------------------- | ------------------------------------------------------ | ----------------------------------------- |
| `FinalizeAlways` (default, zero value) | Always, even on partial failure or full skip   | Backward-compatible behavior              |
| `FinalizeOnSuccess`    | Only when no op failed **and** no op was skipped       | Finalize must see a fully-applied plan    |
| `FinalizeOnAnySuccess` | When at least one op succeeded; skipped only on zero progress | Finalize hook is useful on partial success but pointless if nothing changed (e.g. matview refresh, cache invalidation) |

`FinalizeOnAnySuccess` is the recommended default for new consumers.

---

## Worked example: self-referencing FK

Suppose `Position` rows have a `ReportingTo` column that points to the row's
manager (another `Position` row, by key). FK enforced.

**Plan diff:**

- Add `manager` (no ReportingTo).
- Add `ic` (ReportingTo = `manager`).
- Update `staff` (changes ReportingTo from `old_manager` → `manager`).
- Delete `old_manager`.

**`DependsOn`:**

```go
DependsOn: func(p Position) []string {
    if p.ReportingTo == "" { return nil }
    return []string{p.ReportingTo}
},
```

**DAG construction:**

- `add:ic -> add:manager` (ic.New.ReportingTo = manager)
- `update:staff -> add:manager` (staff.New.ReportingTo = manager)
- `delete:old_manager -> update:staff` (staff.Old.ReportingTo = old_manager →
  deletion inverted)

**Layers (Kahn's, lex-sorted within each):**

- Layer 0: `add:manager`
- Layer 1: `add:ic`, `update:staff`
- Layer 2: `delete:old_manager`

**Apply:** manager is inserted first. ic and staff run concurrently in layer 1
— both can safely point at `manager`, which now exists. Finally old_manager is
deleted; by then staff no longer references it.

If `add:manager` fails, `add:ic`, `update:staff`, `delete:old_manager` all
land in `Skipped` — none of those callbacks ever fire.

---

## Quick Start

1. Add `DependsOn` to your `GenerateParams`. Return the dep keys of each
   record. That is the whole opt-in.
2. Pick a `FinalizeOn` policy (`FinalizeOnAnySuccess` is the sensible default
   for new code).
3. Run `plan.Generate` and `apply.Run` exactly as before. Layered execution
   kicks in automatically when `plan.Layers` is non-nil.

```go
// plan
plan, err := plan.Generate(plan.GenerateParams[Position]{
    // ... existing fields ...
    DependsOn: func(p Position) []string { return p.refs() },
})

// apply
err := apply.Run(apply.RunParams[Position]{
    // ... existing fields ...
    FinalizeOn: types.FinalizeOnAnySuccess,
})
```

That is it. Cycles surface as errors before the plan file lands; stale plans
are rejected before any DB write; failed layers cascade into `Skipped`
instead of corrupting downstream state.
