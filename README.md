<table align="center">
  <tr>
    <td align="right" valign="middle" style="padding-right:1.5em">
      <span style="display:inline-block;border:1px solid;border-radius:999px;padding:0.15em 0.85em 0.3em;line-height:1">
        <span style="font-size:3em;font-weight:700;letter-spacing:-0.02em">Ghost&nbsp;Vault</span>
      </span>
      <br /><br />
      <span style="font-size:0.95em;opacity:0.85">local memory for your models</span>
      <hr align="right" width="72" style="margin:0.75em 0 0 auto;opacity:0.45" />
    </td>
    <td align="left" valign="middle" style="padding-left:1.5em">
      <a href="https://ghostvault.bizs.app/"><img src="https://ghost.bizs.app/resource/ghost/selective-icon/suitcase.webp" width="128" height="128" alt="Ghost Vault" /></a>
    </td>
  </tr>
</table>

<p align="center">
  <strong>Your memory, your machine</strong> — the durable record of your work lives in a layer <em>you</em> govern, not as a retention feature in someone else’s account. <strong>Ghost Vault</strong> is a <strong>user-held memory service</strong>: connect the same vault to Claude, ChatGPT, Gemini, Cursor, and the rest through <strong>open, swappable interfaces</strong> (OpenAPI, Model Context Protocol) so the model sees <strong>your</strong> ground truth. Switching tools should not mean starting from zero.
</p>

<p align="center">
  <a href="https://ghostvault.bizs.app/">ghostvault.bizs.app</a> — product overview, tutorial (including the Anthropic remote MCP), and blog
</p>

**What it is:** a small **Go** service plus **Postgres** (vectors + full-text), **hybrid retrieval**, and **optional encryption at rest** (ciphertext on disk until you unlock). Integrations: [`docs/integration/`](docs/integration/) (MCP, ChatGPT Actions, Gemini, and similar).

## Run it

1. `make setup` — creates `.env` from [`.env.example`](.env.example) if needed; set API keys and `DATABASE_URL` as documented there.  
2. `make up` — brings up Postgres, the vault API (`gvsvd`), an edge reverse proxy on **:8989**, and (by default) the MCP sidecar and dashboard. Use `make` to see all targets; details live in [`docs/deploy.md`](docs/deploy.md) and [`docs/README.md`](docs/README.md).

**Dashboard:** after `make up` (dashboard profile on by default), open **`http://127.0.0.1:8989/dashboard/`** — the edge serves **`/api`**, **`/mcp/`**, and proxies **`/dashboard/`** to the dashboard container (internal **:80**). For a **local Vite** UI with hot reload instead, run **`make dashboard-dev`** (default **:5177**, set `DASHBOARD_DEV_PORT` if needed) and point **`GHOSTVAULT_BASE_URL`** / **`GHOSTVAULT_PROXY_URL`** at your `gvsvd` (see [`.env.example`](.env.example)). With **`make up Dashboard=disable`**, use `dashboard-dev` the same way.

**Without Docker:** copy `.env`, set `DATABASE_URL`, `go run ./cmd/gvsvd`. **MCP / IDE setup:** [docs/integration/Claude-mcp.md](docs/integration/Claude-mcp.md).

**Product and threat-model depth:** [docs/OVERVIEW.md](docs/OVERVIEW.md) · [openapi/openapi.yaml](openapi/openapi.yaml). **Related (GitS / Ghost):** [../gis/docs/design.md](../gis/docs/design.md).
