#!/usr/bin/env bash
# Reset GhostVault Postgres to an empty vault (schema kept, all rows removed).
# Use after HTTP 409 "vault already initialized" when you want a new passphrase via gvctl init.
# Requires: interactive terminal (three confirmations). Restarts are recommended after wipe
# so in-memory sessions match the DB (see gvsvd logs / docker compose restart ghostvault).

set -euo pipefail

REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$REPO_ROOT"

if [[ ! -t 0 ]]; then
	echo "This script must be run interactively (three confirmations on stdin)." >&2
	exit 1
fi

echo "This will DELETE all vaults and dependent data (chunks, embeddings, sessions, tokens)." >&2
echo "The schema and goose migration version are left intact." >&2
echo >&2

for i in 1 2 3; do
	read -r -p "Confirmation ${i}/3 — type exactly RESET: " line || exit 1
	if [[ "${line}" != "RESET" ]]; then
		echo "Aborted (expected RESET)." >&2
		exit 1
	fi
done

if docker compose exec -T postgres true 2>/dev/null; then
	echo "Using docker compose postgres service…" >&2
	docker compose exec -T postgres psql -U ghostvault -d ghostvault -v ON_ERROR_STOP=1 -c "DELETE FROM vaults;"
elif [[ -n "${DATABASE_URL:-}" ]]; then
	echo "Using DATABASE_URL…" >&2
	psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -c "DELETE FROM vaults;"
else
	echo "Could not reach Postgres: start the stack (make up) or set DATABASE_URL for host psql." >&2
	exit 1
fi

echo >&2
echo "Done. Run: gvctl init -password '…'  (and refresh tokens / GHOSTVAULT_DEFAULT_VAULT_ID if you use direnv)." >&2
