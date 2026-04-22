# Ghost Vault

[![Ghost Vault](https://raw.githubusercontent.com/Z-ghostshell/GhostVault/main/dashboard/public/favicon.svg)](https://ghostvault.bizs.app/)

**Site:** [https://ghostvault.bizs.app/](https://ghostvault.bizs.app/) — product overview, tutorial (including the Anthropic remote MCP connector), and blog.

**The idea:** the durable record of your work should live in a layer *you* govern—not as a retention feature inside someone else’s cloud account. Ghost Vault is a **user-held memory service**: your notes, decisions, and context stay **on your machine** (or infra you control). The models you already use—Claude, ChatGPT, Gemini, Cursor, and the rest—connect through **open, swappable interfaces** (HTTP/OpenAPI, Model Context Protocol) so the assistant sees **your** ground truth, not a fresh session every time. Switching tools should not mean starting from zero.

**What it is:** a small **Go** service plus **Postgres** (vectors + full-text), **hybrid retrieval**, and **optional encryption at rest** so ciphertext stays on disk until you unlock. Integrations are documented under [`docs/integration/`](docs/integration/) (MCP, ChatGPT Actions, Gemini, and similar).

## Run it

1. `make setup` — creates `.env` from [`.env.example`](.env.example) if needed; set API keys and `DATABASE_URL` as documented there.  
2. `make up` — brings up Postgres, the vault API (`gvsvd`), an edge reverse proxy on **:8989**, and (by default) the MCP sidecar and dashboard. Use `make` to see all targets; details live in [`docs/deploy.md`](docs/deploy.md) and [`docs/README.md`](docs/README.md).

**Dashboard:** after `make up` (dashboard profile on by default), open **`http://127.0.0.1:8989/dashboard/`** — the edge serves **`/api`**, **`/mcp/`**, and proxies **`/dashboard/`** to the dashboard container (internal **:80**). For a **local Vite** UI with hot reload instead, run **`make dashboard-dev`** (default **:5177**, set `DASHBOARD_DEV_PORT` if needed) and point **`GHOSTVAULT_BASE_URL`** / **`GHOSTVAULT_PROXY_URL`** at your `gvsvd` (see [`.env.example`](.env.example)). With **`make up Dashboard=disable`**, use `dashboard-dev` the same way.

**Without Docker:** copy `.env`, set `DATABASE_URL`, `go run ./cmd/gvsvd`. **MCP / IDE setup:** [docs/integration/Claude-mcp.md](docs/integration/Claude-mcp.md).

**Product and threat-model depth:** [docs/OVERVIEW.md](docs/OVERVIEW.md) · [openapi/openapi.yaml](openapi/openapi.yaml). **Related (GitS / Ghost):** [../gis/docs/design.md](../gis/docs/design.md).
