#!/usr/bin/env bash
# End-to-end check: GET /v1/stats → POST /v1/ingest (infer=false) → stats + retrieve.
# Confirms the write path persists rows for the same vault as the Bearer token.
#
# Requires: curl, jq; gvsvd up; valid session in GHOSTVAULT_TOKEN_FILE (default .ghostvault-bearer).
# Env: GHOSTVAULT_BASE_URL (default http://127.0.0.1:8989/api), optional GHOSTVAULT_DEFAULT_USER_ID (default default).
#
# Usage: ./scripts/test/ingest-write-verify.sh
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

if [[ ! -f "$TOKEN_FILE" ]] || [[ ! -s "$TOKEN_FILE" ]]; then
	echo "Missing or empty token file: $TOKEN_FILE" >&2
	echo "Run: make refresh-token   (or gvctl unlock -write-token-file .ghostvault-bearer)" >&2
	exit 1
fi

TOK="$(tr -d '\n\r' < "$TOKEN_FILE")"
BASE="${GHOSTVAULT_BASE_URL:-http://127.0.0.1:8989/api}"
BASE="${BASE%/}"
USER_ID="${GHOSTVAULT_DEFAULT_USER_ID:-default}"

tmp="$(mktemp)"
cleanup() { rm -f "$tmp"; }
trap cleanup EXIT

echo "=== $BASE/healthz ==="
curl -sfS "$BASE/healthz" && echo

echo "=== GET /v1/stats (before) ==="
curl -sfS -H "Authorization: Bearer $TOK" "$BASE/v1/stats" -o "$tmp"
jq '{vault_id, chunks_total, ingest_events, recent_activity: (.recent_activity|length)}' "$tmp"
VID="$(jq -r '.vault_id' "$tmp")"
if [[ -z "$VID" || "$VID" == "null" ]]; then
	echo "Could not read vault_id from stats" >&2
	exit 1
fi
BEFORE_CHUNKS="$(jq -r '.chunks_total' "$tmp")"
BEFORE_INGEST="$(jq -r '.ingest_events.total' "$tmp")"

MARKER="gv-write-verify $(date -u +%Y-%m-%dT%H:%M:%SZ)-$$-${RANDOM}"

echo
echo "=== POST /v1/ingest (infer=false, unique text) ==="
INGEST_PAYLOAD="$(jq -n \
	--arg vid "$VID" \
	--arg uid "$USER_ID" \
	--arg text "User's favorite fruit is banana. $MARKER" \
	'{vault_id:$vid, user_id:$uid, text:$text, infer:false}')"

HTTP_CODE="$(curl -sS -o "$tmp" -w '%{http_code}' \
	-X POST "$BASE/v1/ingest" \
	-H "Content-Type: application/json" \
	-H "Authorization: Bearer $TOK" \
	-d "$INGEST_PAYLOAD")"

echo "HTTP $HTTP_CODE"
cat "$tmp" | jq . 2>/dev/null || cat "$tmp"
echo

if [[ "$HTTP_CODE" != "200" ]]; then
	echo "ingest failed (expected 200). Fix auth, OPENAI_API_KEY on gvsvd, or vault mismatch." >&2
	exit 1
fi

CHUNK_COUNT="$(jq -r '(.chunk_ids // []) | length' "$tmp")"
if [[ "$CHUNK_COUNT" -eq 0 ]]; then
	echo "WARN: ingest returned 200 but chunk_ids is empty (dedup hit or no text chunks?)." >&2
fi

echo "=== GET /v1/stats (after) ==="
curl -sfS -H "Authorization: Bearer $TOK" "$BASE/v1/stats" -o "$tmp"
jq '{vault_id, chunks_total, ingest_events, recent_activity: (.recent_activity|length)}' "$tmp"
AFTER_CHUNKS="$(jq -r '.chunks_total' "$tmp")"
AFTER_INGEST="$(jq -r '.ingest_events.total' "$tmp")"
RECENT_N="$(jq -r '.recent_activity|length' "$tmp")"

echo
echo "=== Checks ==="
ok=0
if [[ "$AFTER_CHUNKS" -gt "$BEFORE_CHUNKS" ]] || [[ "$AFTER_INGEST" -gt "$BEFORE_INGEST" ]]; then
	echo "PASS: counts increased (chunks $BEFORE_CHUNKS → $AFTER_CHUNKS, ingest_events.total $BEFORE_INGEST → $AFTER_INGEST)"
	ok=1
else
	echo "FAIL: stats did not increase after successful ingest — write may not be persisting for this vault/DB." >&2
fi

if [[ "$RECENT_N" -eq 0 ]] && [[ "$CHUNK_COUNT" -gt 0 ]]; then
	echo "FAIL: recent_activity empty but chunk_ids non-empty (ingest_history mismatch?)." >&2
	ok=0
fi

echo
echo "=== POST /v1/retrieve (query includes marker) ==="
curl -sfS -H "Authorization: Bearer $TOK" -H "Content-Type: application/json" \
	-d "$(jq -n \
		--arg vid "$VID" \
		--arg uid "$USER_ID" \
		--arg q "$MARKER" \
		'{vault_id:$vid, user_id:$uid, query:$q, max_chunks:5, max_tokens:800}')" \
	"$BASE/v1/retrieve" -o "$tmp"
jq '{results: ((.results // []) | length)}' "$tmp"
R_LEN="$(jq -r '(.results // []) | length' "$tmp")"
if [[ "$CHUNK_COUNT" -gt 0 ]] && [[ "$R_LEN" -eq 0 ]]; then
	echo "WARN: retrieve returned no results for marker query (indexing delay unlikely with sync insert — check user_id / model)." >&2
fi

if [[ "$ok" -eq 1 ]]; then
	exit 0
fi
exit 1
