# Claude integration

Ghost Vault talks to Claude through **`gvmcp`** (MCP tools **`memory_search`**, **`memory_save`**, **`memory_stats`** → **`gvsvd`**). For **orchestrated (agentic) tool use** and host prompts, see the **[ghostvault skill](./skills/ghostvault/SKILL.md)**. There are **two** main ways to wire the connector — **not** interchangeable (who runs the client, network path, where you configure, secrets). **Full comparison table:** [Claude-connector.md — § 1](Claude-connector.md#1-two-ways-into-claude-not-interchangeable).

**Full guide (hosted connector + OAuth + IdP + troubleshooting):** **[Claude-connector.md](./Claude-connector.md)**. **OAuth / Auth0 / ZITADEL recipes:** [oauth.md](./oauth.md). **All MCP modes, env vars, Desktop JSON:** [Claude-mcp.md](./Claude-mcp.md) (Desktop, hosted connector, OAuth).

---

## Claude Code and gvctl

Use a project or personal agent skill: **[skills/README.md](./skills/README.md)**.

---

## See also

- [Claude-connector.md](./Claude-connector.md) — **primary doc** for claude.ai + connector + OAuth  
- [skills/ghostvault/SKILL.md](./skills/ghostvault/SKILL.md) — tool timing, `meta` on retrieve, session modes, host rules  
- [deploy.md](../deploy.md) — Compose, edge, tunnels, Funnel vs Serve  
- [OPENAPI.md](./OPENAPI.md) — Bearer token and REST (same vault as MCP)  
- [Anthropic — Connectors](https://support.claude.com/en/articles/11176164-use-connectors-to-extend-claude-s-capabilities)
