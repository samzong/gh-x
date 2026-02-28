# gh-x

GitHub CLI extension for batch repo operations.

## Install

```bash
gh extension install samzong/gh-x
```

## Usage

```bash
gh x clone <user1> [org2] ...   # Clone missing repos and update existing ones
```

## Behavior

- Creates a `<user-or-org>/` directory under your current working directory.
- If a local repo already exists (has `.git`), it runs `git pull --ff-only`.
- If the repo does not exist locally, it runs `gh repo clone`.
