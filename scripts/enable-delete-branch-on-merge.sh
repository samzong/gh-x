#!/usr/bin/env bash
set -euo pipefail

OWNER="${1:-$(gh api user --jq .login)}"

gh repo list "$OWNER" --limit 1000 --json nameWithOwner,isArchived --jq \
  '.[] | select(.isArchived == false) | .nameWithOwner' |
while read -r repo; do
  echo "Enabling delete-branch-on-merge: $repo"
  gh repo edit "$repo" --delete-branch-on-merge
done
