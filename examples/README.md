# Planear example: user management

A runnable example that syncs user records between `users.csv` and a mock database.

## Layout

```
examples/
├── cmd/
│   ├── plan/main.go    # generate plan.json
│   └── apply/main.go   # execute plan.json
├── lib/                # UserRecord, validators, mock remote, action callbacks
└── data/
    ├── users.csv             # source of truth
    └── generated/plan.json   # output of `cmd/plan`
```

## Run it

```bash
cd examples
go run cmd/plan/main.go     # writes data/generated/plan.json
go run cmd/apply/main.go    # executes the plan against the mock DB
```

`cmd/plan` prints the diff and writes the plan file; `cmd/apply` reads it back and dispatches to the callbacks in `lib/actions.go`.

## What the example demonstrates

The callbacks in `lib/actions.go` are wired to fail a controlled number of times, so the apply output shows the retry-with-exponential-backoff behavior:

| Operation | Simulated failures | Outcome |
|-----------|--------------------|---------|
| `AddUser` | 2 | succeeds on attempt 3 (waits 100ms, 200ms) |
| `UpdateUser` | 1 | succeeds on attempt 2 |
| `DeleteUser` | 0 | succeeds immediately |
| `Finalize` | 1 | succeeds on attempt 2 |

Failing attempts log a `RETRY:` line; successes are silent. Defaults are 3 retries, 100ms base delay, doubling each attempt — see `pkg/concurrency/pool.go`.

## Adapting to your own data

1. Replace `UserRecord` in `lib/types.go` with your struct.
2. Point `lib/remote.go`'s loader at your real source.
3. Implement real operations in `lib/actions.go`.
4. Put your data in `data/users.csv` (or change the CSV path).

For patterns beyond this example (composite keys, dependency ordering, field-level auditing, finalize policies), see [../docs/EXAMPLES.md](../docs/EXAMPLES.md).
