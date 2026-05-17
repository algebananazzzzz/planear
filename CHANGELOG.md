# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

> **Maintenance:** this file is hand-edited. The release workflow
> (`.github/workflows/release.yml`) only creates the git tag and GitHub
> release — it does not regenerate this changelog. Move entries from
> `[Unreleased]` into a new versioned section as part of the PR that ships
> them. See [docs/RELEASING.md](./docs/RELEASING.md) for how versions are
> bumped.

## [Unreleased]

### Added
- **Dependency-aware layered execution.** Opt in by setting
  `GenerateParams.DependsOn`; `plan.Generate` builds a dependency DAG over the
  plan and serializes a topological layering into the new `Plan.Layers` field.
  At apply time, layers execute sequentially with full intra-layer parallelism
  and a hard barrier between layers. Deletions automatically invert (a
  delete is scheduled after every op that references the deleted key,
  including other deletes — cascading deletes are safe). Cycles surface as
  `cycle detected: A -> B -> C -> A` errors *before* the plan file is written.
  See `docs/LAYERED_EXECUTION.md`.
- `Plan.Layers [][]LayerOp` field and `LayerOp{Kind LayerOpKind, Key string}`
  type (`LayerOpAdd` / `LayerOpUpdate` / `LayerOpDelete`). JSON wire format
  preserves `"add"`/`"update"`/`"delete"`.
- `ExecutionReport.Skipped Plan[T]` — ops in layers after a failed layer,
  recorded instead of executed. The execution report prints a dedicated
  *N operation(s) were skipped* block; `apply.Run`'s error string reports
  both failed and skipped counts.
- `RunParams.FinalizeOn` (and `ExecuteOperationsParams.FinalizeOn`) of type
  `types.FinalizeOn`. Values: `FinalizeAlways` (default, backward-compatible),
  `FinalizeOnSuccess` (no failures *and* no skips), `FinalizeOnAnySuccess`
  (at least one op succeeded — recommended for new consumers).
- Internal `pkg/internal/dag` package implementing layered topological sort
  with cycle detection (Kahn's algorithm, lex-sorted within each layer for
  reproducible output).
- Stale-plan guard: layered apply rejects a plan whose `Layers` multiset
  does not match `Additions`/`Updates`/`Deletions` exactly, with a
  descriptive error, before any DB write occurs.

### Fixed
- `plan.Generate` now removes a stale plan file at `OutputFilePath` when the
  computed plan is empty, so callers cannot accidentally re-apply yesterday's
  plan.
- `plan.Generate` and `apply.Run` validate required function parameters and
  return a descriptive error instead of nil-panicking.

## [1.0.0] - 2024-11-08

### Added
- Initial release of Planear
- Two core entry points: `plan.Generate()` and `apply.Run()`
- Type-safe generics for handling any record type
- CSV-driven declarative state management
- Custom callbacks for complete control: `OnAdd`, `OnUpdate`, `OnDelete`, `OnFinalize`
- Automatic exponential backoff retry logic (3 retries, 100ms base delay)
- Configurable parallel execution with worker pool
- Field-level change tracking for updates
- Comprehensive execution reporting
- Record validation support
- Streaming CSV parser for large files

### Documentation
- Complete README with quick start
- Real-world examples and use cases
- Comparison with Terraform, Pulumi, Liquibase
- Contributing guidelines
- Publishing guides

### Test Coverage
- 98.4% code coverage
- Comprehensive test suite covering:
  - Plan generation from CSV
  - Retry logic with exponential backoff
  - Parallel execution and task management
  - Error handling and recovery
  - Formatting and reporting

### Examples
- Basic example with successful operations
- Mixed results example demonstrating retry behavior with partial failures

[1.0.0]: https://github.com/algebananazzzzz/planear/releases/tag/v1.0.0
