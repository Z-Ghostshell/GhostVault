# Retrieval: efficient memory database

**Status:** The **shipped** fusion pipeline, parameters, and equations live in **[`../OVERVIEW.md`](../OVERVIEW.md#retrieval-algorithm-fusion-score-and-tuning-knobs)**. This file: ingest/index **design** and **research** alternates.

Goal: a **self-hosted** memory engine that stays fast as volume grows, combining **semantic** (embedding) retrieval with **lexical** and **structured** signals—without assuming plaintext is visible to backup or object-store operators (see `CRYPTO-STORAGE.md` for at-rest encryption).

## Design principles

1. **Hybrid retrieval** beats pure vector search for names, IDs, rare tokens, and “exact phrase” queries.
2. **Budget-aware** output: every `retrieve` call takes `max_tokens` / `top_k` caps so MCP and Actions stay predictable.
3. **Version everything**: embedding model id, chunk schema version, index build id—so re-embedding does not corrupt history silently.

## Pipeline (ingest)

```
Raw text → chunk (overlap) → optional enrich (title, tags) → embed → persist ciphertext + index rows
```

- **Chunking**: sliding window with overlap (e.g. 512–1024 tokens, 10–15% overlap); split on paragraph boundaries when possible.
- **Deduplication**: hash normalized chunk text before encrypt; skip or merge duplicates per user namespace.

## Index types (all referencing **opaque row ids**, not plaintext in backup)

| Layer | Role | Typical implementation |
|-------|------|-------------------------|
| **Dense** | Semantic similarity | HNSW or IVF-PQ over **in-process** vectors (see crypto note below) |
| **Sparse** | Keywords, names, SKUs | BM25 / inverted index over **tokenized** text **inside the trust boundary** after decrypt, or over a **private** sidecar index on the self-hosted node |
| **Metadata** | Time range, source, labels | Narrow filters **before** expensive vector search |

**Crypto note:** For “storage cannot read memory,” **embeddings cannot be computed on untrusted storage**. The self-hosted service, **after unlock**, computes embeddings and holds vectors **in RAM or local encrypted volume** for the session; on disk, store **ciphertext blobs** for chunks; vectors may be stored encrypted at rest or rebuilt after login (tradeoff: cold-start vs disk size). See `CRYPTO-STORAGE.md`.

## Retrieval fusion

### (A) Implemented in Ghost Vault (canonical)

**See [`../OVERVIEW.md`](../OVERVIEW.md#retrieval-algorithm-fusion-score-and-tuning-knobs):** **linear weighted fusion** of semantic and normalized lexical scores, with a **semantic floor** (threshold) on dense candidates, parallel dense + sparse candidate pulls, over-fetch, then **pack** to `top_k` / token budget. This path is **not** reciprocal rank fusion (RRF) in code today.

### Alternates and research

- **Reciprocal rank fusion (RRF)** or other merge strategies vs the shipped weighted fusion—useful to benchmark later.
- **Cross-encoder rerank** on top 20–50 candidates (CPU-heavy; feature-flag).
- **Entity side-index** / multi-phase retrieve pipelines (optional research): richer **entity** linking and second-stage rank beyond chunk-only fusion.

**Typical research pipeline shape** (for comparison, not the only possible future):

1. **Retrieve candidates**: top `k_dense` from vector index + top `k_sparse` from sparse channel (if enabled).
2. **Merge**: e.g. RRF or tunable `α` weighted blend—*contrast with (A) above*.
3. **Filter**: metadata (date, source).
4. **Pack**: greedy fill by relevance until `max_tokens` for LLM context.

## Efficiency tactics

- **Quantization** (PQ / SQ) for large corpora if recall remains acceptable.
- **Sharding** by user id + time partition for very large libraries.
- **Caching**: short-TTL cache of recent `retrieve` keyed by `(user, query_hash, budget)`—**only** after auth; clear on logout.

## API shape (logical)

- `POST /v1/retrieve` — `{ "query", "max_tokens", "top_k", "filters": {} }`
- `POST /v1/ingest` — `{ "text", "metadata" }`
- Optional: `POST /v1/hybrid_config` (admin) — weights, enable/disable sparse index.

Keep OpenAPI stable; retrieval internals evolve behind the same endpoints.

## See also

- [`DATA-MODEL.md`](./DATA-MODEL.md) — chunk schema, embedding versioning.
- [`CRYPTO-STORAGE.md`](./CRYPTO-STORAGE.md) — where indexes may live relative to ciphertext.
- [`../OVERVIEW.md`](../OVERVIEW.md#retrieval-algorithm-fusion-score-and-tuning-knobs) — **Shipped** hybrid fusion and equations.
