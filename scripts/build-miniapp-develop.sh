#!/usr/bin/env bash
# Canonical local build entrypoint. It prevents WeChat DevTools from being
# refreshed from another worktree by refusing to build outside `develop`.
set -Eeuo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BRANCH="$(git -C "$REPO_ROOT" branch --show-current)"

if [[ "$BRANCH" != "develop" ]]; then
  echo "Refusing to build from branch '$BRANCH'. Use the develop worktree instead." >&2
  exit 1
fi

cd "$REPO_ROOT/miniapp"
npm run build:mp-weixin

echo
echo "WeChat DevTools canonical import path:"
echo "$REPO_ROOT/miniapp/dist/mp-weixin"
