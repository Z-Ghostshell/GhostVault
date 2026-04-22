# Threat model

**Status:** Shipped confidentiality story and deferrals: **[`../OVERVIEW.md`](../OVERVIEW.md)**, **[`../ROADMAP.md`](../ROADMAP.md)**. This file states **goals** and **adversaries**; **on-chain** mentions are **optional future** research unless **ROADMAP** commits to them.

## Active design (aligned with pivot)

- **Self-hosted** memory service with **encrypted database at rest**; **plaintext only** in process **after** user unlock (credential/session). Operators who only have disk/backups see **ciphertext**. Details: [`CRYPTO-STORAGE.md`](./CRYPTO-STORAGE.md).
- **Optional future:** on-chain anchors for **identity / commitments** (not the corpus)—**deferred** per [`../OVERVIEW.md`](../OVERVIEW.md); see same doc for ciphertext vs operator access.

## Assets to protect

- **Full memory corpus** (documents, notes, embeddings, graph): highest value; must remain **confidential** and **non-copyable** as a whole by unrelated parties.
- **Cryptographic keys** (encryption, signing): compromise = full read/write as the user.
- **Metadata** (timestamps, sizes, access patterns): may leak **usage** even if ciphertext is secure.

## Adversaries (examples)

| Adversary | Capability | Typical goal |
|-----------|------------|--------------|
| **LLM provider** | Sees prompts and context for sessions using their API | Learn as much as possible from traffic; cannot be assumed to forget |
| **Malicious or breached plugin host** | Runs connector code; may exfiltrate | Steal tokens, plaintext snippets, or keys if mishandled |
| **Other users / apps** | No direct access to your store | Should learn nothing without authorization |
| **Storage provider** | Holds ciphertext blobs | Should not decrypt without keys |
| **Network attacker** | TLS break, MITM on bad configs | Read/modify traffic |

## What “cannot replicate the entire DB” can mean

1. **Strong interpretation**: Attackers with **only** ciphertext (and, if used in a future design, **public** non-sensitive chain data) **cannot** recover or clone the full logical database without the user’s keys (standard **confidentiality** goal).

2. **Operational interpretation**: No **single** third-party service (including your own memory relay) holds **both** full ciphertext **and** keys; or keys are **only** on user devices / HSM / user-controlled enclave.

3. **Weaker (avoid if possible)**: “Vendor doesn’t store our DB” but the vendor **still sees every retrieved snippet** each turn—**session logging** can approximate a large fraction of memory over time.

**Design implication**: Be explicit which interpretation you ship. **Inference leakage** to the active LLM host is **unavoidable for whatever you put in the prompt** that turn.

## Security properties to aim for

- **Encryption at rest** for all memory blobs; **keys** derived from user-controlled material (password + KDF, hardware key, or MPC split—TBD).
- **Authenticated encryption** and **integrity** for sync blobs (object store, or **future** on-chain manifests if ever used).
- **Minimal trust** in the memory service: ideally **zero-knowledge** to plaintext for hosted relays, or **self-hosted** / **local-first**.
- **On-chain content**: only **non-sensitive** or **hashed** data unless you deliberately publish ciphertext to a content-addressed layer with separate key management.

## Out of scope without extra mechanisms

- **Hiding retrieved text from the LLM** you are asking to answer—you must either **not send** it or accept **that host** can log it for that session.
