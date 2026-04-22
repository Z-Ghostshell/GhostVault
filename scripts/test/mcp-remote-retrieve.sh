#!/usr/bin/env bash
# Exercise remote (or local) GhostVault streamable MCP: initialize session, call memory_search.
# Confirms the same path Claude Desktop uses with mcp-remote + HTTP MCP (gvmcp behind edge).
#
# Requires: curl, jq
# Token: GHOSTVAULT_BEARER_TOKEN or GHOSTVAULT_TOKEN_FILE (default .ghostvault-bearer) — used to
#   (1) GET /v1/stats for vault_id when GHOSTVAULT_DEFAULT_VAULT_ID is unset
#   (2) optional Authorization header to the MCP endpoint if GHOSTVAULT_MCP_SEND_AUTHORIZATION=1
# The gvsvd session must match what **gvmcp on the server** was started with (Compose env / token file);
# otherwise memory_search returns HTTP 401 inside the tool result.
#
# Env:
#   GHOSTVAULT_MCP_URL   — required, e.g. https://your-node.ts.net/mcp/ (trailing slash ok)
#   GHOSTVAULT_BASE_URL  — gvsvd API origin for stats, e.g. https://your-node.ts.net/api (no trailing slash)
#   GHOSTVAULT_DEFAULT_VAULT_ID — optional; skips stats if set
#   GHOSTVAULT_DEFAULT_USER_ID — default default
#   GHOSTVAULT_TEST_QUERY — search string (default: memory)
#   GHOSTVAULT_MCP_SEND_AUTHORIZATION — if 1, send Authorization: Bearer <same token> to /mcp/ (only if you added edge auth)
#
# Usage:
#   GHOSTVAULT_MCP_URL=https://zf-mac.example.ts.net/mcp/ \
#   GHOSTVAULT_BASE_URL=https://zf-mac.example.ts.net/api \
#   ./scripts/test/mcp-remote-retrieve.sh
#
# Same client as Claude Desktop (run in terminal to verify mcp-remote can reach the server):
#   export GHOSTVAULT_BEARER_TOKEN='…'   # optional: only if you forward this header to nginx
#   exec npx -y mcp-remote 'https://zf-mac.example.ts.net/mcp/' --transport http-only \
#     --header "Authorization: Bearer ${GHOSTVAULT_BEARER_TOKEN}"
#
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# Load .env as *fallback only*: variables already set in the caller's environment
# (direnv, explicit command-line `VAR=… ./script.sh`, etc.) must not be clobbered by
# stale values in .env — that silently re-introduces deleted vault ids and old tokens.
if [[ -f "$REPO_ROOT/.env" ]]; then
	while IFS= read -r line || [[ -n "$line" ]]; do
		[[ "$line" =~ ^[[:space:]]*# ]] && continue
		[[ -z "${line//[[:space:]]/}" ]] && continue
		if [[ "$line" =~ ^[[:space:]]*([A-Za-z_][A-Za-z0-9_]*)=(.*)$ ]]; then
			k="${BASH_REMATCH[1]}"
			v="${BASH_REMATCH[2]}"
			# strip optional surrounding single or double quotes
			if [[ ${#v} -ge 2 ]]; then
				if [[ ${v:0:1} == '"' && ${v: -1} == '"' ]]; then v="${v:1:${#v}-2}"; fi
				if [[ ${v:0:1} == "'" && ${v: -1} == "'" ]]; then v="${v:1:${#v}-2}"; fi
			fi
			if [[ -z "${!k:-}" ]]; then
				export "$k=$v"
			fi
		fi
	done < "$REPO_ROOT/.env"
fi

BASE="${GHOSTVAULT_BASE_URL:-}"
RAW_MCP="${GHOSTVAULT_MCP_URL:-}"
if [[ -z "$RAW_MCP" || -z "$BASE" ]]; then
	echo "Set GHOSTVAULT_MCP_URL and GHOSTVAULT_BASE_URL (example: …/mcp/ and …/api)." >&2
	exit 1
fi
BASE="${BASE%/}"
# Streamable client POST target (nginx location is /mcp/).
MCP_URL="${RAW_MCP%/}/"
case "$MCP_URL" in
	https://* | http://*) ;;
	*) echo "GHOSTVAULT_MCP_URL must start with http:// or https://" >&2; exit 1 ;;
esac

TOKEN_FILE="${GHOSTVAULT_TOKEN_FILE:-.ghostvault-bearer}"
if [[ "${TOKEN_FILE:0:1}" != "/" ]]; then
	TOKEN_FILE="$REPO_ROOT/$TOKEN_FILE"
fi
TOK="${GHOSTVAULT_BEARER_TOKEN:-}"
if [[ -z "$TOK" ]] && [[ -f "$TOKEN_FILE" ]] && [[ -s "$TOKEN_FILE" ]]; then
	TOK="$(tr -d '\n\r' < "$TOKEN_FILE")"
fi
if [[ -z "$TOK" ]]; then
	echo "Set GHOSTVAULT_BEARER_TOKEN or create non-empty $TOKEN_FILE (session after unlock)." >&2
	exit 1
fi

USER_ID="${GHOSTVAULT_DEFAULT_USER_ID:-default}"
QUERY="${GHOSTVAULT_TEST_QUERY:-memory}"
VID="${GHOSTVAULT_DEFAULT_VAULT_ID:-}"
tmp="$(mktemp)"
hdr="$(mktemp)"
body="$(mktemp)"
trap 'rm -f "$tmp" "$hdr" "$body"' EXIT

if [[ -z "$VID" ]]; then
	echo "=== GET $BASE/v1/stats (vault_id) ==="
	curl -sfS -H "Authorization: Bearer $TOK" "$BASE/v1/stats" -o "$tmp"
	VID="$(jq -r '.vault_id' "$tmp")"
	if [[ -z "$VID" || "$VID" == "null" ]]; then
		echo "Could not read vault_id from stats" >&2
		exit 1
	fi
	echo "vault_id=$VID"
else
	echo "Using GHOSTVAULT_DEFAULT_VAULT_ID=$VID"
fi

AUTH_ARGS=()
if [[ "${GHOSTVAULT_MCP_SEND_AUTHORIZATION:-0}" == "1" ]]; then
	AUTH_ARGS=(-H "Authorization: Bearer $TOK")
fi

mcp_post() {
	local payload="$1"
	local -a sess_args=()
	if [[ -n "${MCP_SESSION_ID:-}" ]]; then
		sess_args=(-H "Mcp-Session-Id: $MCP_SESSION_ID")
	fi
	HTTP_CODE="$(curl -sS -o "$body" -w '%{http_code}' -X POST "$MCP_URL" \
		-H "Content-Type: application/json" \
		-H "Accept: application/json, text/event-stream" \
		-H "Mcp-Protocol-Version: 2025-06-18" \
		"${AUTH_ARGS[@]}" \
		"${sess_args[@]}" \
		-D "$hdr" \
		-d "$payload")"
}

echo "=== MCP POST initialize ==="
INIT_PAYLOAD="$(jq -n '{jsonrpc:"2.0",id:1,method:"initialize",params:{protocolVersion:"2025-06-18",capabilities:{},clientInfo:{name:"gv-mcp-remote-test",version:"0.1.0"}}}')"
mcp_post "$INIT_PAYLOAD"
echo "HTTP $HTTP_CODE"
if [[ "$HTTP_CODE" != "200" ]]; then
	cat "$body" >&2
	exit 1
fi
MCP_SESSION_ID=""
while IFS= read -r line || [[ -n "$line" ]]; do
	line="${line%$'\r'}"
	case "${line,,}" in
		mcp-session-id:*) MCP_SESSION_ID="${line#*: }"; MCP_SESSION_ID="${MCP_SESSION_ID#"${MCP_SESSION_ID%%[![:space:]]*}"}" ;;
	esac
done < "$hdr"
if [[ -z "$MCP_SESSION_ID" ]]; then
	echo "WARN: no Mcp-Session-Id in response headers; tools/call may fail." >&2
fi

echo "=== MCP POST notifications/initialized ==="
mcp_post "$(jq -n '{jsonrpc:"2.0",method:"notifications/initialized",params:{}}')"
echo "HTTP $HTTP_CODE"

echo "=== MCP POST tools/call memory_search ==="
CALL_PAYLOAD="$(jq -n \
	--arg vid "$VID" \
	--arg uid "$USER_ID" \
	--arg q "$QUERY" \
	'{jsonrpc:"2.0",id:2,method:"tools/call",params:{name:"memory_search",arguments:{vault_id:$vid,user_id:$uid,query:$q,max_chunks:5,max_tokens:800}}}')"
mcp_post "$CALL_PAYLOAD"
echo "HTTP $HTTP_CODE"
jq . "$body"

IS_ERR="$(jq -r '.result.isError // false' "$body")"
if [[ "$IS_ERR" == "true" ]]; then
	echo "FAIL: tool returned isError=true (see content — often gvsvd 401 if server gvmcp bearer is stale)." >&2
	exit 1
fi

TEXT="$(jq -r '.result.content[0].text // empty' "$body")"
if [[ -z "$TEXT" ]]; then
	echo "FAIL: empty tool text content" >&2
	exit 1
fi
if ! echo "$TEXT" | jq -e . >/dev/null 2>&1; then
	echo "WARN: tool text is not JSON: ${TEXT:0:200}" >&2
	exit 0
fi
N="$(echo "$TEXT" | jq -r '(.results // []) | length')"
echo "=== PASS: memory_search returned JSON with results length=$N ==="
