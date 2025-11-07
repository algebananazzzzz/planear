# Planear Examples

This directory contains examples demonstrating Planear's core features:
- CSV-driven declarative state management
- Reconciliation with automatic retry and exponential backoff
- Custom callbacks for complete control

## Example Structure

### `lib/` - Reusable library code

- **types.go** - Define your record structure (`UserRecord`)
- **actions.go** - Callbacks for operations with exponential backoff simulation
- **remote.go** - Mock database for loading remote state
- **validators.go** - Record validation rules
- **formatters.go** - Display formatting functions
- **constants.go** - Configuration constants

### `cmd/` - Executable examples

#### `plan/main.go` - Plan Generation
Generates a reconciliation plan by comparing local CSV with remote database state.

```bash
cd examples && go run cmd/plan/main.go
```

This produces `data/generated/plan.json` showing what operations are needed.

#### `apply/main.go` - Plan Execution
Executes the previously generated plan with automatic retry and exponential backoff.

```bash
cd examples && go run cmd/apply/main.go
```

### `data/` - Example data

- **users.csv** - Local desired state (CSV format)
- **generated/plan.json** - Generated plan (created by plan command)

## How Exponential Backoff Works

The example demonstrates exponential backoff through simulated failures:

### In actions.go:

```go
// AddUser fails 2 times, then succeeds
// Demonstrates how exponential backoff retries the operation
func AddUser(rec types.RecordAddition[UserRecord]) error {
    attempt := atomic.AddInt32(failureSimulation["add"], 1)
    if attempt <= 2 {
        log.Printf("RETRY: Failed to add %s (attempt %d) - temporary network error",
                   FormatRecord(rec.New), attempt)
        return errors.New("temporary network error")
    }
    // Silent on success - no logging
    return nil
}
```

### The retry mechanism (in pkg/concurrency):

When an operation fails, the executor automatically retries with exponential backoff:

1. **Attempt 1** fails → logs `RETRY:` message → waits 100ms (2^0 * 100ms)
2. **Attempt 2** fails → logs `RETRY:` message → waits 200ms (2^1 * 100ms)
3. **Attempt 3** succeeds → silent (no logging on success)

Total wait time: 100ms + 200ms = 300ms before success

### Max retries: 3 attempts per operation

If all 3 attempts fail, the operation is marked as failed and included in the execution report.

### Logging Strategy

- **Success operations**: Silent (no output)
- **Failed operations**: Logged with `RETRY:` prefix
- **Results**: Printed by the core apply package (summary statistics)

## Running the Examples

### Step 1: Generate a Plan

```bash
cd examples
go run cmd/plan/main.go
```

Output:
```
Actions are indicated with the following symbols:
    + add
    ~ update
    - delete
    ? ignore

Executing plan will perform the following actions:

# 3 row(s) will be added
    + alice@example.com=Alice (30 points)
    + bob@example.com=Bob (25 points)
    + charlie@example.com=Charlie (28 points)

Successfully written plan file to: data/generated/plan.json
```

### Step 2: Reset and Execute the Plan

```bash
cd examples
go run cmd/apply/main.go
```

Output:
```
RETRY: Failed to add alice@example.com=Alice (30 points) (attempt 1) - temporary network error
RETRY: Failed to add alice@example.com=Alice (30 points) (attempt 2) - temporary network error
RETRY: Failed to update bob@example.com=Bob (25 points) (attempt 1) - database temporarily unavailable
RETRY: Finalization failed (attempt 1) - commit failed

Summary of the executed result:

# 3 operation(s) succeeded

# 3 row(s) will be added
    + alice@example.com=Alice (30 points)
    + bob@example.com=Bob (25 points)
    + charlie@example.com=Charlie (28 points)

Summary: 3 added, 0 updated, 0 deleted

# 0 operation(s) failed

Finalization:
  ✓ Finalization succeeded
```

**Note**: The `RETRY:` messages show exponential backoff in action:
- Each operation failure is logged with attempt number
- After each failure, the system waits with exponential backoff (100ms, then 200ms, then 400ms)
- **Finalization is also retried** - if finalization fails, it's automatically retried up to 3 times with the same exponential backoff
- Successful operations are silent

## Understanding the Failure Simulation

The example code includes simulated failures to demonstrate the retry mechanism:

| Operation | Failures | Behavior |
|-----------|----------|----------|
| AddUser | 2x | Fails twice, succeeds on 3rd attempt (with 100ms + 200ms backoff) |
| UpdateUser | 1x | Fails once, succeeds on 2nd attempt (with 100ms backoff) |
| DeleteUser | 0x | Succeeds immediately |
| Finalize | 1x | **Retried 3 times with backoff** - Fails once, succeeds on 2nd retry (with 100ms backoff) |

## Key Concepts Demonstrated

### 1. Declarative State
CSV file defines desired state → Plan shows what needs to change

### 2. Complete Control via Callbacks
```go
OnAdd:      lib.AddUser,      // Custom logic for additions
OnUpdate:   lib.UpdateUser,   // Custom logic for updates
OnDelete:   lib.DeleteUser,   // Custom logic for deletions
OnFinalize: lib.Finalize,     // Custom logic after all operations
```

### 3. Automatic Retry with Exponential Backoff
- Handles transient failures (network, database timeouts)
- Configurable: 3 max retries, 100ms base delay
- Backoff doubles on each retry: 100ms → 200ms → 400ms

### 4. Parallel Execution
Operations run in parallel (default 2 workers) with synchronized failure tracking.

## Customizing the Example

To use with your own data:

1. **Modify types.go** - Define your record structure
2. **Update users.csv** - Put your actual data
3. **Update actions.go** - Implement real operations (API calls, DB writes)
4. **Update remote.go** - Connect to your actual system
5. **Update validators.go** - Add validation rules

Then run the same commands:

```bash
go run cmd/plan/main.go
go run cmd/apply/main.go
```

## Testing Failure Handling

To test how your callbacks handle failures:

1. Modify `failureSimulation` in actions.go to trigger at specific conditions
2. Run `go run cmd/apply/main.go` to see failures in execution report
3. Check output for which operations failed and why

## See Also

- [TESTING.md](../TESTING.md) - Testing guide
- [EXAMPLES.md](../EXAMPLES.md) - More use cases and patterns
- [pkg/concurrency/pool.go](../pkg/concurrency/pool.go) - Retry and exponential backoff implementation
- [pkg/core/apply/execute_operations.go](../pkg/core/apply/execute_operations.go) - Parallel execution with callbacks
