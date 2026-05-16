# Dependency-Aware Plan Ordering (v2)

I want to add **native dependency resolution** to planear so consumers with intra-entity references — self-referencing FKs, cross-table references, BEFORE-trigger validations — can declare relationships once and have planear handle the execution order. This replaces an earlier proposal (low-level `OrderPlan` callback) with a higher-level, declarative API. Walk me through whether this fits the library's design before implementing.

## Motivating use case

A consumer table `cca_positions` has a self-referencing FK:

```sql
CREATE TABLE cca_positions (
  id SERIAL PRIMARY KEY,
  cca_id INTEGER NOT NULL,
  name TEXT NOT NULL,
  reporting_position INTEGER REFERENCES cca_positions(id),
  ...
);
```

CSV source rows reference each other by name:

```csv
cca_name,position_name,reporter_cca,reporter_position
JCRC,President,,
JCRC,Welfare Head,JCRC,President
JCRC,Welfare Member,JCRC,Welfare Head
```

Today, planear dispatches all additions to a worker pool with no order. Workers race; a child position attempts to insert before its parent → FK violation → retried 3× → marked as failure. The only ops that succeed are roots.

The consumer's current workaround is a two-pass DB procedure (insert with `reporting_position = NULL`, then patch FK by name lookup in pass 2). This works but bloats the schema with a procedure that exists solely because the apply layer is dependency-blind.

## What I want planear to do natively

Let the consumer declare what each record depends on, then have planear:
1. Build a DAG over the entire plan (additions + updates + deletions) at **plan generation time**.
2. Detect cycles at plan time (fail fast, before any DB writes).
3. Topologically sort into execution layers and serialize the layer assignments into the plan JSON.
4. At apply time, walk the layers in order; within each layer, dispatch to the existing worker pool in parallel; wait for the layer to drain before starting the next.

The consumer writes one callback (`DependsOn func(T) []string`). planear owns everything else.

## Design

### API additions

**`GenerateParams[T]` (`pkg/core/plan/generate_plan.go`)** — add one optional field:

```go
type GenerateParams[T any] struct {
    // ... existing fields ...

    // DependsOn returns the keys (as produced by ExtractKeyFunc) that this
    // record references. When set, Generate builds a dependency DAG over the
    // entire plan, topologically sorts it into layers, and stores the layer
    // assignment in Plan.Layers. Cycles produce an error before the plan is
    // written. Keys returned that are not present in the plan are treated as
    // "already satisfied" (they exist in remote and impose no ordering
    // constraint within this plan).
    //
    // For deletions, planear automatically inverts: a row being deleted is
    // scheduled after every row in the plan that depends on it (whether by
    // its old state for deletes/updates, or its new state for adds/updates).
    DependsOn func(T) []string
}
```

**`Plan[T]` (`pkg/types/plan.go`)** — add one optional field:

```go
type Plan[T any] struct {
    Additions []RecordAddition[T] `json:"additions"`
    Updates   []RecordUpdate[T]   `json:"updates"`
    Deletions []RecordDeletion[T] `json:"deletions"`
    Ignores   []RecordIgnored[T]  `json:"ignores"`
    // Layers, if non-nil, dictates apply-time execution order. Each inner
    // slice is a layer; ops in the same layer dispatch in parallel; layer
    // N+1 starts after layer N drains. References ops in Additions /
    // Updates / Deletions by (Kind, Key).
    Layers [][]LayerOp `json:"layers,omitempty"`
}

type LayerOp struct {
    Kind string `json:"kind"` // "add" | "update" | "delete"
    Key  string `json:"key"`
}
```

**`RunParams[T]` (`pkg/core/apply/apply_plan.go`)** — no new fields. Apply automatically takes the layered path when `plan.Layers != nil`.

**`ExecutionReport[T]` (`pkg/types/plan.go`)** — add one field for failure cascading:

```go
type ExecutionReport[T any] struct {
    Success              Plan[T]            `json:"success"`
    Failure              Plan[T]            `json:"failure"`
    Skipped              Plan[T]            `json:"skipped"` // ops not attempted because an earlier layer failed
    Ignores              []RecordIgnored[T] `json:"ignores"`
    FinalizationSuccess  bool               `json:"finalization_success"`
    FinalizationErrorMsg string             `json:"finalization_error_msg,omitempty"`
}
```

### Algorithm (plan time)

When `DependsOn != nil`:

1. Walk every op in the plan. For each, build the dependency edge set:
   - For an addition or update: edges from this op's key to every key returned by `DependsOn(record.New)` that exists in the plan.
   - For a deletion: edges from every other op in the plan whose `DependsOn(...)` includes this deletion's key, **into** this deletion. (Inverts the direction so the deletion runs after its dependents.)
2. Topologically sort the DAG into layers using Kahn's algorithm. Layer 0 = nodes with no remaining incoming edges. Strip those nodes, repeat.
3. If any nodes remain after no-progress iteration, there is a cycle. Return an error formatted as `cycle detected: A -> B -> C -> A`. Do not write the plan file.
4. Serialize layers into `plan.Layers` as `[][]LayerOp`.

### Algorithm (apply time)

