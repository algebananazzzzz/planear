# Releasing Planear

This document explains how to create new releases for Planear.

## Automatic Release Process

Planear uses GitHub Actions to automatically create tags and releases when changes are merged to `main`.

### How It Works

1. **Developer updates VERSION file** with new semantic version
2. **Developer updates CHANGELOG.md** with release notes
3. **Changes merged to main** via pull request
4. **GitHub Actions automatically**:
   - Detects VERSION file change
   - Creates git tag (e.g., `v1.0.1`)
   - Creates GitHub release with changelog
   - Tags are pushed to repository

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

#### 3. Update CHANGELOG.md

Add new section at the top with release notes:

```markdown
## [1.1.0] - 2024-11-15

### Added
- New feature description
- Another feature

### Fixed
- Bug fix description

### Changed
- Breaking change description (if any)

## [1.0.0] - 2024-11-08
...
```

**Section Types:**
- `Added` - New features
- `Fixed` - Bug fixes
- `Changed` - Changes in existing functionality
- `Deprecated` - Soon-to-be removed features
- `Removed` - Removed features
- `Security` - Security-related changes

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
git add VERSION CHANGELOG.md
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
- GitHub Actions detects VERSION change
- Automatically creates tag `v1.1.0`
- Automatically creates GitHub release
- Release is now live! 🎉

## Workflow Triggers

The release workflow triggers on:
- **Push to main branch**
- **AND changes to VERSION or CHANGELOG.md**

So if you only commit code changes without updating VERSION/CHANGELOG, no release is created (which is correct for pre-release work).

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

**CHANGELOG.md:**
```markdown
## [1.0.1] - 2024-11-15

### Fixed
- Retry logging now shows correct attempt numbers
- Fixed off-by-one error in exponential backoff calculation

## [1.0.0] - 2024-11-08
...
```

### Example 2: Feature Release (v1.1.0)

**VERSION file:**
```
1.1.0
```

**CHANGELOG.md:**
```markdown
## [1.1.0] - 2024-11-20

### Added
- New `--parallel` flag for apply command
- Support for custom validators in CSV loading

### Fixed
- Memory leak in worker pool cleanup

## [1.0.0] - 2024-11-08
...
```

### Example 3: Major Release (v2.0.0)

**VERSION file:**
```
2.0.0
```

**CHANGELOG.md:**
```markdown
## [2.0.0] - 2024-12-01

### Added
- New streaming API for large CSV files

### Changed
- **BREAKING**: Removed deprecated OnFinalize callback (use context cancellation)
- **BREAKING**: Changed FormatRecord signature to return error

### Removed
- Legacy retry configuration options

## [1.0.0] - 2024-11-08
...
```

## Release Checklist

- [ ] Code changes complete and tested
- [ ] Bump VERSION file with semantic version
- [ ] Update CHANGELOG.md with release notes
- [ ] Run `go test ./...` - all pass
- [ ] Run `go vet ./...` - clean
- [ ] Commit with message `chore: release vX.Y.Z`
- [ ] Push to release branch
- [ ] Create and merge PR to main
- [ ] Verify GitHub Actions workflow runs
- [ ] Check https://github.com/algebananazzzzz/planear/releases for new release
- [ ] (Optional) Wait 24-48 hours and verify pkg.go.dev updated

## Questions?

See the main [PUBLISHING.md](./PUBLISHING.md) for more details on versioning strategy and maintenance.
