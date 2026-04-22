#!/usr/bin/env bash
# Sanity-check Ghost Vault env as seen by this shell (same vars a local gvmcp child process would get
# if your MCP host merges them into mcpServers.*.env). Does not start MCP.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
if [[ -f "$REPO_ROOT/.env" ]]; then
	set -a
	# shellcheck disable=SC1091
	source "$REPO_ROOT/.env"
	set +a
fi
TOKEN_FILE="${GHOSTVAULT_TOKEN_FILE:-.ghostvault-bearer}"
if [[ "${TOKEN_FILE:0:1}" != "/" ]]; then
	TOKEN_FILE="$REPO_ROOT/$TOKEN_FILE"
fi

auto_unlock=0
case "${GHOSTVAULT_DIRENV_AUTO_UNLOCK:-}" in
	1 | true | TRUE | yes | YES | on | ON) auto_unlock=1 ;;
esac
if [[ "$auto_unlock" -eq 1 ]] && [[ ! -s "$TOKEN_FILE" ]] && [[ -n "${GHOSTVAULT_BASE_URL:-}" ]]; then
	need_pass=0
	case "${GV_ENCRYPTION:-on}" in
		on | true | TRUE | 1) need_pass=1 ;;
	esac
	if [[ "$need_pass" -eq 0 ]] || [[ -n "${GHOSTVAULT_PASSWORD:-}" ]]; then
		if [[ -x "$REPO_ROOT/bin/gvctl" ]]; then
			"$REPO_ROOT/bin/gvctl" unlock -write-token-file "$TOKEN_FILE" || true
		elif command -v go >/dev/null 2>&1; then
			(cd "$REPO_ROOT" && go run ./cmd/gvctl unlock -write-token-file "$TOKEN_FILE") || true
		fi
	fi
fi

if [[ -f "$TOKEN_FILE" ]] && [[ -s "$TOKEN_FILE" ]]; then
	export GHOSTVAULT_BEARER_TOKEN="$(tr -d '\n\r' < "$TOKEN_FILE")"
fi

mask_val() {
	local s="${1:-}"
	local n=${#s}
	if [[ -z "$s" ]]; then
		echo "<unset or empty>"
		return
	fi
	if (( n <= 8 )); then
		echo "<len=$n>"
		return
	fi
	echo "${s:0:4}…${s: -4}(len=$n)"
}

echo "=== Ghost Vault MCP-related environment (masked) ==="
echo "GHOSTVAULT_BASE_URL=${GHOSTVAULT_BASE_URL:-<unset>}"
echo "GHOSTVAULT_BEARER_TOKEN=$(mask_val "${GHOSTVAULT_BEARER_TOKEN:-}")"
echo "GHOSTVAULT_DEFAULT_VAULT_ID=${GHOSTVAULT_DEFAULT_VAULT_ID:-<unset>}"
echo "GHOSTVAULT_DEFAULT_USER_ID=${GHOSTVAULT_DEFAULT_USER_ID:-<unset>}"
echo "GHOSTVAULT_DEBUG_AUTH=${GHOSTVAULT_DEBUG_AUTH:-<unset>}"
echo "GHOSTVAULT_DEBUG_AUTH_FULL=${GHOSTVAULT_DEBUG_AUTH_FULL:-<unset>}"
echo

BASE="${GHOSTVAULT_BASE_URL:-}"
if [[ -z "$BASE" ]]; then
	echo "Set GHOSTVAULT_BASE_URL to run HTTP checks (e.g. http://127.0.0.1:8989/api)."
	exit 0
fi

BASE="${BASE%/}"
echo "=== GET $BASE/healthz ==="
if curl -sfS "$BASE/healthz" | head -c 200; then
	echo
	echo "healthz: ok"
else
	echo "healthz: failed (is gvsvd up and URL correct?)"
	exit 1
fi

VID="${GHOSTVAULT_DEFAULT_VAULT_ID:-}"
USER_ID="${GHOSTVAULT_DEFAULT_USER_ID:-test-user}"
TOK="${GHOSTVAULT_BEARER_TOKEN:-}"
if [[ -z "$TOK" || -z "$VID" ]]; then
	echo
	echo "Skip ingest probe: set GHOSTVAULT_BEARER_TOKEN and GHOSTVAULT_DEFAULT_VAULT_ID to POST /v1/ingest."
	exit 0
fi

echo
echo "=== POST $BASE/v1/ingest (probe) ==="
code=$(curl -sS -o /tmp/gv-ingest-body.$$ -w '%{http_code}' \
	-X POST "$BASE/v1/ingest" \
	-H "Content-Type: application/json" \
	-H "Authorization: Bearer $TOK" \
	-d "{\"vault_id\":\"$VID\",\"user_id\":\"$USER_ID\",\"text\":\"mcp-env-check probe $(date -u +%Y-%m-%dT%H:%M:%SZ)\"}")
cat /tmp/gv-ingest-body.$$
rm -f /tmp/gv-ingest-body.$$
echo
echo "ingest HTTP status: $code"
if [[ "$code" == "401" ]]; then
	echo "401: token rejected (wrong value, expired idle/max, or gvsvd restarted). Unlock again; compare token_fp with GV_DEBUG_AUTH logs on gvsvd."
	exit 1
fi
if [[ "$code" != "200" ]]; then
	exit 1
fi
