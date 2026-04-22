#!/usr/bin/env bash
# Unlock gvsvd and store session_token for direnv (overrides GHOSTVAULT_BEARER_TOKEN from .env).
# Requires GHOSTVAULT_PASSWORD in .env when GV_ENCRYPTION=on. Run after container restart.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
TOKEN_FILE="${GHOSTVAULT_TOKEN_FILE:-.ghostvault-bearer}"
GVCTL="${ROOT}/bin/gvctl"
if [[ ! -x "$GVCTL" ]]; then
  GVCTL=(go run ./cmd/gvctl)
else
  GVCTL=("$GVCTL")
fi
if [[ -f .env ]]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi
"${GVCTL[@]}" unlock -write-token-file "$TOKEN_FILE"
echo "direnv: allow then open a new shell, or: export GHOSTVAULT_BEARER_TOKEN=\$(tr -d '\n\r' < \"$TOKEN_FILE\")"
