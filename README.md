# gh-x

GitHub CLI extension for batch repo operations.

## Prerequisites

Install [GitHub CLI](https://cli.github.com/) (`gh`) and authenticate:

```bash
# macOS (Homebrew)
brew install gh

# Other platforms: https://cli.github.com/
gh auth login
```

Upgrade `gh` itself (Homebrew):

```bash
brew upgrade gh
```

## Install

```bash
gh extension install samzong/gh-x
```

Verify:

```bash
gh extension list    # should show samzong/gh-x
gh x --help
```

## Upgrade

```bash
gh extension upgrade x          # upgrade gh-x only
gh extension upgrade --all      # upgrade all extensions
gh extension upgrade x --dry-run  # preview without upgrading
```

### Push to `main` does not update installed extensions

`gh extension install` and `gh extension upgrade` pull **prebuilt binaries from [GitHub Releases](https://github.com/samzong/gh-x/releases)**, not the latest commit on `main`.

| Action | Updates installed `gh x`? |
|--------|---------------------------|
| `git push origin main` | No |
| `gh extension upgrade x` (no new release) | No — reports "already up to date" |
| Push a `v*` tag → CI publishes a release → `gh extension upgrade x` | Yes |

If you pushed code but `gh x` still lacks new commands, check the installed version:

```bash
gh extension list
gh x --help
```

Compare with the latest release: https://github.com/samzong/gh-x/releases

## Usage

```bash
gh x clone <user1> [org2] ...   # Clone missing repos and update existing ones
gh x webhook list <owner/repo|owner>...
gh x webhook list --source <owner>
gh x webhook list --fork --private <owner>
gh x webhook add https://example.com/webhook <owner/repo|owner>...
gh x webhook add --source --public https://example.com/webhook <owner>
gh x webhook add -e push -e pull_request --secret-env WEBHOOK_SECRET https://example.com/webhook <owner/repo|owner>...
gh x webhook delete https://example.com/webhook <owner/repo|owner>...
WEBHOOK_PARALLEL=16 gh x webhook list <owner>
```

## Behavior

- Creates a `<user-or-org>/` directory under your current working directory.
- If a local repo already exists (has `.git`), it runs `git pull --ff-only`.
- If the repo does not exist locally, it runs `gh repo clone`.
- Webhook targets accept either `owner/repo` or an owner/org name. Owner/org names are expanded with `gh repo list`.
- Use `--source`, `--fork`, `--private`, or `--public` to filter owner/org expansion.
- Webhook add matches existing hooks by URL. It updates one matching hook or creates one when missing.
- Webhook add subscribes to all events by default. Use `-e`/`--event` to select specific events.
- Webhook delete removes hooks matching the URL.
- Webhook secrets are read from the named environment variable.
- Webhook list uses `WEBHOOK_PARALLEL` for concurrent read requests. The default is `8`.

## Development

### Local install (test changes without a release)

```bash
go build -o gh-x .
gh extension install . --force
```

`gh` symlinks to the local `gh-x` binary. Rebuild after code changes:

```bash
go build -o gh-x .
```

### Release a new version (maintainers)

Pushing a `v*` tag triggers [`.github/workflows/release.yml`](.github/workflows/release.yml), which precompiles binaries and publishes a GitHub Release.

```bash
git tag v0.0.3
git push origin v0.0.3
```

Wait for the release workflow to finish, then upgrade:

```bash
gh extension upgrade x
```

Checklist after merging to `main`:

1. Tag (`git tag vX.Y.Z && git push origin vX.Y.Z`)
2. Confirm release at https://github.com/samzong/gh-x/releases
3. `gh extension upgrade x`
4. `gh x --help` shows new commands
