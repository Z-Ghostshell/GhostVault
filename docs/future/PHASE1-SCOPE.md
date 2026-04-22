# Phase 1 — Scope (pivot): encrypted memory DB + hybrid retrieval + MCP + ChatGPT Actions

**Status:** Shipped product scope and deferrals are **[`../OVERVIEW.md`](../OVERVIEW.md)** and **[`../ROADMAP.md`](../ROADMAP.md)**. This note is a historical “pivot” slice: it may describe **research** or **later** directions (e.g. on-chain identity). Treat **on-chain / testnet** items below as **not** committed for the current vertical slice unless **ROADMAP** pulls them forward.

## Product intent

Build a **very efficient**, **self-hosted**, **open-source** memory database with:

- **Embedding retrieval** plus **other retrieval** (lexical / BM25, metadata filters, optional rerank)—see [`RETRIEVAL.md`](./RETRIEVAL.md).
- **Integration**: **Claude Desktop** via **MCP**; **ChatGPT** via **Actions** (OpenAPI → HTTPS). Same core HTTP API for both patterns.
- **Security**: **All user memory encrypted in the database**; hosts and storage operators **must not** be able to read memory without the user’s **credentials**. **Plaintext exists in the service only** after **login/unlock**, and when **explicitly** returned to the LLM or client during use (same as any RAG). *(You wrote “Firmalow”—we treat that as **plaintext** in session.)*
- **On-chain identity / commitments** (research / later): possible future use for locking, DID, or manifest hashes—**not** for storing the corpus. **Not** in the first vertical slice for shipped Ghost Vault; see *Deferred* in [`../OVERVIEW.md`](../OVERVIEW.md). [`CRYPTO-STORAGE.md`](./CRYPTO-STORAGE.md) for ciphertext story.

## In scope

| Area | Notes |
|------|--------|
| **Memory engine** | Chunking, pluggable embedder, hybrid retrieve with **token/chunk budget**. |
| **Encrypted persistence** | Envelope encryption, per-vault DEKs, Argon2id (or similar) from password; session unlock. |
| **HTTP API** | OpenAPI 3: `health`, `login`/`unlock`, `logout`, `ingest`, `retrieve` (exact paths TBD). |
| **MCP server** | Tools wrapping the API for **Claude Desktop** (localhost). |
| **ChatGPT** | Custom GPT **Actions** importing OpenAPI; tunnel/VPS + **short-lived** or scoped tokens. |
| **Deploy** | Docker Compose; documented threat model for self-host. |
| **On-chain (research)** | DID / pubkey registry / Merkle manifest hash on a testnet was an **optional experiment** in this old slice—not a current v1 deliverable; see [`../OVERVIEW.md`](../OVERVIEW.md) *Deferred*. |

## Out of scope (Phase 1)

- Full **consumer** Gemini in-browser product integration (use **Gemini API + functions** as a follow-on if desired).
- **Persona / digital twin** agent beyond memory retrieval.
- **Outsourced** indexing where the cloud computes embeddings **without** trust (requires different crypto research stack).

## Architecture pattern

```
                    ┌──────────────────────────────┐
                    │  Memory service (OSS)         │
                    │  encrypted DB · hybrid search │
                    │  unlock → plaintext in RAM    │
                    └──────────────┬───────────────┘
           ┌──────────────────────┼──────────────────────┐
           ▼                      ▼                      ▼
   ┌───────────────┐      ┌───────────────┐      ┌───────────────┐
   │ Claude        │      │ ChatGPT       │      │ Future:       │
   │ MCP → local   │      │ Actions →     │      │ on-chain ID   │
   │ HTTP          │      │ HTTPS+tunnel  │      │ (deferred)    │
   └───────────────┘      └───────────────┘      └───────────────┘
```

## Definition of done

1. Single repo runs via **Docker Compose**; docs for **threat model** and **unlock** flow.  
2. **Retrieve** supports hybrid search with documented **latency** and **budget** behavior.  
3. **At-rest encryption** verified: DB dump unreadable without derived keys; tests with fixture vault.  
4. **MCP** documented for Claude Desktop.  
5. **OpenAPI** + **ChatGPT Actions** checklist (HTTPS, auth).  
6. *(Research / not v1 deliverable)* Optional chain flow (e.g. manifest hash on testnet) if revisited after core ship—see [`../OVERVIEW.md`](../OVERVIEW.md) *Deferred*.

## Risks (tracked)

| Risk | Mitigation |
|------|------------|
| Remote LLM **sees** retrieved snippets | Document; minimize per turn; user-visible “what was sent” where possible. |
| **ChatGPT** needs public URL | Tunnel + TLS + scoped tokens. |
| Cold start **slow** if vectors rebuilt from ciphertext | Offer **encrypted embedding cache** optional mode. |

## Next

- Implementation ADRs: embed backend, SQL vs embedded Lance, exact crypto primitives (libsodium vs RustCrypto).  
- [`ROADMAP.md`](../ROADMAP.md) for later phases (persona agent, extensions).
