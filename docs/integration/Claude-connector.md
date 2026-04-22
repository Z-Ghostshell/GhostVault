# Claude + Ghost Vault: hosted connector and OAuth

This guide is the **single entry point** for using Ghost Vault with **Claude** when you care about the **hosted custom connector** (claude.ai / Claude Desktop / **Claude mobile** sync) and optional **OAuth** (Auth0, **ZITADEL**, other OIDC providers). It combines what used to live in [CLAUDE.md](./CLAUDE.md) (MCP vs connector) and [oauth.md](./oauth.md) (OAuth, facade, IdP-specific notes).

**Deeper reference:** [Claude-mcp.md](./Claude-mcp.md) (all transports, env vars, Desktop JSON), [deploy.md](../deploy.md) (Compose, Funnel, tunnels), [OPENAPI.md](./OPENAPI.md) (REST Bearer, same vault).

---

## 1. Two ways into Claude (not interchangeable)

Ghost Vault reaches Claude through **`gvmcp`**, which exposes MCP tools **`memory_search`** and **`memory_save`** and forwards them to **`gvsvd`**.

| | **MCP** (stdio or remote bridge) | **Hosted custom connector** |
|---|----------------------------------|-----------------------------|
| **Who runs the client** | Your machine (Claude Desktop, Cursor, Claude Code) or local **`npx mcp-remote`** | **Anthropic’s servers** call your URL from the cloud |
| **Network** | Loopback, tailnet, or **you** point **`mcp-remote`** at HTTPS | **Public HTTPS** must reach your host (e.g. **Tailscale Funnel**, ngrok). Tailnet-only **Serve** is not enough for claude.ai |
| **Where you configure** | **`claude_desktop_config.json`**, Cursor MCP JSON, or Claude Code MCP settings | **Customize → Connectors → Add custom connector** on claude.ai / Desktop (syncs across surfaces) |
| **Secrets** | Bearer + API URL on the **`gvmcp`** process (host env or Compose); Desktop may only pass the MCP URL | Same vault Bearer on **server `gvmcp`**; connector stores **one HTTPS URL**. Optional OAuth client id/secret in Advanced. Ghost Vault uses a **path token** (`GV_MCP_PATH_TOKEN`) so the MCP URL is not guessable |
| **Best for** | Daily dev, IDE workflows, air-gapped or tailnet-only setups | Using Ghost Vault from **claude.ai**, **Claude Desktop**, and the **Claude iOS / Android app** with one registration (connector syncs across surfaces) |

**Local (stdio):** Claude Desktop or Cursor **spawns** `bin/gvmcp`; set **`GHOSTVAULT_BASE_URL`**, **`GHOSTVAULT_BEARER_TOKEN`** (or token file), optional defaults in MCP **`env`**. No public URL required.

**Remote MCP bridge:** **`gvmcp`** on a server at **`https://…/mcp-<token>/`**. On Desktop you can use **`npx mcp-remote`** to that URL instead of the hosted connector — **your** machine initiates traffic, not Anthropic’s cloud. Full steps: [Claude-mcp.md](./Claude-mcp.md) §5 and [`configs/`](../../configs/).

**This doc focuses on the hosted connector** (one URL, Anthropic’s cloud as client).

---

## 2. Hosted custom connector: what you run

Register **one** URL: **`https://YOUR-HOST/mcp-<GV_MCP_PATH_TOKEN>/`** under **Customize → Connectors → Add custom connector**. Anthropic runs the MCP session; you do **not** add a duplicate **`mcpServers`** entry in **`claude_desktop_config.json`** for the same vault if you only want this path.

**Requirements:**

- **`make up`** with the **mcp** profile; **`GV_MCP_PATH_TOKEN`** in **`.env`**.
- Valid vault session for **`gvmcp`** (e.g. **`.ghostvault-bearer`** / **`make rotate-token`** after **`gvsvd`** restarts).
- **Public HTTPS** (e.g. Tailscale **Funnel** to edge `:8989`) so Anthropic can reach you. **POST** must work with and without a trailing slash (handled in [edge nginx templates](../../edge/nginx.full.conf.template)).

