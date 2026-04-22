# Data model

**Status:** The **physical** schema (Postgres tables, `tsvector`, `chunk_embeddings`, defaults like **`text-embedding-3-small`**) is in **[`../OVERVIEW.md`](../OVERVIEW.md)**. This note is **conceptual** uses of memory, encryption/index **options**, and **research** (e.g. on-chain) that may **not** match the shipped v1 row layout.

## Memory kinds (conceptual)

| Kind | Examples | Typical retrieval |
|------|----------|-------------------|
| **Episodic** | Dated events, conversations, decisions | Time + similarity |
| **Semantic** | Stable facts, preferences, entities | Similarity + graph |
| **Procedural** | How the user likes tasks done, templates | Tag + similarity |
| **Style / voice** | Tone, vocabulary, taboo topics | Small curated slice for prompt |

**Shipped mapping:** the API exposes optional **`item_kind`** (string) and **`metadata` (JSON)** per chunk, plus a logical **abstract** and **body** in the v2 on-disk/encrypted payload. Optional **`source_documents`** rows hold a shared full document; **`memory_chunks.source_document_id`** links slices when per-chunk body is omitted. Dense vectors index the abstract when it is set; `search_tsv` uses abstract plus an excerpt of the shared source when linked. See [OVERVIEW.md](../OVERVIEW.md) for `source_document` ingest, `content_mode` on retrieve, and two-stage (rank → `GET` full) use.

## Encrypted storage (durable)

Each **chunk** is one logical unit of memory. The **concrete** columns (`ciphertext` vs dev `body_text`, `item_kind`, `item_metadata`, `search_tsv`, `content_sha256`, vector row in `chunk_embeddings`, etc.) are documented in [`../OVERVIEW.md`](../OVERVIEW.md). At a high level:

- **Body**: encrypted (AES-GCM / ChaCha20-Poly1305 style) with **nonce** and **`key_id`** for DEK rotation, or dev plaintext mode—never both.
- **Identity / scope**: `vault_id` + `user_id` namespace; stable **chunk id** for citations.
- **Embeddings:** stored in-process / pgvector in the shipped design; design alternatives include **encrypted vector columns** or **rebuild vectors after unlock** (slower cold start, smaller at-rest surface). See `CRYPTO-STORAGE.md`.

**Sparse index:** tokenized / BM25 / `tsvector` structures live **inside the trusted process** (or encrypted sidecar files in alternate designs); not exposed to off-device operators as plaintext bulk.

## Raw ingest path

1. Plaintext at ingest (only after unlock) → chunk with overlap.
2. Encrypt chunk → persist row.
3. Update **dense** and **sparse** indexes in memory (and persist encrypted snapshots if used).

## Indexing and search

- **Hybrid**: dense (HNSW/IVF) + sparse (BM25) + metadata filters; shipped fusion in [`../OVERVIEW.md`](../OVERVIEW.md), alternates in [`RETRIEVAL.md`](./RETRIEVAL.md).
- **No** plaintext bulk export to object storage without user-initiated export and explicit key.

## Embeddings

- Version **`embedding_model_id`** per chunk or per index generation.
- **Rotation**: new model → background re-embed (vault unlocked); keep old vectors until cutover.

## Metadata (minimize plaintext)

- Prefer **encrypted** metadata bag alongside chunk ciphertext.
- **On-chain**: only hashes / pubkeys / policy commitments—see `CRYPTO-STORAGE.md`.

## Deletion

- **Tombstone** row + remove from indexes; periodic **secure delete** of ciphertext blobs.
- **Merkle root** update on chain if you publish manifest commitments.

## Export / portability

- **Encrypted archive**: ciphertext + manifest + **`key_id`** map; user holds recovery key.
- Optional **plaintext export** (opt-in) for user backup to JSONL.

## See also

- [`../OVERVIEW.md`](../OVERVIEW.md) — physical schema; v1 uses chunk rows + `content_sha256` dedup. An **entity** side-index (separate from chunk rows) remains **optional future** research.
