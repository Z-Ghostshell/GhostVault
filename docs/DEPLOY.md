# Deploy: Docker Compose, reverse proxy, tunnels

This doc covers **running the stack**, **routing multiple services behind one port** (nginx), and **exposing** HTTP to the internet or a tailnet — replacing the older split between “local vs hosting” notes and a separate DEPLOY guide.

---

## Docker Compose (default layout)

[`docker-compose.yml`](../docker-compose.yml) runs **Postgres**, **`gvsvd`** (**`ghostvault`**), optional **`gvmcp`** (profile **`mcp`**), optional **`dashboard`** (profile **`dashboard`** — nginx serving the built SPA), and **`edge`** ([`edge/nginx.full.conf.template`](../edge/nginx.full.conf.template), [`edge/Dockerfile`](../edge/Dockerfile)) — proxy only, **no** embedded UI build. **Edge requires `GV_MCP_PATH_TOKEN`** in **`.env`** (generated with **`openssl rand -hex 32`**) so the MCP endpoint is served at **`/mcp-<token>/`** instead of a bare **`/mcp/`** ([Hosted custom connector](integration/Claude-mcp.md#hosted-custom-connector)).

| Service | Role | Published to host |
|--------|------|-------------------|
| **postgres** | pgvector | *(internal only — not published by default)* |
| **ghostvault** | **`gvsvd`** (`/v1/…`, `/healthz`, …) on **8080** | *(internal)* |
| **gvmcp** | Streamable MCP on **3751** | *(internal)* |
| **dashboard** | Static SPA only (**[`dashboard/Dockerfile`](../dashboard/Dockerfile)**); **`/dashboard/`** on port **80** inside the network | *(internal)* |
| **edge** | Reverse proxy: **`/api`**, **`/mcp/`**, **`/dashboard/`** → **`dashboard:80`** | **`8989`** (`EDGE_HOST_PORT`) |

**Local UI (not Compose):** **`make dashboard-dev`** runs **Vite** on **`127.0.0.1:5177`** with hot reload; it proxies **`/v1`** to **`gvctl`** / **`GHOSTVAULT_BASE_URL`** (often **`http://127.0.0.1:8989/api`** when the stack is up). That is **separate** from the **`dashboard`** container, which is what **`http://127.0.0.1:8989/dashboard/`** uses.

### Retrieval / ingest API debug (operators only)

- Set **`GV_RETRIEVE_DEBUG=true`** on **`ghostvault` (`gvsvd`)** and restart. When the client sets **`"debug": true`**, **`POST /v1/retrieve`** may include a **`debug`** object (dense/lexical candidates, fusion parameters, packing). **`POST /v1/ingest`** may include **`debug.segments`** (chunking) or **`debug.extracted_facts`** (when **`infer: true`**). If the env is unset or false, the server **ignores** the flag in the body and returns normal responses without **`debug`**.

- **Security:** debug payloads list **chunk IDs and relative scores** for many candidates. **Do not** enable **`GV_RETRIEVE_DEBUG`** on internet-exposed or multi-tenant hosts unless you understand the risk. Default is off.

- **Dashboard “Retrieve debug”** page: in dev (`make dashboard-dev`), the sidebar link is on by default (you can turn it off with the on-page opt-out). For **production** static builds, set **`VITE_SHOW_RETRIEVE_DEBUG=true`** at image build time ([`dashboard/Dockerfile`](../dashboard/Dockerfile) `ARG`); otherwise the nav entry stays hidden. The UI still only receives rich **`debug`** JSON when the server allows it as above.

### `make up` options (not `.env`)

| Variable | Default | When **`disable`** |
|----------|---------|---------------------|
| **`MCP`** | **`enable`** | **`gvmcp`** is not started (Compose omit **`--profile mcp`**). **`/mcp/`** on edge returns **502** unless you change nginx. |
| **`Dashboard`** | **`enable`** | Starts the **`dashboard`** service (profile **`dashboard`**) and builds edge with **`WITH_DASHBOARD=1`** (**`/dashboard/`** → **`dashboard:80`**). **`disable`:** no **`dashboard`** container; edge uses **`nginx.api-mcp.conf.template`** (**`/`** plain-text hint); rebuild edge when switching. |

Examples (space-separated — **not** comma-separated): **`make up`**, **`make up MCP=disable`**, **`make up Dashboard=disable`**, **`make up MCP=Enable Dashboard=disable`**. Changing **`Dashboard`** requires **`BUILD=true make up`** (or **`make build`**) so edge rebuilds.

**Only** **`http://127.0.0.1:8989`** (by default) is exposed on the host:

| Path | Backend |
|------|---------|
| **`/api/…`** | **`gvsvd`**: browser and **`gvctl`** use **`GHOSTVAULT_BASE_URL=http://127.0.0.1:8989/api`** (no trailing slash). Example: **`/api/v1/stats`**, **`/api/healthz`**. |
| **`/mcp-<token>/`** | **HTTP MCP** (remote connectors → `https://…/mcp-${GV_MCP_PATH_TOKEN}/`). The bare **`/mcp/`** path returns **404** by design. |
| **`/dashboard/`** | Dashboard SPA (`/` redirects to **`/dashboard/`**) |

Inside the Compose network, **`gvmcp`** still calls **`http://ghostvault:8080`** directly (not through **`/api`**).

Env templates: [`.env.example`](../.env.example), **`make setup`**.

---

## Single edge nginx (reference)

The packaged templates are **[`edge/nginx.full.conf.template`](../edge/nginx.full.conf.template)** (proxy **`/dashboard/`** to **`http://dashboard:80`**) and **[`edge/nginx.api-mcp.conf.template`](../edge/nginx.api-mcp.conf.template)** (no UI; **`make up Dashboard=disable`**). Both are rendered by the nginx image entrypoint at container start with envsubst (`NGINX_ENVSUBST_FILTER=^GV_`) so **`${GV_MCP_PATH_TOKEN}`** is substituted while nginx built-ins like **`$host`** stay literal.

**Remote MCP URL:** Register **`https://your-host/…/mcp-${GV_MCP_PATH_TOKEN}/`** in Anthropic’s **custom connector** UI (same URL for **claude.ai** and **Claude Desktop** after account sync). **`gvmcp`** env **`GHOSTVAULT_BASE_URL=http://ghostvault:8080`** in Compose. Bearer and defaults stay on the **`gvmcp`** process — not in the connector form ([Claude-mcp.md — Hosted custom connector](integration/Claude-mcp.md#hosted-custom-connector)). Rotate the path token by editing **`.env`** and running **`docker compose up -d --force-recreate edge`**.

**Tailscale — MagicDNS is not HTTPS by itself:** Resolving **`your-node.your-tailnet.ts.net`** only answers DNS; it does **not** listen on port **443**. Edge serves **plain HTTP** on **`127.0.0.1:8989`** (see table above). To use **`https://…/api/…`** or **`https://…/mcp-${GV_MCP_PATH_TOKEN}/`** (e.g. **Anthropic hosted connector** or **`mcp-remote`** in Claude Desktop), run **Tailscale Serve** (tailnet-only) or **Funnel** (public internet) on the GhostVault host so TLS terminates and traffic forwards to **`http://127.0.0.1:8989`** (or **`EDGE_HOST_PORT`**). Hosted connectors are reached **from Anthropic’s cloud**, which is **not** on your tailnet — those need **Funnel** (or another public HTTPS option).

| Mechanism | Typical use | Idea |
|-----------|-------------|------|
| **`tailscale serve`** | HTTPS on your tailnet hostname → local **8989** | Often **`tailscale serve --bg http://127.0.0.1:8989`** — use the **`https://…`** URL Tailscale prints (CLI flags vary by version; see **`tailscale serve --help`**). |
| **`tailscale funnel …`** | Public internet HTTPS → local **8989** | **`tailscale funnel 8989`** (or your **`EDGE_HOST_PORT`**) — same edge paths: **`/api`**, **`/mcp/`**, **`/dashboard/`**. |

**Verify from the client (after Serve/Funnel is up):**

```bash
curl -sS -o /dev/null -w "%{http_code}\n" "https://YOUR-NODE.TAILNET-NAME.ts.net/api/healthz"
```

Expect **`200`**. **Connection refused on port 443** means nothing is terminating HTTPS for that name — start or fix Serve/Funnel, or confirm the URL matches what **`tailscale status`** / the Serve dashboard shows.

---

## Local vs remote processes (topology)

**`gvsvd`** holds data; **`gvmcp`** is a bridge that calls **`gvsvd`** with **`GHOSTVAULT_BASE_URL`** and a **Bearer** token.

- **Fully local:** **`make up`** — loopback URLs until you add a tunnel; nothing is on the public internet by default.
- **Hosted MCP (public HTTPS):** Some products need a **reachable MCP URL**. Run **`gvmcp -listen`**, terminate **TLS + hostname** in front of that listener (**ngrok**, **Cloudflare Tunnel**, **edge nginx** + Funnel, etc.). **`GHOSTVAULT_BASE_URL`** is still **`gvsvd`** (often **`http://127.0.0.1:…`** or **`http://ghostvault:8080`** in Docker when co-located).
- **Private access (Tailscale):** Join machines to one tailnet; use tailnet IPs or MagicDNS. **ACLs** still matter — anyone who can reach the port needs the Bearer token.
- **Assistant local, vault in the cloud:** Run **`gvsvd`** on the server; on the laptop, **`gvmcp`** with **`GHOSTVAULT_BASE_URL`** pointing at the server (Tailscale, SSH port-forward, or HTTPS).
- **After `gvsvd` restart**, session tokens invalidate — unlock again and refresh **`GHOSTVAULT_BEARER_TOKEN`** wherever **`gvmcp`** or clients store it ([Claude-mcp.md](integration/Claude-mcp.md)).

**Using both local and remote:** Common pattern — daily **stdio** **`gvmcp`** + local **`gvsvd`**; sometimes **HTTP `gvmcp`** + tunnel for a hosted connector session while **`gvsvd`** stays local.

---

<a id="expose-gvsvd-over-https-chatgpt-actions-remote-testing"></a>

## Expose gvsvd over HTTPS (ChatGPT Actions, remote testing)

Point your tunnel or reverse proxy at **edge** (default **`127.0.0.1:8989`**) or at **`gvsvd`** directly if you run it without Compose.

- **Docker Compose (default):** tunnel **`8989`** — health check **`https://YOUR-TUNNEL/…/api/healthz`** (or **`http://127.0.0.1:8989/api/healthz`** locally).
- **`go run ./cmd/gvsvd`:** often **`http://127.0.0.1:8080`** — **`curl …/healthz`** at that origin.

Confirm from the internet:

```bash
curl -sS "https://YOUR-TUNNEL.example.com/api/healthz"
```

To expose **streamable MCP** through the same tunnel (Compose default), use **`https://YOUR-TUNNEL/mcp-${GV_MCP_PATH_TOKEN}/`** (the bare **`/mcp/`** path returns **404**).

<a id="a-ngrok"></a>

### (a) ngrok

1. Install the [ngrok agent](https://ngrok.com/download) and **`ngrok config add-authtoken …`**.
2. Start **`gvsvd`** (or **edge** on **`8989`**).
3. **`ngrok http 8989`** (Compose default edge port).
4. Set **`servers[0].url`** in `openapi/openapi.yaml` to the **`https://…`** forwarding URL (no trailing slash). See [OPENAPI.md — ngrok](integration/OPENAPI.md#ngrok-integration).

<a id="b-cloudflare-tunnel-quick--ephemeral"></a>

### (b) Cloudflare Tunnel (quick / ephemeral)

Use **Cloudflare Tunnel** (or another HTTPS reverse proxy) the same way: terminate TLS and forward to **`127.0.0.1:8989`** (edge) or your **`gvsvd`** port. Set **`servers[0].url`** to **`https://…/api`** (no trailing slash) so OpenAPI paths **`/v1/…`** resolve correctly. Product-specific steps: [OPENAPI.md](integration/OPENAPI.md).

---

## See also

- [UNLOCK-AND-BEARER.md](UNLOCK-AND-BEARER.md) — **init / unlock**, **`session_token`**, **Bearer** vs **`gvmcp`**, security notes  
- [integration/Claude-mcp.md](integration/Claude-mcp.md) — **`gvmcp`**, stdio vs **`-listen`**, env vars  
- [integration/OPENAPI.md](integration/OPENAPI.md) — **`servers.url`**, Bearer, ngrok / Tailscale Funnel detail  
- [OVERVIEW.md](OVERVIEW.md) — product scope and limits  
