# Cryptography, encrypted database, and blockchain “lock”

This document captures the **pivot**: memory is **encrypted at rest**; **operators of storage** (disk, backup, misconfigured S3) must not be able to read user memory. **Plaintext exists only** (a) **after the user authenticates** in the self-hosted service, and (b) **when explicitly sent** to an external LLM or tool during a session—those providers then see **that** payload for that request.

## Threat model (summary)

| Party | Should see |
|-------|------------|
| **Disk / backup / host admin without creds** | Ciphertext + non-sensitive config only |
| **Self-hosted app process** | Plaintext **only while vault unlocked** (session) |
| **ChatGPT / Claude / etc.** | Only **retrieved snippets** you send in that turn |
| **Blockchain** | **Public commitments / keys you choose to publish**—never raw memory |

## Key hierarchy (recommended pattern)

1. **User password** (or passphrase) → **Argon2id** (or scrypt) → **Key-encryption key (KEK)** (never stored raw).
2. **Data-encryption keys (DEKs)** per vault / user / rotation epoch—random; stored on disk **wrapped** by KEK (AES-GCM or ChaCha20-Poly1305 envelope).
3. **Optional hardware / OS**: OS keychain (DPAPI, Keychain) stores a **device key** that wraps KEK for “remember this device” without retyping password every boot—trade convenience vs security.

**Database contents:**

- Rows: **ciphertext** + **nonce** + **key id** (which DEK) + integrity tag.
- **No** plaintext columns for body text or embeddings in the durable store.

## Session / unlock model

- On **login**, derive KEK, unwrap DEKs, load **decrypted index structures into memory** (or mmap encrypted files and decrypt pages on access—implementation detail).
- On **logout** or **timeout**: **zeroize** keys and sensitive buffers; drop in-memory indexes; optionally restart process for defense in depth.
- **Background jobs** (embedding refresh) run **only** while vault is unlocked or via scheduled unlock (user choice).

This matches: “plaintext **only** when you log in with your credential, and when talking to other services” (the latter meaning you **choose** to send retrieved text to MCP/Actions-connected models).

## Embeddings and search (honest constraint)

- **Embeddings require plaintext** (or a deterministic transform of plaintext) at compute time. So: compute **on the self-hosted node after unlock**; persist **encrypted embedding tables** if you want warm restarts without re-embedding everything, or **rebuild vectors from ciphertext** on unlock (slower startup, smaller trust surface on disk).
- **BM25 / inverted index**: same story—built **inside** the trusted process after decrypting chunks, or stored encrypted.

There is **no** magic where untrusted cloud storage both **indexes** and **never sees** plaintext unless you use **client-only** indexing (different product shape). **Self-hosted + unlock** is the practical OSS pattern.

## Blockchain role (“locking” / secure identity)

The chain does **not** store the memory database. Use it for **identity and policy anchors**:

| On-chain (examples) | Off-chain |
|---------------------|----------|
| **DID document** or compact **registry**: signing pubkey for “this user’s vault” | Ciphertext DB + encrypted backups |
| **Hash** of **policy** or **vault manifest** (Merkle root of chunk ids) | Full manifest in encrypted backup |
| **Key rotation events** (publish new wrapping pubkey) | Wrapped DEKs in DB |

**Wallet-based login** (optional): user proves control of an address linked to registered pubkey; session still uses **symmetric** crypto for bulk data (hybrid design).

**Smart contract “lock”** (advanced): time-locks, multisig for recovery, or attestation hooks—not required for v1.

## MCP / Actions security

- **MCP** (Claude Desktop): memory daemon listens on **localhost**; tools send **Bearer** or **mTLS** to the service; vault must be **unlocked** for retrieve/ingest.
- **ChatGPT Actions**: HTTPS to your tunnel/VPS; use **short-lived tokens** + **TLS**; never embed long-lived password in the Custom GPT—use OAuth device flow or API keys scoped to **retrieve only** if possible.

## Operational checklist

- [ ] Envelope encryption for all user payload rows  
- [ ] No secrets in logs; redact query text in production logs by default  
- [ ] Secure backup = **encrypted blob** + separate **offline** recovery key (paper or hardware)  
- [ ] Document **what** is visible to OpenAI/Anthropic when a tool fires  
