# Cursor integration (Ghost Vault)

**Cursor** can use Ghost Vault the same way other **MCP hosts** do: run **`gvmcp`** (this repo) so the agent gets **`memory_search`**, **`memory_save`**, and **`memory_stats`** tools that call **`POST /v1/retrieve`**, **`POST /v1/ingest`**, and **`GET /v1/stats`** on **`gvsvd`**. For **project rules and agentic (multi-step) usage**, see the **[ghostvault skill](./skills/ghostvault/SKILL.md)**. Cursor does **not** import **`openapi/openapi.yaml`** as first-class “GPT Actions” style tools; for in-editor agents, **MCP** is the supported path.

For **unlock, tokens, tunnels, and REST details**, see [OPENAPI.md](./OPENAPI.md) and the full MCP walkthrough in [Claude-mcp.md](./Claude-mcp.md). This page focuses on **Cursor-specific** wiring: **MCP**, **project rules and skills**, and the **OpenAPI** contract when you need the raw HTTP surface.

---

## 1. MCP with `gvmcp` (recommended)

**When:** You want the Cursor agent to call Ghost Vault with discoverable tools.

**What to do:**

1. Run **`gvsvd`** somewhere reachable from **`gvmcp`** and set **your Ghost Vault server URL** as **`GHOSTVAULT_BASE_URL`** (e.g. `make up` — edge is often **`http://127.0.0.1:8989/api`** — or a tailnet/tunnel origin; [deploy.md](../deploy.md)).
2. Build **`gvmcp`**: `make mcp` → **`bin/gvmcp`**.
3. Unlock and set auth the same way as other integrations ([Get a Bearer token](./OPENAPI.md#get-a-bearer-token-ghost-vault)). Prefer a **token file** or env the MCP process can read so you are not pasting secrets into chat.
4. In **Cursor**, add an MCP server whose **command** is the absolute path to **`gvmcp`**, with **`env`** (or flags) matching [Claude-mcp.md — Minimum steps](./Claude-mcp.md#minimum-steps-mcp-host).

Use [`configs/mcp.example.json`](../../configs/mcp.example.json) as the template (**`mcpServers`** only). Copy it to your repo’s **`.cursor/mcp.json`**, or merge the **`ghostvault`** entry via **Cursor Settings → MCP** — then replace absolute paths and placeholders (`command`, **`args`**, **`GHOSTVAULT_BASE_URL`**, **`GHOSTVAULT_TOKEN_FILE`** or **`GHOSTVAULT_BEARER_TOKEN`**). **Claude Desktop** uses the same shape: merge **`mcpServers`** into your full config (legacy files may also have a top-level **`preferences`** object).

**After changing tokens** (e.g. **`gvsvd` restart**), update the MCP server env and **reload** MCP in Cursor so the new session is picked up — same caveats as [Claude-mcp.md — Limits](./Claude-mcp.md#limits-to-keep-in-mind).

**stdio vs HTTP:** For daily local use, **stdio** (default `gvmcp` with no listen flag) is simplest. If you ever need a **remote** MCP URL, run **`gvmcp -listen …`** and put HTTPS in front; see [Claude-mcp.md — Local vs remote](./Claude-mcp.md#local-stdio-vs-remote-http--tunnel).

---

## 2. Project rules and skills

**When:** You want consistent **instructions** for the model (vault IDs, base URL, unlock flow, where to read) alongside or **instead of** MCP.

- **With MCP:** Rules or skills can point at [skills/ghostvault/SKILL.md](./skills/ghostvault/SKILL.md), [OPENAPI.md](./OPENAPI.md), and [Claude-mcp.md](./Claude-mcp.md) so contributors use the same env, token hygiene, and tool-budget copy.
- **Agent skill (shell):** copy **`docs/integration/skills/ghostvault/`** or use **`scripts/install-ghostvault-skill-cursor.sh`** — see **[skills/README.md](./skills/README.md)**.

Rules and skills do **not** implement the API; they steer **how** the agent uses shell or MCP against **`gvsvd`**.

---

## 3. OpenAPI (raw HTTP)

**When:** You need operations **`gvmcp`** does not expose yet, or you are implementing against the documented REST contract.

Use [`openapi/openapi.yaml`](../../openapi/openapi.yaml) as the source of truth: same **Bearer** auth and request bodies as [OPENAPI.md](./OPENAPI.md). You can also experiment with a **third-party OpenAPI → MCP** bridge; **`gvmcp`** remains the **first-party** supported server ([Claude-mcp.md — providers summary](./Claude-mcp.md#providers-and-clients-summary)).

---

## See also

- [Claude-mcp.md](./Claude-mcp.md) — **`gvmcp`**, transports, troubleshooting  
- [skills/ghostvault/SKILL.md](./skills/ghostvault/SKILL.md) — agentic tool use, `meta` on retrieve, **Cursor**-friendly host rules  
- [OPENAPI.md](./OPENAPI.md) — HTTPS, Bearer token, tunnels  
- [skills/README.md](./skills/README.md) — **`ghostvault`** agent skill (`gvctl`)  
- [CLAUDE.md](./CLAUDE.md) — Claude entry (same MCP binary pattern)
