# Claude + Ghost Vault: MCP integration

Start with **[CLAUDE.md](./CLAUDE.md)** for the **stdio / `mcp-remote` vs hosted custom connector** comparison (the two paths are **not** interchangeable).

**Hosted connector + OAuth (Auth0, verification, troubleshooting):** **[Claude-connector.md](./Claude-connector.md)**. **OAuth-only details:** [oauth.md](./oauth.md). **This page** is the long-form **`gvmcp`** guide — env vars, Desktop JSON, **`mcp-remote`**, hosted connector mechanics, and the **`GHOSTVAULT_OAUTH_*`** table.

## Claude Code and gvctl

Use a project or personal agent skill: **[skills/README.md](./skills/README.md)**.

---

Ghost Vault itself is a **REST API** with **Bearer** auth ([`openapi/openapi.yaml`](../../openapi/openapi.yaml)). It does **not** speak MCP on the wire. This repo ships **`gvmcp`**, a small MCP server that exposes tools **`memory_search`**, **`memory_save`**, and **`memory_stats`**, and forwards each call to `POST /v1/retrieve`, `POST /v1/ingest`, and `GET /v1/stats` with `Authorization: Bearer …`. For agent orchestration and host prompts, see the **[ghostvault skill](./skills/ghostvault/SKILL.md)**.

**Anthropic Claude is supported** as a first-class MCP client: **Claude Desktop**, **claude.ai Connectors** (hosted MCP), and **Claude Code** all use the same **`gvmcp`** binary and environment variables as other MCP hosts (e.g. **Cursor**). There is no “upload `openapi.yaml` to Claude” flow comparable to ChatGPT Custom GPT Actions — attach Ghost Vault through **MCP** instead.

For **init, unlock, health, retrieve, …** without hand-written **`curl`**, use the **`gvctl`** CLI: `make ctl` → **`bin/gvctl`**, then **`gvctl help`**. Same flags and env as below (`GHOSTVAULT_BASE_URL`, passwords, tokens).

**Ghost Vault server URL:** Use one **HTTP origin** for **`gvsvd`** everywhere — the same value as **`GHOSTVAULT_BASE_URL`** for **`gvmcp`** / **`gvctl`** and (for OpenAPI-style clients) **`servers[0].url`** in [`openapi/openapi.yaml`](../../openapi/openapi.yaml). That origin can be **local** (e.g. Docker Compose edge `http://127.0.0.1:8989/api`), **private** (Tailscale, SSH port-forward), or **public HTTPS** (tunnel or hosted deploy). Topology and ports: [deploy.md](../deploy.md).

The sections that follow cover **stdio and remote MCP**, **env vars**, **Claude Desktop** registration, **hosted connector** details, and how this differs from ChatGPT Actions or Gemini.

### Claude vs ChatGPT (integration shape)

| Surface | Typical integration |
|---------|---------------------|
| **ChatGPT** | Custom GPT → **Actions** → import **OpenAPI**; OpenAI’s servers call your URL. See [CHATGPT.md](./CHATGPT.md) (after [OPENAPI.md](./OPENAPI.md) step 1). |
| **Claude** | **Connectors** / MCP config → **`gvmcp`** (**stdio**, **remote via `npx mcp-remote`**, **or one hosted custom connector URL** for web + Desktop — see below). |

### Anthropic documentation

