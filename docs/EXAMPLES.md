# Patterns and Use Cases

For the runnable example walkthrough, see [`examples/README.md`](../examples/README.md). This doc collects patterns you'll reach for once you're past the basic case.

## Use case 1: directory sync (LDAP/AD → DB)

Pull users from a directory, export to CSV, reconcile into the database.

```go
type ADUser struct {
    Username    string `csv:"username"`  // primary key
    Email       string `csv:"email"`
    Department  string `csv:"department"`
    IsActive    bool   `csv:"is_active"`
}

params := plan.GenerateParams[ADUser]{
    CSVPath:           "ldap_export.csv",
    LoadRemoteRecords: db.FetchAllUsers,
    ValidateRecord: func(u ADUser) error {
        if u.Email == "" {
            return errors.New("email required")
        }
        if !strings.HasSuffix(u.Email, "@company.com") {
            return errors.New("must be company email")
        }
        return nil
    },
    OnAdd:    func(a types.RecordAddition[ADUser]) error { return db.CreateUser(a.New) },
    OnUpdate: func(u types.RecordUpdate[ADUser]) error   { return db.UpdateUser(u.New) },
    OnDelete: func(d types.RecordDeletion[ADUser]) error {
        return db.DeactivateUser(d.Key)  // soft-delete
    },
}
```

## Use case 2: self-referencing FK with dependency layering

`positions.reporting_to` points at another `positions.id` row. The DB enforces the FK, so naive parallel inserts/deletes race. Provide `DependsOn` and Planear topologically sorts the plan.

```go
type Position struct {
    ID          string `csv:"id"`
    Title       string `csv:"title"`
    ReportingTo string `csv:"reporting_to"`  // FK to another Position.ID
}

genParams := plan.GenerateParams[Position]{
    CSVPath:           "./data",
    OutputFilePath:    "plan.json",
    ExtractKeyFunc:    func(p Position) string { return p.ID },
    LoadRemoteRecords: positions.LoadAll,
    ValidateRecord:    validatePosition,
    FormatRecordFunc:  formatPosition,
    FormatKeyFunc:     func(k string) string { return k },

    // Return the keys this record references. Keys not present in the plan
    // are treated as external and impose no in-plan ordering.
    DependsOn: func(p Position) []string {
        if p.ReportingTo == "" {
            return nil
        }
        return []string{p.ReportingTo}
    },
}

runParams := apply.RunParams[Position]{
    PlanFilePath: "plan.json",
    FormatRecord: formatPosition,
    FormatKey:    func(k string) string { return k },
    OnAdd:        func(a types.RecordAddition[Position]) error { return db.Insert(a.New) },
    OnUpdate:     func(u types.RecordUpdate[Position]) error { return db.Update(u.New) },
    OnDelete:     func(d types.RecordDeletion[Position]) error { return db.Delete(d.Key) },
    FinalizeOn:   types.FinalizeOnAnySuccess,
    OnFinalize:   func() error { return matviews.Refresh("org_chart") },
}
```

Cycles surface as `cycle detected: A -> B -> C -> A` *before* the plan file is written. If a layer fails at apply time, remaining layers' ops land in `ExecutionReport.Skipped` and their callbacks never fire. See [LAYERED_EXECUTION.md](./LAYERED_EXECUTION.md) for the algorithm.

## Patterns

### Composite keys

```go
func key(r RoleAssignment) string {
    return fmt.Sprintf("%s:%s", r.UserID, r.RoleName)
}
params.ExtractKeyFunc = key
```

The remote loader must produce a `map[string]T` keyed the same way.

### Transaction-style finalize

```go
OnAdd:      func(a types.RecordAddition[T]) error { return tx.Insert(a.New) },
OnUpdate:   func(u types.RecordUpdate[T]) error   { return tx.Update(u.New) },
OnDelete:   func(d types.RecordDeletion[T]) error { return tx.Delete(d.Key) },
OnFinalize: func() error { return tx.Commit() },
```

For layered apply, pair this with `FinalizeOn: FinalizeOnSuccess` so a partial run rolls back instead of committing.

### Field-level auditing

`OnUpdate` receives the per-field changes alongside the new record:

```go
OnUpdate: func(u types.RecordUpdate[UserRecord]) error {
    for _, c := range u.Changes {
        audit.Record(u.Key, c.Field, c.OldValue, c.NewValue)
    }
    return db.UpdateUser(u.New)
},
```

You can also use `Changes` to short-circuit when no field you care about moved.

### Picking a `FinalizeOn`

```go
runParams.FinalizeOn = types.FinalizeAlways       // zero value; runs on any outcome
runParams.FinalizeOn = types.FinalizeOnSuccess    // run only if no failures and no skips
runParams.FinalizeOn = types.FinalizeOnAnySuccess // run if at least one op succeeded
```

Recommended starting point for new code: `FinalizeOnAnySuccess` — pointless to refresh caches if nothing changed, but useful even on partial success.

### Inspecting skipped ops

`apply.Run` prints a `# N operation(s) were skipped` block and returns an error summarising counts. To act on them programmatically, call `ExecuteOperations` directly:

```go
report, _ := apply.ExecuteOperations(apply.ExecuteOperationsParams[T]{ /* ... */ })
for _, a := range report.Skipped.Additions {
    log.Printf("skipped add: %s (earlier layer failed)", a.Key)
}
```

### Parallelization

```go
import "runtime"

n := runtime.NumCPU()             // CPU-bound callbacks
params.Parallelization = &n

n := 50                           // network-bound (raise above core count)
params.Parallelization = &n
```

## CSV mapping notes

- Empty cells become `nil` for pointer types (`*int`, `*string`).
- Numeric fields are parsed by `strconv`; bad values surface as plan-generation errors.
- Nested structs and slices are supported; slices are comma-separated by default.

## Execution report shape

```go
type ExecutionReport[T any] struct {
    Success              Plan[T]            // ops that succeeded
    Failure              Plan[T]            // ops that exhausted retries
    Skipped              Plan[T]            // layered apply only: ops after a failed layer
    Ignores              []RecordIgnored[T] // validation-rejected records (from plan)
    FinalizationSuccess  bool
    FinalizationErrorMsg string
}
```

Validation rejections live in `plan.Ignores` and pass through to `ExecutionReport.Ignores`; their callbacks never fire.
