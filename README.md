# Ghost Vault

User-held **local memory replica** for LLMs: hybrid retrieval (pgvector + FTS + weighted gated fusion), **optional encryption at rest** (`GV_ENCRYPTION=on|off`, immutable per database), **OpenAPI** integrations (**ChatGPT** Actions, **Gemini** tools) via [`openapi/openapi.yaml`](openapi/openapi.yaml) (template: [`openapi/openapi.example.yaml`](openapi/openapi.example.yaml)), and **MCP** via **`gvmcp`** ([docs/integration/Claude-mcp.md](docs/integration/Claude-mcp.md)).

**CLI:** **`gvctl`** (`make ctl` → `bin/gvctl`) wraps common REST calls (`health`, `init`, `unlock`, `retrieve`, …) — `gvctl help`. **`gvmcp`** (`make mcp` → `bin/gvmcp`) is the MCP server; configure `GHOSTVAULT_BASE_URL`, `GHOSTVAULT_BEARER_TOKEN`, and optionally `GHOSTVAULT_DEFAULT_VAULT_ID` / `GHOSTVAULT_DEFAULT_USER_ID` (see MCP doc). Both binaries are also built into the Docker image under `/usr/local/bin/`.

## Quickstart (Docker)

```bash
make up               # .env → Postgres + gvsvd + gvmcp + dashboard container + edge (:8989)
make logs             # follow gvsvd
make down             # stop (volume kept)
make dashboard-dev    # optional: local Vite on :5177 (hot reload) → same API as Docker; not the dashboard container
```

**Single entry (default `http://127.0.0.1:8989`):** **`/api`** → **`gvsvd`**, **`/mcp/`** → **`gvmcp`**, **`/dashboard/`** → **`dashboard`** container (internal **:80**). **`make dashboard-dev`** is **:5177** on the host for local development only. Postgres is not published; use `docker compose exec postgres psql -U ghostvault -d ghostvault` for SQL.

Set secrets in [`.env.example`](.env.example) → `.env` (see [`Makefile`](Makefile)). With [direnv](https://direnv.net), use [`.envrc`](.envrc) (same idea as gis: `dotenv` loads `.env`).

## Quickstart (local `go run`)

```bash
cp .env.example .env   # set OPENAI_API_KEY, OPENAI_BASE_URL (OpenAI-compatible), DATABASE_URL
direnv allow           # optional
export DATABASE_URL=postgres://ghostvault:ghostvault@localhost:5432/ghostvault?sslmode=disable
go run ./cmd/gvsvd
```

**MCP (Claude Desktop, Cursor, …):** wire [docs/integration/Claude-mcp.md](docs/integration/Claude-mcp.md); use `gvctl unlock -token-only` and optionally `gvctl unlock -vault-id-only` for `GHOSTVAULT_DEFAULT_*` env vars.

Docker details: [`docker-compose.yml`](docker-compose.yml) · [`edge/nginx.full.conf.template`](edge/nginx.full.conf.template) · [`docs/deploy.md`](docs/deploy.md) (tunnels, remote access). Optional services: `make up MCP=disable` and/or `Dashboard=disable` (defaults enable). **HTTP tools:** step 1 [docs/integration/OPENAPI.md](docs/integration/OPENAPI.md); step 2 [docs/integration/CHATGPT.md](docs/integration/CHATGPT.md) or [docs/integration/GEMINI.md](docs/integration/GEMINI.md).

**Docs:** [docs/OVERVIEW.md](docs/OVERVIEW.md) · [docs/README.md](docs/README.md) · [gis/docs/design.md](../gis/docs/design.md) (GitS context).
