# Releasing Planear

Releases are cut automatically on every push to `main` by `.github/workflows/release.yml`, using [`anothrNick/github-tag-action`](https://github.com/anothrNick/github-tag-action).

## What happens on merge to `main`

1. The action looks at commit messages since the last tag.
2. It picks the bump from any `#major` / `#minor` / `#patch` / `#none` token found in those messages. If none is present, it defaults to **patch**.
3. A new tag (`vX.Y.Z`, with `WITH_V: true`) is created and pushed.
4. A GitHub release is created with a `compare` link to the previous tag.

`CHANGELOG.md` is **not** auto-generated — see [CHANGELOG.md](../CHANGELOG.md) for how it's maintained.

## Controlling the bump from a commit

Add the token to the commit message (or PR squash-merge message):

```
fix: handle empty CSV files       #patch
feat: add streaming API           #minor
refactor: redesign callbacks      #major
chore: bump deps                  #none
```

Tokens are matched anywhere in the commit message. Default is `#patch` when no token is given, so most bug fixes need no marker.

## Manual release

If the workflow fails or you need to tag out-of-band:

```bash
git tag -a v1.2.3 -m "Release v1.2.3"
git push origin v1.2.3
gh release create v1.2.3 --generate-notes
```

## Checking the result

- Releases: https://github.com/algebananazzzzz/planear/releases
- Workflow runs: https://github.com/algebananazzzzz/planear/actions
- `pkg.go.dev` indexes within 24–48 hours: https://pkg.go.dev/github.com/algebananazzzzz/planear