In `Run` / `ExecuteOperations`:

1. Load plan.
2. If `plan.Layers == nil`, take the existing flat dispatch path. (Backward compat: every plan generated by a planear version without `DependsOn` falls through here.)
3. Otherwise:
   - Verify multiset: every op in `Additions` ∪ `Updates` ∪ `Deletions` must appear exactly once across all `Layers`. Mismatch = hand-edited or stale plan; error out before any DB writes.
   - For each layer in order:
     - Resolve each `LayerOp` to its actual op via (Kind, Key) lookup into the plan buckets.
     - Build tasks (reuse existing `addTask` / `updateTask` / `deleteTask` closures from `execute_operations.go`).
     - Dispatch via `concurrency.ExecuteTasks` with `Parallelization` workers.
     - After the layer drains, check if any op landed in `failure`. If so, mark all subsequent layers' ops into `Skipped` and break.
4. Run `OnFinalize` per the policy below.

### Open question 1 — Finalize policy

When the run partially fails (some layers succeeded, later layers skipped), should `OnFinalize` run? Today it always runs. Three reasonable policies:

- `FinalizeAlways` (current behavior, default for backward compat)
- `FinalizeOnSuccess` (skip finalize if any op failed or was skipped)
- `FinalizeOnAnySuccess` (run finalize if at least one op succeeded; skip only if zero progress)

Add a `FinalizeOn` enum field to `RunParams[T]`. Default to `FinalizeAlways` to preserve existing behavior. Pick one default for new consumers — recommend `FinalizeOnAnySuccess` since most finalize hooks (matview refresh, cache invalidation) are useful even on partial success but pointless on zero progress. What's your read?

### Open question 2 — Formatter output

`formatters.FormatExecutionReport` today iterates Success / Failure / Ignores. Should it also print `Skipped`? I think yes — operators reading the report need to distinguish "didn't try" from "tried and failed." A new section between Failure and Ignores titled `SKIPPED (n)` listing each skipped op via `FormatRecord`. Confirm before implementing.

### Open question 3 — Cross-domain dependencies

A position can reference a CCA (the parent table). Today, each domain is a separate planear pipeline with its own plan/apply commands. `DependsOn` returns keys within the same plan only — it cannot express "this position depends on a row in a different plan."

Two interpretations:
- **In-scope**: planear only handles intra-plan deps; cross-domain ordering is the consumer's responsibility (sequence the apply commands: ccas first, then positions). I lean here — keeps planear's scope tight.
- **Out-of-scope for v1**: ship intra-plan, document the limitation.

Confirm intra-plan only.

### Open question 4 — Should `DependsOn` also live on `RunParams`?

Considered: let `Run` accept `DependsOn` so an old plan (without `Layers`) can be re-layered at apply time. Rejected because:
- It puts the same callback in two places (drift risk).
- The plan file is the source of truth for the work; layering is part of the work description.
- Old plans falling through to the flat path is correct behavior — they were generated under the contract "no ordering needed."

Confirm rejection.

## How this resolves earlier concerns

| Concern from v1 | How v2 handles it |
|---|---|
| Consumer must implement topo-sort | gone — planear owns it |
| `OrderPlan` callback could silently drop ops | gone — `DependsOn` is invoked once per record by planear; multiset is structural |
| Plan JSON not self-documenting | `Layers` makes execution order explicit and human-readable |
| Cycle errors only surface at apply time | now caught at plan time, before any DB writes |
| `Skipped` not printed in formatter | open question 2, will be addressed |
| `OnFinalize` always runs on partial state | open question 1, configurable via `FinalizeOn` |
| Stale or hand-edited plan | apply-time multiset verification rejects it |

## Backward compatibility

- Plans generated before this change have no `Layers` field → apply takes the existing flat path. No consumer change required.
- Consumers who don't set `DependsOn` see no behavior change at plan time. `Plan.Layers` stays nil. Apply takes flat path.
- Existing tests should pass unchanged. New tests cover the layered path.

## What to produce

Before writing any code:

1. **Confirm the design fits planear's philosophy.** Is dependency resolution reasonable to bake in, or is it a slippery slope toward more domain-specific features? My argument: it's the same kind of cross-cutting concern as retry/parallelism — generic enough that every consumer with FK relationships needs it.

2. **Pick answers for open questions 1–4.** One sentence each.

3. **Sketch the file layout.** Which existing files change, which new files appear, where the topo algorithm lives (suggest: `pkg/core/plan/depgraph.go` or `pkg/internal/dag/` if you want to hide it).

4. **Write a task-by-task implementation plan** (TDD-style: failing test → implementation → passing test → commit, like the v1 plan that lives in `docs/superpowers/plans/2026-05-16-ordering-hook.md`). Suggest 8–12 tasks.

5. **Flag any breaking changes I haven't noticed.** I believe this is fully additive but I'd like a second look at:
   - JSON marshaling of plans (does adding `Layers` field break consumers parsing plan JSON externally?)
   - Generic type inference across the new `LayerOp` references
   - Concurrency model interaction (does `ExecuteTasks` correctly drain before returning?)

Then I'll review the plan and greenlight implementation.
