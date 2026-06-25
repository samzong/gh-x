# gh-x

GitHub CLI extension for batch repo operations.

## Install

```bash
gh extension install samzong/gh-x
```

## Usage

```bash
gh x clone <user1> [org2] ...   # Clone missing repos and update existing ones
gh x webhook list <owner/repo|owner>...
gh x webhook list --source <owner>
gh x webhook list --fork --private <owner>
gh x webhook add https://example.com/webhook <owner/repo|owner>...
gh x webhook add --source --public https://example.com/webhook <owner>
gh x webhook add -e push -e pull_request --secret-env WEBHOOK_SECRET https://example.com/webhook <owner/repo|owner>...
WEBHOOK_PARALLEL=16 gh x webhook list <owner>
```

## Behavior

- Creates a `<user-or-org>/` directory under your current working directory.
- If a local repo already exists (has `.git`), it runs `git pull --ff-only`.
- If the repo does not exist locally, it runs `gh repo clone`.
- Webhook targets accept either `owner/repo` or an owner/org name. Owner/org names are expanded with `gh repo list`.
- Use `--source`, `--fork`, `--private`, or `--public` to filter owner/org expansion.
- Webhook secrets are read from the named environment variable.
- Webhook list uses `WEBHOOK_PARALLEL` for concurrent read requests. The default is `8`.
