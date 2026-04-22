# Example MCP connector configs

Copy one of the JSON files into your client’s MCP settings (Claude Desktop, Cursor, etc.) and replace the placeholders. Full wiring details: [docs/integration/Claude-mcp.md](../docs/integration/Claude-mcp.md).

## Edge routes (reminder)

On the default Docker **edge** (`http://127.0.0.1:8989` by default), paths are split as in the main [README](../README.md): **`/api/`** → **`gvsvd`** (REST), **`/mcp/`** → **`gvmcp`** (streamable HTTP MCP). **`mcp-remote`** must target the **MCP** URL (`…/mcp/`), not **`/api/`**.

## `mcp.develop.example.json` — local **`gvmcp`** (stdio)

The client runs the **`gvmcp`** binary and talks to it over **stdio**. No **`npx`** / **`mcp-remote`**.

Use this when you are **developing** Ghost Vault or running **`gvsvd`** where you can point **`GHOSTVAULT_BASE_URL`** at your API base (for example Compose or `go run` on the same machine). Set **`GHOSTVAULT_TOKEN_FILE`** (or bearer token) after **`gvctl unlock`**, and paste vault/user defaults from **`gvctl unlock -vault-id-only`** as needed.

## `mcp.local.example.json` and `mcp.remote.example.json` — **`mcp-remote`** (stdio → HTTPS)

Both use **[`mcp-remote`](https://github.com/geelen/mcp-remote)** so clients that only support **stdio** MCP can reach **`gvmcp`** where it is already exposed as **streamable HTTP MCP** behind **HTTPS** (edge **`/mcp/`**, Tailscale Serve/Funnel, tunnel, etc.).

| File | Use when |
|------|----------|
| **`mcp.local.example.json`** | HTTPS URL uses your **tailnet** hostname (e.g. **`https://your-mac.your-tailnet.ts.net/mcp/`** after Serve/Funnel to **`127.0.0.1:8989`**). |
| **`mcp.remote.example.json`** | HTTPS URL is **any other** reachable host (public tunnel, ngrok, another machine, etc.). |

Replace in the JSON:

- **`command`**: absolute path to **`npx`** — **`/opt/homebrew/bin/npx`** (Apple Silicon) or **`/usr/local/bin/npx`** (Intel Homebrew). Do not use the **mise** / **asdf** shim in GUI apps (Claude Desktop often reports “No version is set for command npx”).
- **URL** (in **`args`**): **`https://…/mcp/`** with a single slash before **`mcp`** and a trailing slash where your edge expects it (see [Claude-mcp.md](../docs/integration/Claude-mcp.md)).
- **`GHOSTVAULT_BEARER_TOKEN`** (in **`env`**): session token from **`gvctl unlock -token-only`** (same string as **`GHOSTVAULT_BEARER_TOKEN`** / **`.ghostvault-bearer`** on the server). Used with **`--header`** so **`mcp-remote`** sends **`Authorization: Bearer …`** on requests to the MCP URL ([mcp-remote custom headers](https://github.com/geelen/mcp-remote#custom-headers)).

**Optional header block:** Stock Ghost Vault **`gvmcp`** authenticates to **`gvsvd`** using the token on the **server** (Docker **`.env`** / **`.ghostvault-bearer`** mount). The client **`Authorization`** header is **optional** and often unused. If you rely on server-side bearer only, remove the last two **`args`** entries (**`--header`** and **`"Authorization: Bearer ${GHOSTVAULT_BEARER_TOKEN}"`**) and remove the **`env`** object.

Configure **`GHOSTVAULT_BASE_URL`**, **`GHOSTVAULT_DEFAULT_*`**, and the **server** bearer (Compose **`.env`**, **`./.ghostvault-bearer`** mount) on the machine where **`gvmcp`** runs — see [docker-compose.yml](../docker-compose.yml) and [Claude-mcp.md](../docs/integration/Claude-mcp.md).

## Other templates

- [`mcp.claude-remote.example.json`](mcp.claude-remote.example.json) — same **`mcp-remote`** idea with a shorter placeholder URL.
