# Mixed Results Example

This example demonstrates how Planear handles mixed success/failure scenarios where some tasks succeed (with or without retries) and some tasks fail after all 3 retry attempts.

## Failure Patterns

This example simulates different failure patterns to show the library's retry behavior:

| Task | Pattern | Behavior |
|------|---------|----------|
| user1 (Alice) add | Success immediately | No retries needed |
| user2 (Bob) add | Fail once, then succeed | 1 RETRY log for attempt 1, succeeds on attempt 2 |
| user3 (Charlie) add | Fail twice, then succeed | 2 RETRY logs for attempts 1-2, succeeds on attempt 3 |
| user4 (Diana) add | Always fails | 3 RETRY logs for all attempts, marked as failed in report |
| user5 (Eve) add | Success immediately | No retries needed |
| user6 (Frank) update | Fail once, then succeed | 1 RETRY log, succeeds on attempt 2 |
| user7 (Grace) update | Always fails | 3 RETRY logs, marked as failed in report |

## Running the Example

```bash
cd examples/cmd/mixed_results
go run main.go
```

## Expected Output

The output shows three distinct sections:

### 1. Retry Logs (RETRY in Red)
All attempts that failed are logged with attempt numbers and error messages:
```
RETRY: Failed to add user2@example.com (Bob, 50 points) (attempt 1) - network timeout
RETRY: Failed to add user3@example.com (Charlie, 75 points) (attempt 1) - database connection lost
RETRY: Failed to add user3@example.com (Charlie, 75 points) (attempt 2) - database connection lost
RETRY: Failed to add user4@example.com (Diana, 60 points) (attempt 1) - permission denied
RETRY: Failed to add user4@example.com (Diana, 60 points) (attempt 2) - permission denied
RETRY: Failed to add user4@example.com (Diana, 60 points) (attempt 3) - permission denied
```

### 2. Execution Report
Shows the final results of all operations:
```
Summary of the executed result:

# 5 operation(s) succeeded

# 4 row(s) will be added
    + user1@example.com (Alice, 100 points)
    + user2@example.com (Bob, 50 points)
    + user3@example.com (Charlie, 75 points)
    + user5@example.com (Eve, 90 points)

# 1 row(s) will be updated
    ~ user6@example.com (Frank, 45 points)

Summary: 4 added, 1 updated, 0 deleted

# 2 operation(s) failed

# 1 row(s) failed to add
    + user4@example.com (Diana, 60 points)

# 1 row(s) failed to update
    ~ user7@example.com (Grace, 35 points)

Summary: 1 added, 1 updated, 0 deleted

Finalization:
  ✓ Finalization succeeded
```

### 3. Error Message (if failures occurred)
If there were operation failures, a beautified error message appears in red:
```
Some operations failed: 1 added, 1 updated, 0 deleted
```

## Key Points

1. **Immediate successes are silent** - user1 and user5 don't generate any RETRY logs
2. **Partial failures are retried** - user2, user3, and user6 retry and eventually succeed
3. **Complete failures are logged** - user4 and user7 fail all 3 attempts and are reported in the failure section
4. **Exponential backoff happens transparently** - the library handles 100ms → 200ms → 400ms delays between retries
5. **Operations are formatted by the formatter** - the formatted record (email, name, points) is shown in each log
6. **Report shows final state** - successful and failed operations are clearly separated
7. **Exit code reflects status** - exits with code 1 if any operations failed (as shown by the `Some operations failed...` message)

## Customizing Failure Behavior

To modify which tasks fail or how many times they fail, edit the callback functions in `main.go`:
- `addWithMixedResults()` - controls add operation failures
- `updateWithMixedResults()` - controls update operation failures

Add new email addresses and customize the failure patterns to simulate different scenarios.
