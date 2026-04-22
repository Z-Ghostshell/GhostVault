# Architecture (pivot: encrypted self-host + hybrid retrieval + MCP / Actions)

**Status:** Shipped scope and deferrals: **[`../OVERVIEW.md`](../OVERVIEW.md)**, **[`../ROADMAP.md`](../ROADMAP.md)**. This file mixes **current** service shape with **research** ideas; **on-chain** layers below are **not** v1 product unless **ROADMAP** says so.

The active design is an **open-source, self-hosted memory service** with:

- **Efficient hybrid retrieval** (dense + sparse + filters)—see [`RETRIEVAL.md`](./RETRIEVAL.md) and **[`../OVERVIEW.md`](../OVERVIEW.md)** for the implemented fusion.
- **Encrypted database** and **session-only plaintext** after user login—see [`CRYPTO-STORAGE.md`](./CRYPTO-STORAGE.md).
- **Integration**: **Claude Desktop** via **MCP**; **ChatGPT** via **Actions** (OpenAPI to your HTTPS service)—see [`PHASE1-SCOPE.md`](./PHASE1-SCOPE.md).
- **Optional future:** **on-chain** identity / commitments / key registry (not the corpus)—**deferred** per [`../OVERVIEW.md`](../OVERVIEW.md) *Deferred*; see also below and `CRYPTO-STORAGE.md`.

## Layer diagram (logical)

```
┌─────────────────────────────────────────────────────────────┐
│  Clients: Claude Desktop (MCP) · ChatGPT (Actions) · future   │
└───────────────────────────┬─────────────────────────────────┘
                            │ localhost HTTPS / Bearer · or tunnel+TLS
┌───────────────────────────▼─────────────────────────────────┐
│  Auth / session: unlock vault · short-lived tokens for GPT    │
└───────────────────────────┬─────────────────────────────────┘
                            │ Plaintext only inside process after unlock
┌───────────────────────────▼─────────────────────────────────┐
│  Memory core: chunk · embed · hybrid index · retrieve (budget) │
└───────────────────────────┬─────────────────────────────────┘
                            │ Encrypted at rest
┌───────────────────────────▼─────────────────────────────────┐
│  Encrypted DB + optional encrypted embedding tables on disk     │
└───────────────────────────┬─────────────────────────────────┘
                            │ Optional future sync / anchors
┌───────────────────────────▼─────────────────────────────────┐
│  Research / deferred: on-chain DID · registry · Merkle / policy  │
└─────────────────────────────────────────────────────────────┘
```

## Family: self-hosted trusted node (default for OSS)

- User (or org) runs the stack on **their** machine or VPS.
- **Storage admins** and **backup tapes** see **ciphertext** only; **application** sees plaintext **only after credential unlock**.
- **Pros**: Matches “encrypt everything in the DB” without impossible outsourced search.
- **Cons**: User must run, update, and back up the service; cold start may rehydrate indexes from ciphertext.

## On-chain roles (research / deferred)

If such a layer is ever added—not part of the current shipped vertical slice—possible uses:

| Use | Rationale |
|-----|-----------|
| **Identity / DID** | Portable id; link wallet or registered pubkey to vault |
| **Key registration** | “This key speaks for this vault” / rotation audit |
| **Commitments** | Merkle root of chunk manifest, policy hash |
| **Not** | Full RAG corpus, raw embeddings, or user plaintext |

**Product stance:** see [`../OVERVIEW.md`](../OVERVIEW.md) *Deferred*. If pursued later, pick one ecosystem and abstract with a thin adapter.

## Persona agent (later)

- Same memory API with **stricter** tool policies; vault scoping and **kill switch**.

## See also

- [`DATA-MODEL.md`](./DATA-MODEL.md) — encrypted row layout, chunk schema.
- [`THREAT-MODEL.md`](./THREAT-MODEL.md) — adversaries and inference leakage.
- [`../OVERVIEW.md`](../OVERVIEW.md#retrieval-algorithm-fusion-score-and-tuning-knobs) — hybrid retrieval and fusion as implemented in this service.
