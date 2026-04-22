# Ghost Vault — roadmap

This roadmap implements [**OVERVIEW.md**](./OVERVIEW.md). **GitS** context: [**gis/docs/design.md**](../../gis/docs/design.md).

## Current focus — ship the integration surface

**Goal:** one **HTTP/OpenAPI** Ghost Vault service on the device, with **remote LLM hosts** seeing only request text (query + snippets), not the full vault. Implement these three tracks together:

| Track | What to ship |
|--------|----------------|
| **OpenAPI integration** | Shared **step 1:** [**integration/OPENAPI.md**](./integration/OPENAPI.md). **Step 2:** [**integration/CHATGPT.md**](./integration/CHATGPT.md) (Actions) or [**integration/GEMINI.md**](./integration/GEMINI.md) (function calling). |
| **MCP server** | Bridge **Claude Desktop** and other MCP hosts to the **same** API. [**integration/Claude-mcp.md**](./integration/Claude-mcp.md). |
| **Dashboard** | Local UI for activity, session, and operator workflows alongside the service. |

Supporting work (encrypted store, unlock/session, hybrid retrieval, deploy/tunnel docs) continues in service of that slice; detail lives in [README.md](./README.md) and [future/](./future/).

## Later (unchanged intent)

**Mobile/browser** surfaces for the same hosts, **extension/sidecar** when custody UX needs it, **backup/restore** of encrypted blobs, key rotation and runbooks — after the integration surface above is solid.

**Deferred:** on-chain identity, persona/twin agent, outsourced trust-free indexing, launch/legal/revenue — see [OVERVIEW.md](./OVERVIEW.md) *Deferred*.

## Continuous

Security review, dependency updates, honest claims (ciphertext on device vs what each LLM host sees per request).
