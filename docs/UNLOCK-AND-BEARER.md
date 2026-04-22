# Auth modes, unlock, and Bearer token security

This document describes GhostVault's credential model. Every vault has **two independent** dimensions, both chosen once at init and immutable thereafter:

| Dimension | Values | Controls | Configured via |
|-----------|--------|----------|----------------|
| **Encryption** | `on` / `off` | Whether chunks are encrypted at rest with a DEK | `GV_ENCRYPTION` env + `POST /v1/vault/init` |
| **Auth mode** | `session` / `auto_unlock` / `api_keys` | How bearers are issued and whether DEK survives restart | `gvctl init -auth-mode …` (stored on the `vaults` row) |

The HTTP wire format is the same in all modes: `Authorization: Bearer <token>`. What differs is how tokens are minted, how long they live, and where the DEK comes from.

---

## 1. Concepts

- **Vault** — one row in the `vaults` table; holds `encryption_enabled`, `auth_mode`, and (if encrypted) the wrapped DEK. A single gvsvd process serves one vault.
- **DEK (Data Encryption Key)** — 32-byte symmetric key used to seal/open chunks. Never stored unwrapped; never logged. Lives only in the gvsvd process `VaultState` struct during operation.
- **Session** — a `(token_hash, vault_id, expires_at, max_until)` row in the `sessions` table plus an in-memory cache entry. Holds the capability to call `/v1/ingest`, `/v1/retrieve`, `/v1/stats`, etc. The raw token is never persisted.
- **Credential / CredentialResolver** — internal abstraction (`internal/auth/credential.go`). Each auth mode implements `CredentialResolver.Resolve(bearer) → Credential`. Handlers never branch on mode.

---

## 2. Init: choosing `auth_mode`

`POST /v1/vault/init` accepts:

```json
{ "password": "…", "auth_mode": "session" }
```

`auth_mode` defaults to `session` when omitted. `gvctl init -auth-mode …` threads the flag. Valid values: `session`, `auto_unlock`, `api_keys` (reserved — returns `400 mode_not_implemented` in this build).

Unauthenticated `GET /v1/vault` returns only `{ "initialized": true }` once a vault row exists (or `404` before init) so callers can probe readiness without exposing `vault_id`, encryption, or `auth_mode`. Use `POST /v1/vault/init` / `unlock` responses or authenticated routes for full metadata.

### Threat-model comparison

| | `session` | `auto_unlock` | `api_keys` (future) |
|---|---|---|---|
| **Who holds the password?** | The human, per-unlock | The server filesystem (password file) | Nobody — each client holds a DEK-wrapping key |
| **Sessions survive `gvsvd` restart?** | ❌ purged on boot | ✅ persisted in `sessions` table | N/A — no shared sessions |
| **Client needs to re-unlock after restart?** | ✅ yes → `make rotate-token` | ❌ no | ❌ no |
| **Compromise of server host = vault compromise?** | Only when unlocked (DEK in RAM) | **Yes, always** (password on disk) | Yes for revoked-but-still-in-use keys until re-wrap |
| **Per-client revocation?** | `gvctl lock` on the session | Same | Per-key DELETE; DEK rotation re-wraps survivors |
| **Recommended session TTL** | idle 1h / max 12h (interactive) | idle 30d / max 90d (long-lived) | N/A |
| **Implemented today?** | ✅ | ✅ | ❌ (stub only — see [future/API-KEYS.md](./future/API-KEYS.md)) |

The right-most column is intentionally separate; picking between `session` and `auto_unlock` is the main operational decision.

> **Immutability:** `auth_mode` is stored on the vault row and cannot be changed via the API. Moving an existing vault from `session` → `auto_unlock` requires re-init (the chunk data cannot currently be migrated to a different mode within this codebase; a migration tool is out of scope for this plan).

---

## 3. Mode: `session`

Today's default. Interactive unlock, RAM-only sessions.

### Init

```bash
gvctl init -auth-mode session          # or just `gvctl init` (session is default)
```

### Unlock flow

1. `POST /v1/vault/unlock` with `{"password":"…"}` (encryption on) or empty body (encryption off).
2. Server derives KEK via Argon2id, unwraps the DEK, stores it in process `VaultState`, mints a `session_token`, and returns it. `session_token` is `sha256`-hashed and written to the `sessions` table; the raw token goes back on the wire exactly once.
3. Client uses `Authorization: Bearer <session_token>` thereafter.

### Restart semantics

On `gvsvd` boot, **every session row for the vault is purged**. This preserves today's behavior: restart invalidates all outstanding bearers. Clients must re-unlock and re-mint the token (see §6 `make rotate-token`).

