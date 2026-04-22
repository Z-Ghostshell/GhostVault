# `auth_mode=api_keys` — per-client DEK-wrapping credentials (roadmap)

> **Status:** design sketch only. `POST /v1/vault/init` currently returns `400 mode_not_implemented` when this mode is requested. The `vaults.auth_mode` CHECK constraint already allows the value so the migration path is forward-compatible with the other two modes.

## Why

`session` requires a human to re-unlock after every `gvsvd` restart. `auto_unlock` fixes that but puts the vault password on the server's filesystem, so host compromise equals vault compromise.

`api_keys` aims for both:

- **No server-side password** — the server only stores per-key material that's useless without the raw API key.
- **Long-lived bearers** — each key is stable across restarts; clients never need to re-provision.
- **Per-client revocation** — deleting one row kills one client without touching the others.
- **DEK rotation friendly** — re-wrap every active key in one transaction; no key re-issue needed.

It is the intended mode for "many MCP clients across devices + one headless GhostVault on a tailnet node" deployments where `session` is too manual and `auto_unlock` is too blast-radius.

## Shape

Each API key is a random `apikey_<base62>` string. The raw string never sits on the server:

- `key_hash = sha256(raw_key)` identifies the row.
- `kdf_salt` + `raw_key` derive a per-key KEK via HKDF (or light Argon2id — see §Decisions).
- `wrapped_dek = ChaCha20-Poly1305-Seal(KEK, DEK)` stores the DEK for that key.

Every request: lookup by `sha256(bearer)`, re-derive KEK, unwrap DEK into request-local memory, serve, discard. `VaultState.DEK` stays empty in this mode.

## Schema (sketch)

```sql
CREATE TABLE api_keys (
    id           UUID PRIMARY KEY,
    vault_id     UUID NOT NULL REFERENCES vaults(id) ON DELETE CASCADE,
    key_hash     BYTEA NOT NULL UNIQUE,
    wrapped_dek  BYTEA NOT NULL,
    kdf_salt     BYTEA NOT NULL,
    kdf_params   JSONB NOT NULL,         -- alg, time, mem, par (Argon2id) or alg+info (HKDF)
    name         TEXT NOT NULL,
    scopes       TEXT[] NOT NULL DEFAULT ARRAY['retrieve','ingest'],
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ,
    expires_at   TIMESTAMPTZ
);
CREATE INDEX api_keys_vault_idx ON api_keys (vault_id);
```

## Mint / verify flow

```mermaid
sequenceDiagram
    participant Admin
    participant gvsvd
    participant DB

    Note over Admin,gvsvd: Admin holds a live session (or auto_unlock has populated VaultState.DEK)

    Admin->>gvsvd: POST /v1/api-keys (session bearer) {name, scopes, ttl}
    gvsvd->>gvsvd: raw_key = random(32); key_hash = sha256(raw_key)
    gvsvd->>gvsvd: KEK = derive(raw_key, salt); wrapped_dek = seal(KEK, VaultState.DEK)
    gvsvd->>DB: INSERT api_keys (key_hash, wrapped_dek, salt, …)
    gvsvd-->>Admin: {api_key: "apikey_…", id, expires_at}   -- shown exactly once

    Note over Admin,gvsvd: Later, any client uses the key

    Admin->>gvsvd: POST /v1/retrieve  (Authorization: Bearer apikey_…)
    gvsvd->>DB: SELECT by sha256(bearer)
    gvsvd->>gvsvd: KEK = derive(bearer, row.salt); DEK = open(row.wrapped_dek)
    gvsvd->>gvsvd: serve retrieve with DEK (discarded at end of request)
    gvsvd->>DB: UPDATE last_used_at = now()
```

## New endpoints

- `POST /v1/api-keys` — mint. Requires an active session (so the server has DEK to wrap).
- `GET /v1/api-keys` — list metadata (no raw keys, no wrapped DEK blobs).
- `DELETE /v1/api-keys/{id}` — revoke. Idempotent.
- `PATCH /v1/api-keys/{id}` — rename / extend TTL (optional; lowest priority).

`gvctl api-keys create|list|revoke` wraps these with the same shape as `gvctl actions-token`.

## DEK rotation

Rotation is the one flow this mode makes expensive. When an operator rotates the vault password (or rotates the DEK directly via a future admin endpoint):

1. Unwrap the current DEK with the old credential.
2. Generate `DEK' = new DEK`; re-seal chunk-level indexes (out of scope here).
3. For each active `api_keys` row: re-derive KEK from `(?, row.kdf_salt)` … **wait** — we can't, because we don't have the raw keys anymore.

So rotation in `api_keys` mode is one of:

- **Reissue:** invalidate every key, mint fresh ones, distribute. Simple but noisy.
- **Dual-wrap window:** store `wrapped_dek_old` + `wrapped_dek_new`; Resolve tries both; after TTL, drop `_old`. Avoids reissue but doubles storage per key.
- **Server-held rotation key:** cheat by keeping one server-held KEK that wraps DEK for rotation; defeats the "no server password" property.

Recommend starting with **Reissue** and adding a dual-wrap window only if users complain. Documented explicitly so clients know to script re-pairing.

## Key-derivation choice

Two viable paths for `raw_key → KEK`:

- **HKDF-SHA256** (recommended default). Fast, deterministic, zero extra DB work. Sufficient because `raw_key` is already 256 bits of uniform entropy — we don't need a slow KDF for brute-force resistance (attacker knows `key_hash` on a DB leak, not raw key material).
- **Argon2id (light params)**. Useful only if we ever want to allow user-chosen API keys ("vanity keys"), which is a UX trap. Skip.

## Integration with Part 2

`api_keys` slots in as a third `CredentialResolver` implementation. `requireVaultAuth` / `vaultInit` / handlers need **no changes** beyond:

- `vaultInit` dropping the `mode_not_implemented` rejection for `api_keys`.
- `cmd/gvsvd` wiring a `&apiKeyResolver{store, crypto}` when the loaded vault has `auth_mode=api_keys` instead of `&sessionResolver{…}`.

The returned `Credential` is of type `apiKeyCredential{ vaultID, dek, scopes, keyHash }` — same interface, populated per-request.

## Out-of-scope for this roadmap slot

- Hardware-backed keys (YubiKey / passkey). Nice future layer on top of `api_keys` but non-trivial on the MCP client side.
- Scoped keys (per-user-id, per-time-of-day). Easy to add once the rows exist but not required for v1.
- Cross-vault keys. Not worth the complexity; one vault per gvsvd process stays the rule.