**Security:** Funnel exposes **`/mcp-<token>/`** without per-request headers; the path segment is a capability URL. Optional OAuth adds **`Authorization: Bearer`** on the Claude → `gvmcp` hop — see below.

More detail: [Claude-mcp.md — Hosted custom connector](./Claude-mcp.md#hosted-custom-connector).

---

## 3. Three different secrets (do not confuse them)

| Layer | What it is | Where it lives |
|-------|------------|----------------|
| **Path token** | **`GV_MCP_PATH_TOKEN`** — secret segment in the public MCP URL | **`.env`**, edge nginx; **not** the OAuth client id |
| **Vault session** | **`GHOSTVAULT_BEARER_TOKEN`** / **`.ghostvault-bearer`** — **`gvmcp` → `gvsvd`** | **Server** running **`gvmcp`** (Compose mount, env) |
| **OAuth (optional)** | Access token **Claude → `gvmcp`** after login at **your** IdP | Issued by Auth0 (etc.); validated by **`gvmcp`** when **`GHOSTVAULT_OAUTH_*`** is set |

**Path token only:** leave **`GHOSTVAULT_OAUTH_INTROSPECTION_URL`** unset; connector Advanced OAuth fields often empty. **Path token + OAuth:** keep both — obscure URL plus bearer validation.

---

## 4. OAuth on `gvmcp`: architecture in one paragraph

Ghost Vault does **not** mint OAuth tokens. **Your IdP** (Auth0, …) issues access tokens; **`gvmcp`** validates them with **RFC 7662 introspection** when **`GHOSTVAULT_OAUTH_INTROSPECTION_URL`** is set. **`gvsvd`** still uses **vault session** Bearer (**`gvmcp` → `gvsvd`**).

**Claude-specific bug / facade:** claude.ai ignores RFC 9728 **`authorization_servers`** and builds **`/authorize`**, **`/token`**, **`/register`**, **`/.well-known/oauth-authorization-server`** on the **MCP host** ([anthropics/claude-ai-mcp#82](https://github.com/anthropics/claude-ai-mcp/issues/82)). **`gvmcp`** therefore serves a small **facade**: metadata on the MCP host; **`GET /authorize`** 302s to the **upstream IdP** with **`audience=`** (when configured); **`POST /token`** and **`POST /register`** proxy to the IdP. **Default** upstream paths match **Auth0** (`{issuer}authorize`, `{issuer}oauth/token`, `{issuer}oidc/register`). **ZITADEL** and many others use different paths — set **`GHOSTVAULT_OAUTH_UPSTREAM_AUTHORIZE_URL`** and **`GHOSTVAULT_OAUTH_TOKEN_URL`** explicitly (see §5). Without the facade, **`GET /authorize`** 404s at nginx.

| Path on MCP host | Role |
|------------------|------|
| `GET /.well-known/oauth-authorization-server` | RFC 8414 metadata (endpoints on this host) |
| `GET /.well-known/oauth-protected-resource` | RFC 9728 resource metadata |
| `GET /authorize` | 302 to upstream authorize URL + inject **`audience`** |
| `POST /token` | Proxy to upstream token URL + inject **`audience`** |
| `POST /register` | Proxy to upstream DCR URL (default Auth0 **`/oidc/register`**) |

Env vars: [Claude-mcp.md — OAuth 2.0 for Claude](./Claude-mcp.md#oauth-20-for-claude-custom-connector-auth). Official connector auth: [Authentication for connectors](https://claude.com/docs/connectors/building/authentication).

---

## 5. Self-hosted OIDC (e.g. ZITADEL)

Lessons from running **ZITADEL** (and similar **non-Auth0** servers) with the hosted connector:

1. **Path layout:** Read **`/.well-known/openid-configuration`** on your IdP. ZITADEL commonly exposes **`authorization_endpoint`** and **`token_endpoint`** under **`/oauth/v2/…`**. **`gvmcp`** defaults assume Auth0-style **`/authorize`** and **`/oauth/token`** — you **must** set **`GHOSTVAULT_OAUTH_UPSTREAM_AUTHORIZE_URL`** and **`GHOSTVAULT_OAUTH_TOKEN_URL`** to those discovery values, and **`GHOSTVAULT_OAUTH_AUTH_SERVER_ISSUER`** to the discovery **`issuer`**.
2. **Introspection:** Set **`GHOSTVAULT_OAUTH_INTROSPECTION_URL`** to the discovery **`introspection_endpoint`** (ZITADEL: typically **`…/oauth/v2/introspect`**). If the endpoint requires auth, set **`GHOSTVAULT_OAUTH_INTROSPECTION_CLIENT_ID`** / **`SECRET`** (e.g. **Basic** auth per ZITADEL docs).
3. **Two OAuth applications in the IdP (do not swap IDs):**
   - **User / browser client** (e.g. ZITADEL **Web**, authorization code + **PKCE**, redirect **`https://claude.ai/api/mcp/auth_callback`**). Its **Client ID** (and secret **only if** that app has one) goes in **Claude → connector → Advanced**.
   - **Introspection / machine client** (e.g. ZITADEL **API**, client credentials / Basic). Its **Client ID + Secret** go **only** in **`.env`** as **`GHOSTVAULT_OAUTH_INTROSPECTION_*`** for **`gvmcp`**. Using the machine client’s ID in the browser **`/authorize`** flow yields ZITADEL **`Errors.App.NotFound`** (or similar).
4. **No dynamic registration:** If discovery has **no `registration_endpoint`**, Claude cannot rely on **`oauth_dcr`**; use a **pre-registered** client and **Advanced** credentials. **`POST /register`** on the MCP host may fail upstream — that is separate from a successful login if Anthropic does not need DCR.
5. **Logout URI:** **`post_logout_redirect_uri`** in the IdP app is the **page after logout** (e.g. `https://claude.ai/`), **not** the IdP’s **`end_session_endpoint`**. Use **HTTPS** unless the app’s **Development mode** allows `http://`.
6. **`audience`:** The facade always injects **`audience`** from **`GHOSTVAULT_OAUTH_AUDIENCE`** (Auth0-style). Some IdPs ignore extra parameters; if authorize or token fails, check IdP logs and consider aligning resource settings or a future **`gvmcp`** option to omit **`audience`**.
7. **After `.env` OAuth changes:** `docker compose up -d --force-recreate gvmcp` (and fix **`gvsvd`** if Compose recreates it) so the container sees new variables.
8. **OAuth success ≠ vault session:** Claude’s token is validated on **`gvmcp`** only. **`gvsvd`** still requires the **vault session** bearer (**`.ghostvault-bearer`** / **`gvctl unlock`**). If **`gvsvd`** restarts or the file is empty, MCP tools return **401** on **`/v1/*`** even when the connector shows “connected.”

More detail: [oauth.md — ZITADEL](./oauth.md#zitadel-self-hosted-oidc).

---

## 6. Auth0: what is “on Auth0” vs “on your machine”

**Auth0 hosts (fixed URLs for your tenant):**

- Issuer: `https://YOUR-TENANT.us.auth0.com/`
- **`/authorize`**, **`/oauth/token`**, **`/oauth/introspect`**, **`/oidc/register`**

**Ignore** the built-in **Management API** identifier `https://YOUR-TENANT.us.auth0.com/api/v2/` — that is Auth0’s **admin** API, not your MCP resource.

**You create inside Auth0:**

1. **APIs → Create API** — **Identifier** must equal **`GHOSTVAULT_OAUTH_AUDIENCE`** (defaults to **`GHOSTVAULT_OAUTH_RESOURCE`**, e.g. `https://YOUR-HOST/mcp-<token>/`). This string is the OAuth **`audience`**; it does not need to be a live HTTP endpoint. **Signing:** RS256.
2. **Applications → Application** (e.g. Regular Web) — **Allowed Callback URLs:** `https://claude.ai/api/mcp/auth_callback`. **Advanced → Grant Types:** `authorization_code` + `refresh_token`. **APIs** tab: **Authorize** this app for the API from step 1.
3. **(Optional)** **Tenant Settings → Advanced → OIDC Dynamic Application Registration = ON** if you want **`/register`** to create clients.

**API settings worth toggling:**

- **Allow Offline Access = ON** if you want refresh tokens (pairs with grant type `refresh_token` on the app). Otherwise users re-auth when the access token expires.
- **RBAC / Permissions:** leave off unless you set **`GHOSTVAULT_OAUTH_SCOPES`** in `.env` for scope enforcement.

**Machine to Machine** tab on the API is for **`client_credentials`** — not the Claude browser flow.

---

## 7. Verification commands (after Auth0 or `.env` changes)

**Auth0-only dashboard changes** take effect immediately — **no Docker restart** unless you changed **`.env`**.

```bash
# RFC 8414 metadata — issuer should be your MCP host origin (not your IdP host)
curl -sS https://YOUR-HOST/.well-known/oauth-authorization-server | jq .

# /authorize should 302 to your IdP (Auth0, ZITADEL /oauth/v2/authorize, …) with audience= matching GHOSTVAULT_OAUTH_AUDIENCE
curl -sSI "https://YOUR-HOST/authorize?response_type=code&client_id=YOUR_CLAUDE_WEB_APP_CLIENT_ID&redirect_uri=https%3A%2F%2Fclaude.ai%2Fapi%2Fmcp%2Fauth_callback&code_challenge=abc&code_challenge_method=S256&state=s" \
  | grep -i '^location:'

# Optional: DCR (Auth0 Open DCR; often N/A on ZITADEL)
curl -sS -X POST https://YOUR-HOST/register \
  -H 'content-type: application/json' \
  -d '{"client_name":"test","redirect_uris":["https://claude.ai/api/mcp/auth_callback"]}' \
  | jq .client_id
```

**Open the `Location:` URL from the second command in a browser.** You should see your IdP’s login page (Auth0, ZITADEL, …). Use the **user-facing** app’s **`client_id`** in the test URL — not the introspection machine client. If you see **`Service not found`** on **Auth0**, the **API Identifier** does not match the **`audience`** the facade sends (character-for-character, including trailing slash).

**Restart only when:**

| Change | Command |
|--------|---------|
| **`.env`** (`GHOSTVAULT_OAUTH_*`, etc.) | `docker compose up -d --force-recreate gvmcp` |
| **edge nginx template** | `docker compose up -d --force-recreate edge` |
| **Go code / image** | `docker compose up -d --build gvmcp` |

---

## 8. Troubleshooting (field notes)

| Symptom | Likely cause | Fix |
|---------|----------------|-----|
| **`error=access_denied`** and **`Service not found: https://…/mcp-…`** on `claude.ai/.../auth_callback` | No Auth0 **API** with **Identifier** equal to **`GHOSTVAULT_OAUTH_AUDIENCE`** / **`audience`** query param | **APIs → Create API** with that Identifier; authorize the OAuth **Application** for that API |
| **`code: Field required`** (Claude) after the above | OAuth failed before Auth0 issued a code; Claude got `error` not `code` | Fix Auth0 first, then **disconnect** and **re-add** the connector |
| Auth0 error shows audience **without** trailing slash; `.env` has **with** slash; **`client_credentials`** test works | Claude’s `/authorize` query included a **wrong** `audience` (e.g. missing `/`); the facade used to forward it verbatim | **Rebuild `gvmcp`** from current `main`: **`gvmcp`** now **always** sets `audience` to **`GHOSTVAULT_OAUTH_AUDIENCE`** on `/authorize` and `/token`, overriding the client |
| **`Callback URL mismatch`** | Missing callback on the **Application** | Add `https://claude.ai/api/mcp/auth_callback` to Allowed Callback URLs |
| **404** on `/authorize` | Facade not deployed or edge not proxying | Rebuild **`gvmcp`** + **edge**; confirm [nginx templates](../../edge/nginx.full.conf.template) proxy `/authorize` to `gvmcp:3751` |
| **401** on MCP tools after login | Introspection fails **or** (common) **vault session** missing/stale — **`gvsvd`** does not accept Claude’s OAuth token | Check **`GHOSTVAULT_OAUTH_INTROSPECTION_*`**; refresh **`.ghostvault-bearer`** (`gvctl unlock` / **`make rotate-token`**) after **`gvsvd`** restarts; see §3 and §5 |
| **`Errors.App.NotFound`** on ZITADEL **`/oauth/v2/authorize`** | **`client_id`** is the **introspection / API** app, not the **Web / Claude** app | Put the **user-facing** app’s Client ID in **Claude Advanced**; keep introspection ID **only** in **`.env`** |
| **`Location:`** still points at **Auth0** (or wrong host) after IdP switch | **`gvmcp`** not recreated or wrong **`GHOSTVAULT_OAUTH_*`** in the **container** | `docker compose up -d --force-recreate gvmcp`; confirm **`UPSTREAM_*`** and **`TOKEN_URL`** for ZITADEL |
| **“Authorization with the MCP server failed”** (Anthropic) after Auth0 consent | **`gvmcp`** rejected the access token: introspection missing **`exp`**, or **`exp`** encoded as a JSON float so parsing failed | Current **`gvmcp`** reads **`exp`** flexibly and falls back to the JWT payload when introspection omits **`exp`**. Rebuild **`gvmcp`**; if it persists, **`docker logs gvmcp`** during connect and test introspection manually with the access token |
| Stale session | **`gvsvd` restart** invalidates **`.ghostvault-bearer`** | `gvctl unlock` / `make rotate-token`; restart **`gvmcp`** if it reads the file |

---

## 9. Where “OAuth Client ID / Secret” in Claude come from

They are the **user-facing OAuth application** in **your IdP** (Auth0, ZITADEL Web app, …), not Ghost Vault. Paste into **Add custom connector → Advanced** when you use a pre-registered client. **Introspection** should use a **separate** machine client in **`.env`** on **`gvmcp`** — do not paste that client’s ID into Claude; see [Claude-mcp.md](./Claude-mcp.md) table and §5 above.

If DCR is enabled, Claude may register its own client; you still often keep a fallback client for the Advanced fields.

---

<a id="claude-mobile"></a>

## 10. Mobile: Claude for iOS and Android

The **hosted custom connector** is not desktop-only. After you register your **remote MCP** URL on **claude.ai** or **Claude Desktop**, Anthropic **syncs** that connector to the **same signed-in account** on the **Claude mobile app** — there is no separate “phone-only” MCP install.

**What to expect**

- **No local `gvmcp` on the phone.** Mobile does not run **stdio** MCP like Claude Desktop; it uses **remote MCP** only — Anthropic’s cloud calls your **HTTPS** endpoint (same as the hosted connector in this doc).
- **Add or edit the connector on web or Desktop first.** Product UIs change; if a control is missing on mobile, use **claude.ai** or **Claude Desktop**, then confirm the connector appears in the app after sync ([Connectors FAQ — mobile](https://claude.com/docs/connectors/faq)).
- **Per-conversation tools.** Enable or pick the connector in chat if the app shows a **Connectors** / **+** / tools control (labels vary by version).
- **Team / Enterprise.** Org-wide connectors may require an admin to allow them before individuals can add or use them ([FAQ](https://claude.com/docs/connectors/faq)).

**Ghost Vault constraints:** your **`gvmcp`** URL must stay reachable from **Anthropic’s servers** (public **HTTPS**), not only from your phone. A **tailnet-only** or **local stdio** setup is the Desktop path; **Claude on a phone** still needs the **remote** MCP registration above.

---

## 11. See also

- [oauth.md](./oauth.md) — OAuth-only details, other IdPs, path-token-only mode  
- [Claude-mcp.md](./Claude-mcp.md) — full **`GHOSTVAULT_OAUTH_*`** list, Desktop **`mcp-remote`**, rotation  
- [deploy.md](../deploy.md) — Funnel vs Serve, ports  
- [OPENAPI.md](./OPENAPI.md) — REST + Bearer (same vault)  
- [Anthropic — Connectors](https://support.claude.com/en/articles/11176164-use-connectors-to-extend-claude-s-capabilities)
