# GitHub Workflows

This directory contains GitHub Actions workflows for automated testing and code quality checks.

## test.yml - Tests and Coverage

**Triggers on:**
- Push to any branch (if `pkg/`, `go.mod`, `go.sum`, or `.github/workflows/test.yml` changed)
- Pull requests to `main` or `master`

**Jobs:**

### 1. Test Job
- ✅ Runs all tests with race detection
- 📊 Generates coverage reports
- 📈 Shows overall coverage
- 📦 Shows library package coverage breakdown
- ⚠️ Checks library code against 95% coverage threshold
- 🔄 Uploads coverage to Codecov
- 💬 Comments on PRs with coverage summary

### 2. Lint Job
- 🎨 Checks code formatting (`go fmt`)
- 🔍 Runs `go vet` for static analysis
- 🏁 Tests for race conditions

## Configuration

### Coverage Threshold
Default threshold is **95%** for library code. To change, edit this line in `test.yml`:

```yaml
THRESHOLD=95
```

### Go Version
Default is **1.22**. To change, edit:

```yaml
go-version: '1.22'
```

### Coverage Service (Codecov)
To enable Codecov integration:

1. Visit [codecov.io](https://codecov.io)
2. Connect your GitHub account
3. Enable this repository
4. Badge will be available in your repo settings

Comment out or remove this step to disable Codecov:

```yaml
- name: Upload coverage to Codecov
  uses: codecov/codecov-action@v3
  ...
```

## Viewing Results

### On GitHub
- **Actions tab**: View workflow runs and logs
- **PR comments**: Coverage report appears automatically on PR
- **Badges**: Add to README.md:

```markdown
![Tests](https://github.com/YOUR_ORG/planear/workflows/Tests%20and%20Coverage/badge.svg)
```

### Local Testing
Before pushing, test locally:

```bash
# Clear cache and run tests
go clean -testcache && go test -v ./...

# Check coverage
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | grep "^total"
```

## Coverage Reports

The workflow generates two coverage reports:

1. **Overall Coverage** - All code including examples and testutils
2. **Library Code Coverage** - Only `pkg/concurrency`, `pkg/core`, `pkg/formatters`, `pkg/input`, `pkg/utils`

Library code should maintain **95%+** coverage as set by the threshold.

## Troubleshooting

### Workflow not triggering
- Check branch protection rules
- Verify paths filter matches your changes
- Check Actions are enabled in repo settings

### Coverage dropped below threshold
- Run locally: `go test -coverprofile=coverage.out ./...`
- Check coverage report: `go tool cover -html=coverage.out`
- See `TESTING.md` for guidance on improving coverage

### Race conditions detected
- Review the workflow logs for specific file/line numbers
- Use `go run -race` locally to debug
- Most common in concurrent code (`pkg/concurrency`)

## Customization

To modify the workflow:

1. Edit `.github/workflows/test.yml`
2. Push to any branch
3. The new workflow version runs immediately
4. Existing PRs may need to be re-synced to use updated workflow

## Related Files

- `TESTING.md` - Comprehensive testing guide
- `CONTRIBUTING.md` - Development guidelines
- `go.mod` / `go.sum` - Dependency management
