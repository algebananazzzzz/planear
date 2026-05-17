# Testing

## Running tests

```bash
go test ./...                       # all packages
go test -v ./pkg/core/plan/...      # one package, verbose
go test -run TestGeneratePlan ./... # one test by name
go test -cover ./...                # with coverage summary
```

For an HTML coverage report:

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

## `testutils/`

Helpers for tests that need a scratch directory or fixture files:

```go
import "github.com/algebananazzzzz/planear/testutils"

dir  := testutils.NewTestDir(t)                          // tempdir, cleaned up by t.Cleanup
path := testutils.CreateMockFile(t, dir, "f.txt", body)
csv  := testutils.WriteCSVFile(t, dir, "data.csv", records)
json := testutils.WriteJSONFile(t, dir, "plan.json", plan)
ok   := testutils.FileExists(t, path)
```

Tests use [`testify`](https://github.com/stretchr/testify) for assertions (`assert.*` continues on failure, `require.*` halts).

## CI

`.github/workflows/test.yml` runs `go test -v -race -coverprofile=coverage.out ./...` on pushes to `feat/**`, `feature/**`, and `JIRA/**` branches that touch `pkg/**`, and posts a coverage summary to the workflow run.
