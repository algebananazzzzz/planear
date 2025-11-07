# Releasing Planear

This document explains how to create new releases for Planear.

## Automatic Release Process

Planear uses GitHub Actions to automatically create tags and releases when the VERSION file is updated on `main`.

### How It Works

1. **Developer updates VERSION file** with new semantic version
2. **Changes merged to main** via pull request
3. **GitHub Actions automatically**:
   - Detects new VERSION on main branch
   - Parses commits since last tag using conventional commits
   - Generates changelog from commit messages
   - Creates git tag (e.g., `v1.0.1`)
   - Creates GitHub release with auto-generated changelog
   - Tags are pushed to repository

No CHANGELOG.md file updates needed—the release notes are generated from your commit messages!

### Release Steps for Developers

#### 1. Create a Release Branch

```bash
git checkout -b release/v1.1.0
```

#### 2. Update VERSION File

Edit `VERSION` file and bump version using semantic versioning:

```
1.1.0
```

**Semantic Versioning Rules:**
- **MAJOR** (X.0.0): Breaking changes
- **MINOR** (0.X.0): New features (backward compatible)
- **PATCH** (0.0.X): Bug fixes

#### 3. Ensure Conventional Commits

Make sure your commit messages follow the conventional commit format so the changelog auto-generates correctly:

```
feat(core): add new streaming API          # Appears in Features
fix(retry): fix exponential backoff bug    # Appears in Bug Fixes
docs: update README                        # Appears in Documentation
perf(diff): optimize comparison algorithm  # Appears in Performance
refactor: simplify error handling          # Appears in Refactoring
```

**Commit Format:**
```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types:**
- `feat` - New features
- `fix` - Bug fixes
- `docs` - Documentation
- `perf` - Performance improvements
- `refactor` - Code refactoring
- `test` - Test changes
- `chore` - Maintenance

**Breaking Changes:**
Include `BREAKING CHANGE:` in commit footer to mark breaking changes:
```
feat(api): change callback signature

BREAKING CHANGE: OnFinalize now accepts context parameter
```

#### 4. Run Pre-Release Checks

Ensure everything passes:

```bash
# Run tests
go test ./...

# Check code quality
go vet ./...

# Format code
go fmt ./...

# Tidy dependencies
go mod tidy
```

#### 5. Commit Changes

```bash
git add VERSION
git commit -m "chore: release v1.1.0"
```

#### 6. Create Pull Request

Push branch and create PR:

```bash
git push origin release/v1.1.0
```

Then on GitHub:
1. Open PR against `main`
2. Add title: `chore: release v1.1.0`
3. Reference any related issues
4. Request review (optional)

#### 7. Merge to Main

Once approved, merge the PR:
- GitHub Actions detects VERSION change on main
- Automatically parses commits since last tag
- Generates changelog from conventional commits
- Creates tag `v1.1.0`
- Creates GitHub release with auto-generated changelog
- Release is now live! 🎉

## Workflow Triggers

The release workflow triggers on:
- **Push to main branch**

And automatically:
1. Reads the VERSION file
2. Checks if that version tag already exists
3. If not, generates changelog from commits since last tag
4. Creates release

This means every push to main is checked, but only new VERSION changes trigger releases.

## Checking Release Status

1. **View releases**: https://github.com/algebananazzzzz/planear/releases
2. **View tags**: https://github.com/algebananazzzzz/planear/tags
3. **View workflow runs**: https://github.com/algebananazzzzz/planear/actions
4. **Check pkg.go.dev** (24-48 hours after release): https://pkg.go.dev/github.com/algebananazzzzz/planear

## Troubleshooting

### Workflow didn't trigger
- Ensure VERSION file was actually modified (not just go.mod)
- Check branch is `main`
- View workflow runs at: https://github.com/algebananazzzzz/planear/actions

### Tag already exists
- Workflow checks if tag exists before creating
- If tag already exists, no release is created
- To fix: merge VERSION with new number

### Release shows old changelog
- Ensure CHANGELOG.md is committed before merge
- Workflow extracts text from CHANGELOG.md
- Format must follow: `## [X.Y.Z] - YYYY-MM-DD`

## Manual Release (If Needed)

If you need to manually create a release:

```bash
# Create and push tag
git tag -a vX.Y.Z -m "Release version X.Y.Z"
git push origin vX.Y.Z

# Create release via gh CLI
gh release create vX.Y.Z --notes "See CHANGELOG.md"
```

## Examples

### Example 1: Bug Fix Release (v1.0.1)

**VERSION file:**
```
1.0.1
```

**Commits since v1.0.0:**
```
fix(retry): fix off-by-one error in exponential backoff
fix(logging): show correct attempt numbers in retry logs
```

Changelog automatically generated:
```
## [1.0.1] - 2024-11-15

### 🐛 Bug Fixes
- fix(retry): fix off-by-one error in exponential backoff
- fix(logging): show correct attempt numbers in retry logs
```

### Example 2: Feature Release (v1.1.0)

**VERSION file:**
```
1.1.0
```

**Commits since v1.0.0:**
```
feat(apply): add new --parallel flag for apply command
feat(validation): support custom validators in CSV loading
fix(pool): memory leak in worker pool cleanup
```

Changelog automatically generated:
```
## [1.1.0] - 2024-11-20

### ✨ Features
- feat(apply): add new --parallel flag for apply command
- feat(validation): support custom validators in CSV loading

### 🐛 Bug Fixes
- fix(pool): memory leak in worker pool cleanup
```

### Example 3: Major Release (v2.0.0)

**VERSION file:**
```
2.0.0
```

**Commits since v1.0.0:**
```
feat(api): new streaming API for large CSV files

BREAKING CHANGE: OnFinalize callback removed (use context cancellation)

feat(core): change FormatRecord signature to return error

BREAKING CHANGE: Changed FormatRecord signature to return error
```

Changelog automatically generated:
```
## [2.0.0] - 2024-12-01

### ⚠️ Breaking Changes
- OnFinalize callback removed (use context cancellation)
- Changed FormatRecord signature to return error

### ✨ Features
- feat(api): new streaming API for large CSV files
- feat(core): change FormatRecord signature to return error
```

## Release Checklist

- [ ] Code changes complete and tested
- [ ] Use conventional commits (feat:, fix:, docs:, etc.)
- [ ] Run `go test ./...` - all pass
- [ ] Run `go vet ./...` - clean
- [ ] Bump VERSION file with semantic version
- [ ] Commit with message `chore: release vX.Y.Z`
- [ ] Push to release branch
- [ ] Create and merge PR to main
- [ ] Verify GitHub Actions workflow runs at https://github.com/algebananazzzzz/planear/actions
- [ ] Check https://github.com/algebananazzzzz/planear/releases for new release with auto-generated changelog
- [ ] (Optional) Wait 24-48 hours and verify pkg.go.dev updated at https://pkg.go.dev/github.com/algebananazzzzz/planear@vX.Y.Z

## Questions?

See the main [PUBLISHING.md](./PUBLISHING.md) for more details on versioning strategy and maintenance.