### When to use

- Local / development.
- Small teams where a human is always around to re-unlock after a restart.
- Compliance settings where a server restart SHOULD cut all live sessions.

---

## 4. Mode: `auto_unlock`

Server holds the password; DEK is unwrapped at boot; sessions survive restart.

### Init

```bash
gvctl init -auth-mode auto_unlock -password "$VAULT_PASSWORD"
```

### Where the password lives

`gvsvd` reads the password at startup in this order:

1. **`GHOSTVAULT_PASSWORD_FILE`** (preferred) — path to a file containing only the password (trailing newline is trimmed). Mount as a Docker secret, K8s secret, or a file on disk with `0400` perms owned by the gvsvd user.
2. **`GHOSTVAULT_PASSWORD`** (env var fallback) — logged with a warning on boot so you know you're using the less-safe path. Useful for laptop development.

If neither is set and encryption is on, `gvsvd` **refuses to serve** (fail-closed). Wrong password → same result (DEK unwrap fails → process exits with a non-zero error).

Password and DEK are never logged. The boot log line is: `auth_mode=auto_unlock vault=<uuid> (DEK unwrapped, sessions persist across restart)`.

### Restart semantics

- Sessions table is **not** purged.
- At boot, DEK is re-derived from the password file and re-inserted into `VaultState`.
- Existing bearers remain valid (as long as they're inside their idle/max TTL — tune both longer with `GV_SESSION_IDLE_MINUTES` / `GV_SESSION_MAX_HOURS`; see `.env.example`).

A client that unlocked once and saved the bearer in `~/.config/…` can `docker compose restart ghostvault` indefinitely and keep calling the API.

### Threat model

- **Host compromise ≈ vault compromise.** An attacker who reads the password file can unwrap the DEK offline against any copy of the ciphertext blob. Treat the host like it holds the plaintext — same level of isolation (firewall, VPN, tenant boundary) you'd apply to a plain-text vault.
- Prefer `GHOSTVAULT_PASSWORD_FILE` with `0400` perms and a non-root process owner. Avoid baking the password into `.env` on shared machines (direnv exports it into every subshell).
- Revocation is per-session: `gvctl lock` deletes one row in `sessions`. To invalidate **all** bearers without rotating the password, truncate `sessions`: `docker compose exec postgres psql -U ghostvault -c "DELETE FROM sessions WHERE vault_id='<uuid>';"`.
- Rotating the password is an out-of-scope operation today (requires re-wrapping the DEK).

### When to use

- Headless servers (home lab, a tailnet node, a VPS) where a human can't re-unlock after every reboot.
- Remote MCP clients (Claude Desktop, Cursor, ChatGPT Custom GPT) where the client machine doesn't hold the vault password and can't run `make rotate-token`.
- Single-tenant deployments where the server host is the trust boundary.

---

## 5. Mode: `api_keys` (future)

Reserved for Part 3 of the roadmap. Each client gets a long-lived API key that doubles as a per-client KEK: the server stores `wrapped_dek` per key, unwraps on each request, and supports per-key revocation plus DEK re-wrap on rotation. No server-side password.

Design lives in [future/API-KEYS.md](./future/API-KEYS.md). `auth_mode=api_keys` is accepted by the CHECK constraint so the migration path is forward-compatible, but `POST /v1/vault/init` with that value currently returns `400 mode_not_implemented`.

---

## 6. Operations: `make rotate-token`, direnv, sanity checks

### Refreshing the token on Docker (`session` mode)

`gvmcp` in Compose resolves its Bearer **once at startup** from the mounted token file (`/run/ghostvault/bearer` ← `./.ghostvault-bearer`). A fresh `gvctl unlock` mints a new session on `gvsvd` and overwrites the file, but the running `gvmcp` container still holds the **old** value in memory — you must recreate it.

```bash
make rotate-token
```

That runs `refresh-token` (builds `bin/gvctl` if needed, unlocks, writes `.ghostvault-bearer`) and then `docker compose --profile mcp up -d gvmcp` so the MCP container is recreated against the fresh file. It's a no-op if the `mcp` profile isn't enabled.

In `auto_unlock` mode, `make rotate-token` still works but is rarely necessary — the previous bearer keeps working across restarts, so you only run it to deliberately revoke the old session.

### The 401 hunt list

`ghostvault HTTP 401: {"title":"Unauthorized","status":401,"detail":"unauthorized","code":"auth"}` means the Bearer `gvmcp` holds does not match any live session on `gvsvd`. In order of likelihood:

1. **`gvsvd` was restarted in `session` mode** (look for a fresh `gvsvd listening on …` log line). All in-memory sessions were cleared and the `sessions` rows were purged. Fix: `make rotate-token`. This does not happen in `auto_unlock` mode.
2. **Stale `GHOSTVAULT_BEARER_TOKEN` exported by `.envrc`.** The project `.envrc` reads `.ghostvault-bearer` on every `cd` and exports it into your shell. If you manually `docker compose restart gvmcp`, Compose can pick that stale value up — but only if the Compose spec still wires `GHOSTVAULT_BEARER_TOKEN` into `gvmcp`. The shipped spec does **not** wire it, so the container is strictly **file-only**; if you re-added it locally, expect this footgun back. Fix: `make rotate-token` (the target also `unset`s the var before invoking Compose) or leave the Compose spec file-only.
3. **Mode mismatch.** `/v1/vault/unlock` in `auth_mode=api_keys` returns `409 mode_mismatch`.
4. **`gvctl` pointed at a different `gvsvd`** than your tunnel. Unlock minted a session on host A but MCP calls reach host B. Fix: same `GHOSTVAULT_BASE_URL` on both sides.

### Sanity check without MCP

```bash
TOK="$(tr -d '\n\r' < .ghostvault-bearer)"
curl -sS -o /dev/null -w '%{http_code}\n' \
  -H "Authorization: Bearer $TOK" \
  https://YOUR-HOST/api/v1/stats
```

`200` means the Bearer is good on `gvsvd`. If that passes but MCP still returns 401, recreate `gvmcp` (or `make rotate-token` again).

---

## 7. Bearer token — security properties (mode-agnostic)

- **Not a signed JWT.** The session token is a random 32-byte hex string mapped on the server to `(vault_id, DEK from VaultState, timing)`. Compromise of the string is compromise of the session's capabilities until it idles out, hits the max TTL, is locked, or (in `session` mode) the server restarts.
- **Hash-at-rest.** The `sessions` table stores `sha256(token)` only; a Postgres dump does not leak live bearers.
- **TLS is mandatory** on any path that crosses a network. The app does not attempt to detect replay and relies on TLS to prevent passive capture.
- **Unauthenticated routes** remain: `/healthz`, `/readyz`, `/v1/vault/init` (empty DB only), `/v1/vault/unlock` (rejects `api_keys` mode), and `GET /v1/vault`. If the edge is reachable by strangers, plaintext vaults effectively allow anonymous unlock — use firewall / VPN / reverse-proxy auth in front of `gvsvd` whenever the network is wider than a single host.
- **Exposed MCP HTTP:** if `/mcp/` is public, attackers may invoke tools while `gvmcp` already holds a valid Bearer toward `gvsvd`. Treat public MCP exactly like public vault access for whatever that Bearer can do.
- **Debug logging:** `GV_DEBUG_AUTH=true` logs bearer fingerprints on 401s; `GV_DEBUG_AUTH_FULL=true` additionally logs the full token (only for isolated troubleshooting). Neither is ever safe in production.

---

## 8. Convenience: `gvctl`

`gvctl init -auth-mode …` sets the mode at init time. `gvctl unlock` posts to `/v1/vault/unlock` (accepted in `session` and `auto_unlock` modes; returns `409` in `api_keys`) and can print the token or write it to a file (`-write-token-file`, e.g. `.ghostvault-bearer`) for Compose mounts or direnv. `GHOSTVAULT_PASSWORD` is **client-only** convenience for `gvctl`; the server reads it from the env only in `auto_unlock` mode (and still prefers `GHOSTVAULT_PASSWORD_FILE`).

Testing **streamable MCP** end-to-end (initialize → `notifications/initialized` → `tools/call memory_search`) is automated by `scripts/test/mcp-remote-retrieve.sh` — it expects `GHOSTVAULT_MCP_URL` (e.g. `https://…/mcp/`) and `GHOSTVAULT_BASE_URL` (e.g. `https://…/api`) plus a current session token.

---

## See also

- [deploy.md](./deploy.md) — Compose, edge paths, tunnels
- [integration/Claude-mcp.md](./integration/Claude-mcp.md) — `gvmcp` configuration
- [integration/OPENAPI.md](./integration/OPENAPI.md) — HTTPS and Bearer for Actions clients
- [future/API-KEYS.md](./future/API-KEYS.md) — third mode design sketch
- [future/THREAT-MODEL.md](./future/THREAT-MODEL.md) — broader adversary framing
