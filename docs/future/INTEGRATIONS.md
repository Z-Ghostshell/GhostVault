# Integrations

**Status:** Shipped scope: **[`../OVERVIEW.md`](../OVERVIEW.md)**, **[`../ROADMAP.md`](../ROADMAP.md)**. A historical “phase 1” slice (memory + **MCP** + **ChatGPT Actions**) lives in [**PHASE1-SCOPE.md**](./PHASE1-SCOPE.md); **on-chain identity** is **not** a v1 commitment. [**CRYPTO-STORAGE.md**](./CRYPTO-STORAGE.md) for encryption. This file is general reference for integration patterns.

## Reality check

Any integration that **sends context to a remote LLM** exposes **that** content to **that** provider for **that** request (logging, safety, training policies vary—user should read provider terms).

Design for: **minimize** what is retrieved (top-k, budgets), **redact** secrets, **user-visible** “what memory was used” where possible.

## Patterns

| Pattern | Fits | Notes |
|---------|------|--------|
| **MCP server** | Claude Desktop, growing ecosystem | Memory as tools/resources; local process can hold keys |
| **Browser extension** | Web UIs of many hosts | Injects or sidecars context; user must trust extension |
| **Desktop companion** | Cross-app | Local memory core; paste or automate injection |
| **Vendor “plugins” / Actions** | ChatGPT, etc. | OAuth to **your** backend; backend must **not** hold long-term plaintext of full DB if you want strongest story—often **tradeoff** vs UX |
| **Mobile app** | Same as desktop with OS keychain | Background sync constraints |

## Suggested API surface (for your memory service)

- `POST /v1/retrieve` — query + budget (max tokens / max chunks); returns **ranked snippets** + **citations** (internal ids).
- `POST /v1/ingest` — new text or structured events; **async** embed/index.
- `GET /v1/policy` — what categories exist, retention, redaction rules.
- **Auth**: OAuth2 / DPoP / signed requests; **optional future** binding to on-chain or wallet identity if productized (see [`../OVERVIEW.md`](../OVERVIEW.md) *Deferred*).

## Token budget

- Every host has **context limits**; retrieval must be **budget-aware** and **deduplicate** across turns.

## Rate limits and abuse

- Hosted gateway needs **per-user quotas**; **audit** without storing full prompts if possible (hashes only).