- [Building custom connectors](https://claude.com/docs/connectors/building) — transports (Streamable HTTP), auth, limits.
- [Authentication reference](https://claude.com/docs/connectors/building/authentication) — OAuth callbacks, hosted vs Claude Code.

### Hosted custom connector

**Claude web + Desktop + mobile, one URL.** Anthropic’s **custom connector** calls your **`https://…/mcp-<token>/`** from **their cloud** (not via local **`mcp-remote`**). Register it **once** in **Customize → Connectors → Add custom connector**; the same connector **syncs** to **claude.ai**, **Claude Desktop**, and the **Claude mobile app** for your account ([Connectors overview](https://support.claude.com/en/articles/11176164-use-connectors-to-extend-claude-s-capabilities), [Get started with remote MCP](https://support.claude.com/en/articles/11175166-get-started-with-custom-connectors-using-remote-mcp), [Claude-connector.md — Mobile](./Claude-connector.md#claude-mobile)). The connector UI does **not** accept arbitrary **`Authorization`** headers — it expects **OAuth** against **your** authorization server ([Authentication reference](https://claude.com/docs/connectors/building/authentication)). Until you run that flow, Ghost Vault also gates the MCP URL with a secret **path segment** at the edge (**`GV_MCP_PATH_TOKEN`**) so the endpoint is not trivially guessable; that is **not** a substitute for OAuth on the wire (see next section).

1. **Generate a path token** (one-time): **`openssl rand -hex 32`**, put it in **`.env`** as **`GV_MCP_PATH_TOKEN=…`**. The edge container fails to start without it ([`docker-compose.yml`](../../docker-compose.yml)).
2. Run **`gvmcp`** behind Compose **edge** with TLS in front (**Tailscale Funnel**, ngrok, Cloudflare Tunnel, etc.) so **`curl -sS -o /dev/null -w "%{http_code}\n" "https://YOUR-HOST/…/api/healthz"`** prints **`200`** ([deploy.md — Tailscale](../deploy.md#single-edge-nginx-reference)). **Tailscale Serve** alone is tailnet-only and Anthropic’s cloud cannot reach it; use **Funnel** (or another public HTTPS option) for hosted connectors.
3. Configure **`GHOSTVAULT_BASE_URL`**, **`GHOSTVAULT_TOKEN_FILE`** (or **`GHOSTVAULT_BEARER_TOKEN`**), and optional **`GHOSTVAULT_DEFAULT_*`** on the **server** **`gvmcp`** process only — **not** in the connector OAuth fields unless you use a separate identity layer ([Claude-connector.md — Mobile](./Claude-connector.md#claude-mobile)).
4. **Register** **`https://YOUR-HOST/…/mcp-<TOKEN>/`** as the custom connector URL (trailing slash). The bare **`/mcp/`** path returns **404** by design.
5. **Remove** any **§5b** **`mcp-remote`** **`ghostvault`** entry from **`claude_desktop_config.json`** when you rely on the hosted connector everywhere, so Desktop does not spawn a second path to the same **`gvmcp`**.

**Verify:** Enable the connector in a chat and run **`memory_search`**. **HTTP 401** on tools means the session token on **`gvmcp`** is wrong or stale — see [Token rotation](#token-rotation-docker-gvmcp-and-hosted-connector) below.

**Path token rotation:** Edit **`GV_MCP_PATH_TOKEN`** in **`.env`**, run **`docker compose up -d --force-recreate edge`** (nginx re-renders [`edge/nginx.full.conf.template`](../../edge/nginx.full.conf.template) via envsubst at startup), then update the connector URL in Claude. The vault Bearer does **not** rotate from this action; do that separately with **`make rotate-token`**.

**Security rationale:** Tailscale Funnel / any public HTTPS has no auth in front of **`/mcp/`**, and Anthropic’s connector form does not support arbitrary request headers. Putting a 256-bit secret in the path keeps Anthropic as the only caller who holds the URL; **`/api/`** stays unchanged (already Bearer-gated by **`gvsvd`**) so **`gvctl`** / dashboard / ChatGPT Actions keep working. This is still a static secret — rotate it if the URL could have been exposed in logs or screenshots.

### OAuth 2.0 for Claude (custom connector auth)

[Authentication for connectors](https://claude.com/docs/connectors/building/authentication) describes what Claude supports: **`oauth_dcr`**, **`oauth_cimd`**, **`oauth_anthropic_creds`**, **`none`**, etc. **User-pasted static bearer tokens** for the MCP connection are **not** supported. For a proper OAuth integration, you run an **authorization server** (Keycloak, Auth0, Okta, Zitadel, ORY Hydra, …) that participates in Claude’s OAuth flow, registers redirect **`https://claude.ai/api/mcp/auth_callback`**, and issues **access tokens** that Anthropic sends to **`gvmcp`** as **`Authorization: Bearer …`**.

That is a **different** secret from **`GHOSTVAULT_BEARER_TOKEN`**: the latter is **`gvmcp` → `gvsvd`** (vault session) and stays on the server. OAuth tokens are **Claude → `gvmcp`** and prove the user connected via Anthropic’s consent screen.

**What this repo adds:** optional **RFC 9728** protected-resource metadata, **RFC 7662** token introspection, and a small **RFC 8414 authorization-server facade** on **`gvmcp`** when **`GHOSTVAULT_OAUTH_INTROSPECTION_URL`** is set. Edge nginx proxies **`GET /.well-known/oauth-protected-resource`**, **`GET /.well-known/oauth-authorization-server`**, **`GET /authorize`**, **`POST /token`** and **`POST /register`** to **`gvmcp`** ([`edge/nginx.full.conf.template`](../../edge/nginx.full.conf.template)). The Go MCP SDK introspection pattern matches [`examples/auth/server`](https://github.com/modelcontextprotocol/go-sdk/blob/main/examples/auth/server/main.go).

**Facade motivation:** claude.ai currently **ignores** the `authorization_servers` field in RFC 9728 metadata and constructs `/authorize`, `/token`, `/register` on the MCP host itself (see [anthropics/claude-ai-mcp#82](https://github.com/anthropics/claude-ai-mcp/issues/82)). Without this facade, `GET /authorize` 404s at the edge. The facade:

- `GET /.well-known/oauth-authorization-server` — returns RFC 8414 metadata whose `authorization_endpoint`, `token_endpoint`, `registration_endpoint` all point back at the MCP host.
- `GET /authorize` — 302 redirects to the upstream AS (defaults to `{issuer}authorize`), forwarding the query and injecting `audience` when missing.
- `POST /token` — reverse-proxies to the upstream token endpoint (defaults to `{issuer}oauth/token`), injecting `audience` when missing; forwards `Authorization` for confidential clients.
- `POST /register` — reverse-proxies to the upstream DCR endpoint (defaults to `{issuer}oidc/register`, the Auth0 Open DCR path).

**`audience`** is an Auth0-specific OAuth parameter; the facade sets it to **`GHOSTVAULT_OAUTH_AUDIENCE`** (which defaults to **`GHOSTVAULT_OAUTH_RESOURCE`**). The Auth0 API with that Identifier must exist, or Auth0 will issue a useless opaque token instead of a JWT with `aud=<resource>`.

**Step-by-step (Auth0, verification, troubleshooting):** [Claude-connector.md](./Claude-connector.md).

| Variable | Purpose |
|----------|---------|
| `GHOSTVAULT_OAUTH_INTROSPECTION_URL` | **Enables OAuth** when non-empty — RFC 7662 introspection POST URL |
| `GHOSTVAULT_OAUTH_AUTH_SERVER_ISSUER` | Issuer URL of your AS; used to derive upstream facade endpoints |
| `GHOSTVAULT_OAUTH_RESOURCE` | Full public MCP URL, e.g. **`https://your-host/mcp-<token>/`** |
| `GHOSTVAULT_OAUTH_METADATA_URL` | Full URL served as RFC 9728 metadata, e.g. **`https://your-host/.well-known/oauth-protected-resource`** |
| `GHOSTVAULT_OAUTH_INTROSPECTION_CLIENT_ID` / `GHOSTVAULT_OAUTH_INTROSPECTION_CLIENT_SECRET` | Optional; if your AS requires confidential introspection |
| `GHOSTVAULT_OAUTH_SCOPES` | Optional comma-separated scopes that must appear on the token |
| `GHOSTVAULT_OAUTH_SCOPES_SUPPORTED` | Optional; advertised in metadata |
| `GHOSTVAULT_OAUTH_AUDIENCE` | `audience` parameter the facade injects into `/authorize` and `/token` (defaults to `GHOSTVAULT_OAUTH_RESOURCE`) |
| `GHOSTVAULT_OAUTH_UPSTREAM_AUTHORIZE_URL` | Override the upstream `/authorize` URL (default `{issuer}authorize`) |
| `GHOSTVAULT_OAUTH_TOKEN_URL` | Override the upstream token URL (default `{issuer}oauth/token`) |
| `GHOSTVAULT_OAUTH_REGISTRATION_URL` | Override the upstream DCR URL (default `{issuer}oidc/register`) |

Introspection JSON must include **`exp`** (Unix seconds) for the access token. CLI flags **`gvmcp -oauth-*`** mirror these env vars.

**Connector “OAuth Client ID/Secret”** in Claude’s UI refer to the **OAuth client at your authorization server** that you registered for Claude — **not** the Ghost Vault vault session. If you cannot expose **DCR** or **CIMD**, use Anthropic’s **`oauth_anthropic_creds`** flow (**email `mcp-review@anthropic.com`**) per the [authentication doc](https://claude.com/docs/connectors/building/authentication).

You may keep **`GV_MCP_PATH_TOKEN`** **and** OAuth (defense in depth: secret URL plus bearer validation).

---

## Minimum steps (MCP host)

On each machine where you configure MCP (Claude Desktop, Cursor, etc.):

1. **Run Ghost Vault (`gvsvd`)** somewhere **reachable from the machine that runs `gvmcp`** — e.g. **`make up`** on the same host (**edge**: **`/api`**, **`/mcp-${GV_MCP_PATH_TOKEN}/`**, **`/dashboard/`** on **`http://127.0.0.1:8989`**), `go run ./cmd/gvsvd`, or a remote/tunneled URL you control ([deploy.md](../deploy.md)).
2. **Unlock** and obtain a **Bearer token** — same contract as OpenAPI integrations ([OPENAPI.md — Get a Bearer token](./OPENAPI.md#get-a-bearer-token-ghost-vault)). Easiest: **`gvctl unlock`** (or **`gvctl unlock -token-only`**) after **`export GHOSTVAULT_BASE_URL=…`** (or **`-base-url`**) matches **your Ghost Vault server URL**.
3. **Build `gvmcp`**: `make mcp` → **`bin/gvmcp`**, or `go build -o gvmcp ./cmd/gvmcp`.
4. **Set environment** for the MCP process:
   - **`GHOSTVAULT_BASE_URL`** — **your Ghost Vault server URL** (origin only, no trailing slash).
   - **`GHOSTVAULT_BEARER_TOKEN`** — session (or actions) token from step 2.
5. **Run `gvmcp`** with no arguments — **stdio** MCP (default). Point your host’s MCP settings at this binary with the env vars above.

**`gvmcp` flags (optional):** `-base-url` and `-bearer` override env for quick tests.

**Docker:** The image includes **`/usr/local/bin/gvmcp`**; most people still run **`gvmcp` on the host** so Claude Desktop/Cursor can spawn it.

---

## Local setup: gvsvd, `gvctl`, and Claude Desktop

**`gvctl`** talks to whatever origin **`GHOSTVAULT_BASE_URL`** (or **`-base-url`**) points at — often **your Ghost Vault server URL** on loopback when **`gvsvd`** runs on the same machine. **`curl`** examples below use **`$GHOSTVAULT_BASE_URL`** so you can swap local, tailnet, or tunneled origins without rewriting commands.

**Claude Desktop:** either spawn **local** **`gvmcp`** (**stdio**) with **`GHOSTVAULT_BASE_URL`** pointing at **`gvsvd`**, or point **`npx mcp-remote`** at an **HTTPS** **`gvmcp`** URL (**§5b**) — [deploy.md](../deploy.md).

### 1. Run Ghost Vault (`gvsvd`)

From the repo root:

```bash
make up
```

Wait until Postgres is healthy and **`gvsvd`** is reachable at **your Ghost Vault server URL** (Compose edge: **`http://127.0.0.1:8989/api`**; override with **`EDGE_HOST_PORT`** in `.env`).

Check liveness (pick one):

```bash
export GHOSTVAULT_BASE_URL="http://127.0.0.1:8989/api"   # your server URL (Compose edge)
gvctl health
```

```bash
# optional: raw curl equivalent (same origin as GHOSTVAULT_BASE_URL)
curl -sS "${GHOSTVAULT_BASE_URL}/healthz"
```

You should see **`ok`**. If you use `go run ./cmd/gvsvd`, set **`GHOSTVAULT_BASE_URL`** (or **`gvctl -base-url`**) to that origin everywhere below.

### 2. Initialize the vault once (first run only)

**`gvsvd` must be up** (step 1) before init. Creating a vault is a single **`POST /v1/vault/init`** on an empty database. It succeeds once; if you already initialized, the API returns **409** (`vault_exists`) — skip to **unlock**.

**Encryption on (`GV_ENCRYPTION=on`, Compose default):**

```bash
gvctl init -password 'YOUR_STRONG_PASSPHRASE'
```

**Encryption off (`GV_ENCRYPTION=off` before first init):**

```bash
gvctl init
```

Optional — same requests with **`curl`**:

```bash
curl -sS -X POST "${GHOSTVAULT_BASE_URL}/v1/vault/init" \
  -H "Content-Type: application/json" \
  -d '{"password":"YOUR_STRONG_PASSPHRASE"}'
# plaintext vault: use -d '{}' instead
```

On success (**HTTP 201**), the JSON includes **`vault_id`** (and **`encryption_enabled`**). Use that **`vault_id`** in **`memory_search`** / **`memory_save`**. Contract: [`openapi/openapi.yaml`](../../openapi/openapi.yaml). Docker/env: [deploy.md](../deploy.md).

Then **unlock** (next section) to get a **`session_token`** for **`GHOSTVAULT_BEARER_TOKEN`**.

### 3. Unlock and set `GHOSTVAULT_BEARER_TOKEN`

**Encrypted vault:**

```bash
gvctl unlock -password 'YOUR_PASSPHRASE'
```

Or set **`GHOSTVAULT_PASSWORD`** and run **`gvctl unlock`**.

**Token only** (e.g. for exports or MCP env):

```bash
gvctl unlock -token-only
```

**Plaintext vault** unlock is covered in [OPENAPI.md — Get a Bearer token](./OPENAPI.md#get-a-bearer-token-ghost-vault).

The printed JSON includes **`session_token`** unless you used **`-token-only`**. That value is **`GHOSTVAULT_BEARER_TOKEN`** for **`gvmcp`**. Refresh the token after expiry and update MCP config.

### 4. Build `gvmcp`

```bash
make mcp
```

Produces **`bin/gvmcp`** (or `go build -o bin/gvmcp ./cmd/gvmcp`). Use the **absolute path** in Claude Desktop (e.g. `/Users/you/.../GhostVault/bin/gvmcp`).

### 5. Register the MCP server in Claude Desktop

Anthropic’s UI changes; use **Settings → Developer / Connectors / MCP** as your version documents, or a legacy **`claude_desktop_config.json`** (e.g. under **`~/Library/Application Support/Claude/`** on macOS). Merge only the **`mcpServers`** object into your full file if it also has **`preferences`** or other top-level keys.

Pick **one** of the two patterns below.

#### 5a. Local **`gvmcp`** (stdio)

Claude spawns **`gvmcp`** on your machine. **`command`** must be the absolute path to **`gvmcp`**, and **`env`** must supply **Bearer** + **API** origin for **`gvsvd`**. Template: [`configs/mcp.example.json`](../../configs/mcp.example.json).

#### 5b. Remote streamable HTTP — **`npx mcp-remote`**

Use this when **`gvmcp`** is already exposed over **HTTPS** (for example **Docker Compose** **edge** at **`https://YOUR-HOST/mcp/`**, or **Tailscale Funnel** on **`EDGE_HOST_PORT`**, or **ngrok** aimed at the MCP path). Ghost Vault’s **`gvmcp -listen`** uses the MCP **streamable HTTP** transport; [`mcp-remote`](https://github.com/geelen/mcp-remote) bridges clients that only speak **stdio** (such as **Claude Desktop**) to that URL.

**What Desktop does *not* configure for remote:** **`GHOSTVAULT_BASE_URL`**, **`GHOSTVAULT_BEARER_TOKEN`**, and **`GHOSTVAULT_DEFAULT_*`** belong to the **server** that runs **`gvmcp`** (Docker **`gvmcp`** service env, or your shell when you launch **`gvmcp -listen`** locally). The remote URL is only the **MCP** endpoint — not the REST **`/api`** URL.

**Minimal example** (merge into **`mcpServers`**):

```json
"ghostvault": {
  "command": "/opt/homebrew/bin/npx",
  "args": [
    "-y",
    "mcp-remote",
    "https://YOUR-HOSTNAME.TAILNET.ts.net/mcp/",
    "--transport",
    "http-only"
  ]
}
```

- **`command`:** use an **absolute** path to **`npx`** so GUI apps (Claude Desktop) are not broken by **mise** / **asdf** shims — **`/opt/homebrew/bin/npx`** (Apple Silicon), **`/usr/local/bin/npx`** (Intel Homebrew). You can use **`"npx"`** only if your Desktop environment’s **`PATH`** resolves to a real Node without a version manager shim.
- Use a **single** slash before **`mcp-<token>`** (avoid **`//mcp-…`**). A trailing slash matches the edge **`location /mcp-${GV_MCP_PATH_TOKEN}/`** in [`edge/nginx.full.conf.template`](../../edge/nginx.full.conf.template); the bare **`/mcp/`** path returns **404** by design.
- **`--transport http-only`** matches **`gvmcp`**’s streamable HTTP handler; if something fails, try omitting it and rely on **`mcp-remote`** defaults (**`http-first`**).
- **Bearer to `gvsvd`:** stays on the server **`gvmcp`** process (**`GHOSTVAULT_BEARER_TOKEN`** in Compose). If you also need an **Authorization** header on **every request to `gvmcp`** (uncommon for tailnet-only access), use **`mcp-remote`**’s **`--header`** pattern — see [their README](https://github.com/geelen/mcp-remote#custom-headers) (avoid spaces inside **`args`** on some Windows builds; put **`Bearer …`** in an **`env`** var as they document).
- **Defaults for tools:** set **`GHOSTVAULT_DEFAULT_VAULT_ID`** / **`GHOSTVAULT_DEFAULT_USER_ID`** on the **server** (Compose env), since **`gvmcp`** **`args`** (**`-default-vault-id`**) are not used through **`npx`**.

Full copy-paste file: [`configs/mcp.claude-remote.example.json`](../../configs/mcp.claude-remote.example.json).

Requires **Node 18+** on the machine running Claude Desktop (**`npx`**). Restart Claude after edits.

**§5b troubleshooting**

| Symptom | What to check |
|---------|----------------|
| **`ECONNREFUSED`** to **`…:443`** (or **`fetch failed`** from **`mcp-remote`**) | Edge is **HTTP-only** on **8989**. **MagicDNS** does not open **443** — run **Tailscale Serve** or **Funnel** on the GhostVault host forwarding to **`http://127.0.0.1:8989`**, then **`curl https://YOUR-NODE/…/api/healthz`** until you get **200**. See [deploy.md — Tailscale](../deploy.md#single-edge-nginx-reference). |
| **`No version is set for command npx`** (mise / asdf) | Claude Desktop does not load interactive shell hooks. Use an **absolute** **`npx`** path in **`command`**, e.g. **`/opt/homebrew/bin/npx`** (Apple Silicon Homebrew) or **`/usr/local/bin/npx`** (Intel Homebrew), not the mise shim. |
| **`502`** on **`/mcp-<token>/`** | **`gvmcp`** Compose profile off — use **`make up`** with **`MCP=enable`** (default). |
| **`404`** on **`/mcp-<token>/`** | Wrong token or edge not rebuilt after **`.env`** change — re-read **`GV_MCP_PATH_TOKEN`** and run **`docker compose up -d --force-recreate edge`**. The bare **`/mcp/`** path also returns **404** intentionally. |
| **`401`** from tools | **`GHOSTVAULT_BEARER_TOKEN`** on the **server** **`gvmcp`** env — unlock again after **`gvsvd` restart**. |

---

**`vault_id` and `user_id` are not Claude UI fields** — they are arguments to **`memory_search`** / **`memory_save`**. For **stdio** (§5a), set defaults on the **`gvmcp`** process (**`env`**). For **remote** (§5b), set **`GHOSTVAULT_DEFAULT_*`** on the server’s **`gvmcp`** — see the table below.

| Variable | Purpose |
|----------|---------|
| **`GHOSTVAULT_BASE_URL`** | **Where `gvmcp` calls `gvsvd`** (e.g. Compose `http://127.0.0.1:8989/api`, or **`http://ghostvault:8080`** inside Docker). Set on the **`gvmcp`** process — **not** in Claude Desktop when using **§5b** **`mcp-remote`**. |
| **`GHOSTVAULT_BEARER_TOKEN`** | Session token (must match this vault). In **§5a**, paste into Desktop **`env`**. In **§5b**, set in Compose / server env for **`gvmcp`**. |
| **`GHOSTVAULT_DEFAULT_VAULT_ID`** | **Optional.** The **`vault_id`** from **`gvctl unlock`**. Same placement rules as Bearer — Desktop **`env`** for **§5a**; server **`env`** for **§5b**. |
| **`GHOSTVAULT_DEFAULT_USER_ID`** | **Optional.** Logical scoping id (e.g. `default`). Same as above. |

**Do not mix these up:** the **`vault_id`** UUID from **`gvctl unlock`** belongs in **`GHOSTVAULT_DEFAULT_VAULT_ID`**, not in **`GHOSTVAULT_DEFAULT_USER_ID`**. The latter is **not** your employer name or work ID — it is an arbitrary **namespace label** inside the vault (one vault can hold separate memory streams for `default` vs `work` if you choose). For a single personal setup, **`default`** is enough.

**Per request vs env:** Each tool call may include **`vault_id`** / **`user_id`**. If you set **`GHOSTVAULT_DEFAULT_VAULT_ID`** / **`GHOSTVAULT_DEFAULT_USER_ID`** on **`gvmcp`**, those values apply when the model **omits** them — and **`gvmcp`** also treats **`vault_id: "default"`** (a common LLM placeholder) as “use **`GHOSTVAULT_DEFAULT_VAULT_ID`**” when that env var is set. **`vault_id`** ultimately comes from the **vault row** created at **`POST /v1/vault/init`**; **`gvctl unlock`** echoes the same **`vault_id`** every time.

**Why Claude doesn’t “know” the UUID:** The MCP host only sends **tool schemas** to the model, not your **`env`**. So the assistant may guess placeholders unless you set **`GHOSTVAULT_DEFAULT_VAULT_ID`** (and unlock after **`gvsvd` restart** so **`GHOSTVAULT_BEARER_TOKEN`** stays valid). **HTTP 401** always means **Bearer token** rejected (wrong, expired, or **`gvsvd` restarted**), not a bad **`vault_id`**.

Get the canonical **`vault_id`** after unlock:

```bash
gvctl unlock -vault-id-only
```

(or run **`gvctl unlock`** and read **`vault_id`** in the printed JSON).

**§5a — full `mcpServers` example (local `gvmcp`, stdio):**

```json
{
  "mcpServers": {
    "ghostvault": {
      "command": "/ABSOLUTE/PATH/TO/GhostVault/bin/gvmcp",
      "args": [
        "-default-vault-id",
        "PASTE_OUTPUT_OF_gvctl_unlock_-vault-id-only",
        "-default-user-id",
        "default"
      ],
      "env": {
        "GHOSTVAULT_BASE_URL": "YOUR_GHOSTVAULT_SERVER_URL",
        "GHOSTVAULT_TOKEN_FILE": "/ABSOLUTE/PATH/TO/.ghostvault-bearer"
      }
    }
  }
}
```

Or pass **`GHOSTVAULT_BEARER_TOKEN`** instead of **`GHOSTVAULT_TOKEN_FILE`**. **`YOUR_GHOSTVAULT_SERVER_URL`** is the REST origin (Compose edge: **`http://127.0.0.1:8989/api`**).

- **`command`**: absolute path to **`gvmcp`**, not **`gvsvd`**.
- If **`GHOSTVAULT_DEFAULT_VAULT_ID`** is wrong or from another install, **`gvsvd`** returns **403** / invalid vault for that session — unlock again and paste the **`vault_id`** from that response.
- Normally **do not** run **`gvmcp`** manually in a terminal; the app starts it when needed.

### 6. Restart Claude Desktop

Quit fully and reopen so MCP settings reload.

### 7. Verify in chat

In Claude, enable the Ghost Vault connector if there is a toggle. Try **`memory_search`** / **`memory_save`** with **`vault_id`** and **`user_id`** from your vault ([OpenAPI](../../openapi/openapi.yaml)).

If tools fail: **`make logs`** for **`gvsvd`**, unlock again if the session expired, then sanity-check from the shell:

```bash
gvctl health
export GHOSTVAULT_BEARER_TOKEN=$(gvctl unlock -token-only)
gvctl retrieve -vault-id "YOUR_VAULT_UUID" -user-id "u1" -query "test"
```

(`retrieve` needs a valid token and OpenAI configured in **`gvsvd`** for real results.)

---

## Local (stdio) vs remote (HTTP + tunnel)

| Mode | When to use | What you run |
|------|-------------|----------------|
| **stdio** | **Cursor**, **Claude Desktop** (§5a), other clients that spawn **`gvmcp`** | Local **`gvmcp`** with **`GHOSTVAULT_BASE_URL`** + **`GHOSTVAULT_BEARER_TOKEN`** (or token file). |
| **Claude Desktop + remote URL** | **`gvmcp`** is already behind **HTTPS** (edge **`/mcp/`**, Tailscale, tunnel) | Desktop **`npx mcp-remote …`** (§5b) — **no** **`GHOSTVAULT_*`** in the Desktop JSON; configure those on the server **`gvmcp`**. |
| **Streamable HTTP (raw)** | **Hosted** Claude **Connectors** (web + Desktop, one URL), or integrations that accept an MCP URL | **`gvmcp -listen`** (or Docker **edge**), **`GHOSTVAULT_*`** on the host running **`gvmcp`**; expose **HTTPS** to the **MCP** path (not the **`/api`** path). See [Hosted custom connector](#hosted-custom-connector). |

For HTTP mode, Anthropic may require **OAuth** or other connector auth as their UI evolves — see [Authentication reference](https://claude.com/docs/connectors/building/authentication) (also listed above). **Local stdio** is the simplest path; **§5b** is for **remote HTTPS MCP** when Claude Desktop **cannot** use the hosted connector UI and must bridge with **`mcp-remote`**.

---

## What this approach works for

| Use case | Why MCP fits |
|----------|----------------|
| **Tool calling from an MCP-capable assistant** | The client discovers tools, invokes them by name, and passes structured arguments — `gvmcp` maps those to `POST /v1/retrieve`, `POST /v1/ingest`, and `GET /v1/stats`. |
| **Claude (hosted) “Connectors”** | Anthropic’s path for custom capabilities is **MCP** (e.g. streamable HTTP). |
| **Claude Desktop / Claude Code** | Load **`gvmcp`** via **Settings → Connectors** or MCP config; stdio is typical. |
| **IDEs (e.g. Cursor)** | Point MCP config at the `gvmcp` binary and env vars. |

You still need **`gvsvd`** **reachable from `gvmcp`** at **`GHOSTVAULT_BASE_URL`** — often **loopback** when both run on one host, or a tailnet/tunnel URL when **`gvsvd`** is remote. For hosted MCP, a common pattern is **`gvmcp`** on an always-on host with **`GHOSTVAULT_BASE_URL`** to co-located **`gvsvd`**, while a **separate** tunnel exposes only the **MCP** HTTP port. General tunnel copy/paste: [deploy.md — Expose `gvsvd` over HTTPS](../deploy.md#expose-gvsvd-over-https-chatgpt-actions-remote-testing) (aim at the **MCP** port when using **`gvmcp -listen`**; aim at **`gvsvd`** when exposing the REST API for ChatGPT Actions).

---

## Providers and clients (summary)

| Provider / surface | MCP relevant? | Typical path |
|--------------------|---------------|--------------|
| **Anthropic Claude** (claude.ai) | **Yes** — Connectors | [**Hosted custom connector**](#hosted-custom-connector) — **`https://…/mcp/`** + server-side **`gvmcp`** env. |
| **Claude Desktop** | **Yes** | Same **hosted connector** URL as web, or [**§5a** / **§5b**](#5-register-the-mcp-server-in-claude-desktop) — local **`gvmcp`** (stdio) or **`npx mcp-remote`**. |
| **Claude Code** / API MCP | **Yes** | Same tools; wire per Anthropic’s API docs. |
| **Cursor** | **Yes** | **`gvmcp`** + env in MCP config. |
| **ChatGPT** (Custom GPT **Actions**) | **No** | [OPENAPI.md](./OPENAPI.md) (step 1) → [CHATGPT.md](./CHATGPT.md) (step 2). |
| **Gemini** | **Not** MCP-first here | [OPENAPI.md](./OPENAPI.md) (step 1) → [GEMINI.md](./GEMINI.md) (step 2). |

**Alternative:** community **OpenAPI → MCP** bridges using [`openapi/openapi.yaml`](../../openapi/openapi.yaml) — **`gvmcp`** is the supported first-party option.

---

## Limits to keep in mind

- **Timeouts and payload caps** depend on the **host** (e.g. Claude hosted surfaces document max tool result size on the order of hundreds of thousands of characters). Very large `memory_search` results may need narrower queries or truncation.
- **Privacy:** Same pattern as ChatGPT Actions — only what the model retrieves in a turn is sent upstream; see Anthropic’s policies for logging and retention.
- **Secrets:** MCP configs often **store** env or tokens; rotate if leaked.
- **Same vault rules:** stable **`vault_id`** / **`user_id`**, tunnel uptime for remote HTTP.
- **Restart `gvsvd` → new unlock:** Session tokens are **in-memory** in the server process. After **`make down`/`make up`**, **`go run` restart**, or container restart, **old `GHOSTVAULT_BEARER_TOKEN` values stop working** (**HTTP 401**). Run **`gvctl unlock -token-only`** (or full **`gvctl unlock`**), refresh the token in **§5a** Desktop **`env`** or in the **Compose / server** **`gvmcp`** env for **§5b** / hosted connector, and restart Claude Desktop if needed. Details: [Token rotation](#token-rotation-docker-gvmcp-and-hosted-connector).

### Token rotation (Docker gvmcp and hosted connector)

When **`gvsvd`** restarts, session tokens invalidate. **`gvmcp`** must get a new Bearer (same rules as [UNLOCK-AND-BEARER.md](../UNLOCK-AND-BEARER.md)).

**Docker Compose with token file (recommended):**

1. From the repo root, with **`GHOSTVAULT_BASE_URL`** pointing at your API (e.g. **`http://127.0.0.1:8989/api`**) and **`GHOSTVAULT_PASSWORD`** or **`-password`** if encrypted: run **`make rotate-token`**. That runs **`scripts/refresh-ghostvault-token.sh`**, writes **`.ghostvault-bearer`**, and **force-recreates** the **`gvmcp`** container so it reloads the mount.
2. If **`gvmcp`** is not running (**`MCP=disable`**), **`make rotate-token`** still refreshes the file; start **`gvmcp`** afterward or use **`make up`** as usual.

**Hosted connector only:** No change in Anthropic’s UI — the same **`https://…/mcp/`** keeps working once **`gvmcp`** holds a valid token again.

### Operational tips

- **Connector / tunnel:** If remote MCP fails to connect, check the MCP transport URL, TLS, and firewall; the tunnel must reach **`gvmcp`**’s **`-listen`** address (not necessarily **`gvsvd`**’s port).
- **Tools missing or wrong shape:** Use **`gvmcp`** from this repo; tool names align with the REST contract in **`openapi/openapi.yaml`**.
- **Shared settings:** Connector UIs may store secrets where others can see them; prefer dedicated vaults or token rotation when settings are visible.

## Troubleshooting

| Symptom | Likely cause |
|---------|----------------|
| **401** on tools | Expired session, **`gvsvd` restarted**, or wrong token in MCP **`env`**. Unlock again; update **`GHOSTVAULT_BEARER_TOKEN`**. |
| **403** / vault mismatch | **`vault_id`** in the tool call (or **`GHOSTVAULT_DEFAULT_VAULT_ID`**) does not match the vault for this session. Use **`gvctl unlock -vault-id-only`** — do **not** use the literal string **`"default"`** as **`vault_id`** (that must be a **UUID**). **`user_id`** can be a label like **`default`**. |
| Connector cannot connect | MCP URL / TLS / firewall; for hosted Claude, the tunnel must terminate on **`gvmcp`**’s HTTP listener. |

---

## See also

- [Claude-connector.md](./Claude-connector.md) — **hosted connector + OAuth + Auth0** (single guide)  
- [CLAUDE.md](./CLAUDE.md) — short MCP vs connector pointer  
- [Anthropic — Connectors](https://support.claude.com/en/articles/11176164-use-connectors-to-extend-claude-s-capabilities)  
- [OPENAPI.md](./OPENAPI.md) — shared HTTP setup and Bearer token (same as `gvmcp` unlock)  
- [CHATGPT.md](./CHATGPT.md) · [GEMINI.md](./GEMINI.md) — product-specific step 2  
- [deploy.md](../deploy.md) — Docker, tunnels, encryption  
- [`configs/mcp.example.json`](../../configs/mcp.example.json) · [`configs/mcp.develop.example.json`](../../configs/mcp.develop.example.json) — local **stdio** `gvmcp` · [`configs/mcp.remote.example.json`](../../configs/mcp.remote.example.json) · [`configs/mcp.local.example.json`](../../configs/mcp.local.example.json) · [`configs/mcp.claude-remote.example.json`](../../configs/mcp.claude-remote.example.json) — **`mcp-remote`** over **HTTPS** `…/mcp/` (use **`/opt/homebrew/bin/npx`** or **`/usr/local/bin/npx`**, not the mise shim)  
