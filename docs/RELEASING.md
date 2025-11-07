# Releasing Planear

This document explains how releases work for Planear—they're fully automated!

## Automatic Semantic Versioning

Planear uses [semantic-release](https://semantic-release.gitbook.io/) to automatically:
- 📊 Analyze your commit history
- 🔄 Determine the next version (major.minor.patch)
- 📝 Generate release notes
- 🏷️ Create git tags
- 🚀 Publish GitHub releases

**Zero manual version management needed!**

## How It Works

### 1. Developers use Conventional Commits

When you make code changes, use the conventional commit format:

```bash
git commit -m "feat(core): add streaming API"          # Minor version bump
git commit -m "fix(retry): fix off-by-one error"      # Patch version bump
git commit -m "feat(api): redesign callbacks

BREAKING CHANGE: OnFinalize signature changed"        # Major version bump
```

### 2. Create Pull Request

Push your feature branch and create a PR against `main`:

```bash
git checkout -b feature/my-feature
# Make your changes...
git add .
git commit -m "feat(feature): add new capability"
git push origin feature/my-feature
```

Then open a PR on GitHub.

### 3. Merge to Main

Once your PR is approved and merged to `main`:
- GitHub Actions automatically triggers
- semantic-release analyzes all commits since last release
- Version is automatically bumped
- CHANGELOG.md is auto-generated
- Git tag is created
- GitHub release is published
- Done! ✅

## Commit Message Format

Follow the Angular commit convention:

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Commit Types

| Type | SemVer | Description |
|------|--------|-------------|
| `feat` | Minor | New feature (backward compatible) |
| `fix` | Patch | Bug fix |
| `docs` | Patch | Documentation changes |
| `perf` | Patch | Performance improvements |
| `refactor` | Patch | Code refactoring |
| `test` | Patch | Test changes |
| `chore` | None | Build, CI, dependencies (no release) |
| `style` | Patch | Code style changes |

### Examples

#### Patch Release (v1.0.0 → v1.0.1)

```bash
git commit -m "fix(retry): correct exponential backoff calculation"
git commit -m "docs: update README with new example"
```

**Result:** Version bumps to v1.0.1

#### Minor Release (v1.0.0 → v1.1.0)

```bash
git commit -m "feat(streaming): add support for large CSV files"
git commit -m "feat(validation): new custom validator API"
git commit -m "fix(pool): memory leak in cleanup"
```

**Result:** Version bumps to v1.1.0

#### Major Release (v1.0.0 → v2.0.0)

```bash
git commit -m "feat(api): redesign callback interface

BREAKING CHANGE: OnFinalize now requires context parameter"
git commit -m "feat(core): new streaming architecture

BREAKING CHANGE: FormatRecord must return error"
```

**Result:** Version bumps to v2.0.0

## Breaking Changes

To trigger a major version bump, include `BREAKING CHANGE:` in your commit footer:

```bash
git commit -m "feat(api): redesign

BREAKING CHANGE: Removed legacy validation callback

Details about what changed and how to migrate..."
```

## Generated Changelog Example

When v1.1.0 is released, CHANGELOG.md looks like:

```markdown
# [1.1.0](https://github.com/algebananazzzzz/planear/compare/v1.0.0...v1.1.0) (2024-11-20)

### ✨ Features

* **streaming:** add support for large CSV files ([abc123](https://github.com/algebananazzzzz/planear/commit/abc123))
* **validation:** new custom validator API ([def456](https://github.com/algebananazzzzz/planear/commit/def456))

### 🐛 Bug Fixes

* **pool:** memory leak in cleanup ([ghi789](https://github.com/algebananazzzzz/planear/commit/ghi789))

# [1.0.0](https://github.com/algebananazzzzz/planear/compare/v1.0.0...v1.0.0) (2024-11-15)

...
```

## Release Workflow Status

Monitor releases in GitHub Actions:

1. **During Release:**
   - View workflow run: https://github.com/algebananazzzzz/planear/actions

2. **After Release:**
   - View release: https://github.com/algebananazzzzz/planear/releases
   - View tags: https://github.com/algebananazzzzz/planear/tags
   - View CHANGELOG: https://github.com/algebananazzzzz/planear/blob/main/CHANGELOG.md

3. **Package Availability:**
   - pkg.go.dev indexes within 24-48 hours
   - Check: https://pkg.go.dev/github.com/algebananazzzzz/planear

## Common Scenarios

### Scenario 1: Bug Fix Only

```bash
git checkout -b fix/bug-name
# Fix the bug...
git commit -m "fix(core): correct null pointer dereference"
git push origin fix/bug-name
# Create PR, merge to main
# ✅ Automatic: Patch release created (v1.0.0 → v1.0.1)
```

### Scenario 2: Feature + Bug Fixes

```bash
git checkout -b feature/new-capability
# Add feature...
git commit -m "feat(apply): add parallel mode configuration"
# Fix unrelated bug...
git commit -m "fix(input): handle empty CSV files"
git push origin feature/new-capability
# Create PR, merge to main
# ✅ Automatic: Minor release created (v1.0.0 → v1.1.0)
```

### Scenario 3: Breaking Changes

```bash
git checkout -b breaking/redesign-api
# Redesign...
git commit -m "feat(core): new callback interface

BREAKING CHANGE: OnUpdate now receives context.Context parameter

Migration: Update your OnUpdate callback to accept context"

git push origin breaking/redesign-api
# Create PR, merge to main
# ✅ Automatic: Major release created (v1.0.0 → v2.0.0)
```

## Best Practices

### ✅ Do

- Write descriptive commit messages
- Use conventional commit format consistently
- Include scope in parentheses for clarity
- Document breaking changes in commit footer
- Review commit messages before merging PR

### ❌ Don't

- Skip commit messages ("fix stuff")
- Mix multiple concerns in one commit
- Forget to mark breaking changes
- Commit directly to main (always use PR)

## Troubleshooting

### No Release Created After Merge

**Possible causes:**
- All commits are `chore:` type (intentional—no release)
- Commits don't follow conventional format
- Branch is not `main`

**Solution:**
Check GitHub Actions logs: https://github.com/algebananazzzzz/planear/actions

### Wrong Version Bump

**Example:** Expected minor (v1.0.0 → v1.1.0) but got patch (v1.0.0 → v1.0.1)

**Cause:** No `feat:` commits found, only `fix:` commits

**Solution:** Use `feat:` prefix for new features

### Breaking Change Not Recognized

**Cause:** Incorrect footer format

**Wrong:**
```
breaking change: OnUpdate signature changed
```

**Correct:**
```
BREAKING CHANGE: OnUpdate signature changed
```

Note the exact capitalization and colon placement.

## Manual Release (If Needed)

If semantic-release workflow fails and you need to manually create a release:

```bash
# 1. Create tag locally
git tag -a v1.2.3 -m "Release v1.2.3"

# 2. Push tag
git push origin v1.2.3

# 3. Create release via gh CLI
gh release create v1.2.3 --generate-notes
```

## Reference

- [semantic-release docs](https://semantic-release.gitbook.io/)
- [Conventional Commits](https://www.conventionalcommits.org/)
- [Angular commit convention](https://github.com/angular/angular/blob/main/CONTRIBUTING.md#-commit-message-guidelines)

## Release Checklist

- [ ] Code changes complete and tested
- [ ] Use conventional commit format (feat:, fix:, docs:, etc.)
- [ ] Run `go test ./...` - all pass
- [ ] Run `go vet ./...` - clean
- [ ] Commit with clear, descriptive message
- [ ] Push to feature branch
- [ ] Create PR against `main`
- [ ] Review and discuss changes
- [ ] Merge PR to `main`
- [ ] Verify GitHub Actions workflow runs
- [ ] Check new release on https://github.com/algebananazzzzz/planear/releases
- [ ] Wait 24-48 hours and verify pkg.go.dev updated

---

That's it! Your releases are now **fully automated** based on your commits. No manual versioning needed! 🚀
