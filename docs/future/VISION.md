# Vision

## One-line pitch

A **user-owned memory database** for LLM workflows: encrypted, portable across applications, **anchored on-chain** for identity and persistence commitments, with a path toward an **agent that reflects the user** after long-term use.

**Soul–mind frame:** the **mind** is the held, encrypted memory substrate; the **soul** is identity, key control, and on-chain anchors for *who* owns that substrate. They are **synergistic**—ownership and continuity without claiming the full vault is public or that providers see nothing. Product name: **Ghost Vault**; overview: [`OVERVIEW.md`](../OVERVIEW.md) · GitS: [`gis/docs/design.md`](../../../gis/docs/design.md).

## User story

1. **Sign-up**: User creates a wallet or identity; **key material and/or commitments** are registered on a **public blockchain** they choose (L1/L2). The full memory corpus is **not** stored on-chain at bulk scale; the chain holds what is needed for **trust-minimized identity, policy, and snapshot roots**.

2. **Daily use**: The user connects **ChatGPT, Claude, Gemini, or other tools** via plugins, MCP, or local bridges. Each session **retrieves** relevant memory (semantic + episodic) and injects **only what is needed** into the prompt context.

3. **Over time**: Memory accumulates—preferences, facts, style, projects—becoming the **“sunlight”** that nourishes a consistent model of the user across surfaces.

4. **End state**: A dedicated **persona agent** (or twin) uses this memory as ground truth for behavior, tone, and long-horizon consistency, under **explicit user control**.

## Goals

- **Single logical memory** per user, **multi-surface** access.
- **Strong privacy posture**: third parties (including LLM hosts) must not be able to **bulk-copy** or **mirror** the full memory store; only **ciphertext** and **minimal metadata** are acceptable outside user-controlled keys.
- **Durability**: survive device loss via **encrypted backup** and **chain-anchored** recovery or verification flows.
- **User agency**: export, delete, redact, and **rotate keys**.

## Non-goals (for v1)

- Proving that **no plaintext** ever touches a vendor when the user sends a message to ChatGPT/Claude/Gemini—**inference-time context is visible to that provider for that request** if it is included in the prompt. The design targets **full-database non-replication**, not “provider sees nothing.”
- Storing **large embedding indexes** entirely on-chain (cost and throughput).
- Legal/ethical “replication of a person” beyond **product and consent** framing—those need separate policy docs if you ship broadly.

## Terminology

| Term | Meaning |
|------|--------|
| **Mind** | The private, persistent memory substrate (encrypted store + continuity under user keys)—not identical to any single chat transcript. |
| **Soul** | Identity and agency under user keys: who the vault belongs to, wallet/DID linkage, rotations, and on-chain commitments—distinct from the bulk memory corpus yet inseparable from it in the product story. |
| **Memory substrate** | Encrypted store + indexing + retrieval API under user keys. |
| **Anchor** | On-chain record: DID, public key, hash of policy or state root, etc. |
| **Persona agent** | Downstream system trained or conditioned on memory + explicit rules. |
