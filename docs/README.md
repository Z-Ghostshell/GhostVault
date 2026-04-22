# Ghost Vault — design notes

**Ghost Vault** is **device-local** encrypted LLM memory: hybrid retrieval, unlock session, **OpenAPI-first** hosts (**ChatGPT** Actions, **Gemini** tools), **Claude** (MCP), and other connectors. Remote hosts see **only what you send per request**. Implementation language: **Go**.

## Where to read

**Integration how-tos** (ChatGPT, Gemini, Claude, MCP, shared OpenAPI HTTP setup): **[integration/](./integration/)**.

| Doc | Role |
|-----|------|
| [**OVERVIEW.md**](./OVERVIEW.md) | **Canonical product overview** — plaintext boundary, scope, honest limits, deferred items, relation to GitS (content maintained here; GitS links in). |
| [**deploy.md**](./deploy.md) | **Docker Compose**, **edge nginx**, **`make up MCP=…` `Dashboard=…`** (enable or disable), tunnels, **`gvsvd`** / **`gvmcp`**. |
| [**UNLOCK-AND-BEARER.md**](./UNLOCK-AND-BEARER.md) | **Init → unlock → session**, **`session_token` vs MCP Bearer**, token **security** and **exposure** (edge, plaintext vaults, restart). |
| [**ROADMAP.md**](./ROADMAP.md) | What we are building now vs deferring. |
| [**future/QUESTIONS.md**](./future/QUESTIONS.md) | Decisions recorded; remaining items explicitly **deferred**. |
| [**future/**](./future/) | Archived detail: architecture, crypto, data model, retrieval, threat model, phase-1 scope, integrations, vision narrative. |
| [**integration/OPENAPI.md**](./integration/OPENAPI.md) | **Step 1 (shared):** HTTPS, `openapi.yaml` `servers`, Bearer token, tunnels (ngrok, Tailscale Funnel, …). |
| [**integration/CHATGPT.md**](./integration/CHATGPT.md) | **Step 2 — ChatGPT:** Custom GPT → Actions (OpenAPI import), web / desktop / mobile. |
| [**integration/GEMINI.md**](./integration/GEMINI.md) | **Step 2 — Gemini:** API function calling / gateway (same REST contract as the spec). |
| [**integration/Claude-connector.md**](./integration/Claude-connector.md) | **Claude hosted connector + OAuth** (Auth0, **ZITADEL** / self-hosted OIDC): one guide — MCP vs connector, facade, verification, troubleshooting; **Claude iOS / Android** uses the same synced remote MCP (no local `gvmcp` on the phone). |
| [**integration/Claude-mcp.md**](./integration/Claude-mcp.md) · [**CLAUDE.md**](./integration/CLAUDE.md) | **Claude MCP:** full **`gvmcp`** reference (stdio, **`mcp-remote`**, hosted connector, OAuth env, Desktop, Cursor, troubleshooting). **CLAUDE** is the one-page orientation table + pointers. |
| [**integration/skills/ghostvault/SKILL.md**](./integration/skills/ghostvault/SKILL.md) | **Ghost Vault MCP skill** — `memory_search` / `memory_save` / `memory_stats` usage, **`meta`** on retrieve, session modes, host rules (canonical copy for Cursor / Claude). |
| [**integration/CURSOR.md**](./integration/CURSOR.md) | **Cursor:** MCP (`gvmcp`), project rules/skills, OpenAPI / raw HTTP when needed. |
| [**integration/skills/**](./integration/skills/) | **Agent skill** (`ghostvault/SKILL.md`) for **`gvctl`** + **`gvsvd`**; copy or install (see [README](./integration/skills/README.md)). |
| [**gis/docs/design.md**](../../gis/docs/design.md) | **GitS / Ghost engine** — multi-agent framework; Ghost Vault is complementary (short pointer + concept index row). |

**Product facts** live in **`OVERVIEW.md`** and **`ROADMAP.md`**; avoid duplicating them elsewhere—**link** instead. If two docs disagree, prefer **`OVERVIEW.md`**, then newer **`integration/*`**, then add a status banner in **`future/*`** rather than a third copy.
