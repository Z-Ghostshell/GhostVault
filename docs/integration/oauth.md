# OAuth and Claude custom connectors (Ghost Vault)

**MCP vs connector, OAuth facade, “three different secrets,” and end-to-end flow** live in **[Claude-connector.md](./Claude-connector.md)** (see [§ 3 — Three different secrets](Claude-connector.md#3-three-different-secrets-do-not-confuse-them) and [§ 4 — OAuth on `gvmcp`](Claude-connector.md#4-oauth-on-gvmcp-architecture-in-one-paragraph)). **This file** is **IdP-specific recipes** (Auth0, ZITADEL, verification, troubleshooting)—not a second copy of the architecture.

**Env vars and `gvmcp` OAuth implementation:** [Claude-mcp.md — OAuth 2.0 for Claude](./Claude-mcp.md#oauth-20-for-claude-custom-connector-auth). **One-page integration pointer:** [CLAUDE.md](./CLAUDE.md).

---

## When you need OAuth vs path token only

- **Path token only** (typical): You expose **`https://your-host/mcp-<token>/`**, set **`GV_MCP_PATH_TOKEN`**, keep **`GHOSTVAULT_OAUTH_INTROSPECTION_URL`** **unset**. In **Add custom connector**, **OAuth Client ID** and **OAuth Client Secret** are usually **left empty** (unless your org forces otherwise). This is a **capability URL** model: anyone with the URL can hit **`gvmcp`**; protect the URL like a password.
- **OAuth on `gvmcp`**: You run an **authorization server**, set **`GHOSTVAULT_OAUTH_*`** per [Claude-mcp.md](./Claude-mcp.md), and Claude sends **`Authorization: Bearer`** on each MCP request. Use this when you need **standard OAuth**, **per-user identity** from the IdP, or **defense beyond a static URL**.

You can use **both** path token **and** OAuth (obscure URL plus token validation).

---

## Where “OAuth Client ID” and “OAuth Client Secret” come from

- **Not** from Ghost Vault and **not** from a Claude download.
- They are the **client credentials** of an OAuth/OIDC **application** you create in **your** identity provider (Auth0, Okta, Keycloak, Zitadel, Azure AD, …), configured for **Claude’s redirect** and (if required) **confidential** client authentication.
- Paste them into **Customize → Connectors → Add custom connector → Advanced** **only** when your IdP and Anthropic’s flow expect that client for the hosted connector.

Official background: [Authentication for connectors](https://claude.com/docs/connectors/building/authentication), [Get started with custom connectors using remote MCP](https://support.claude.com/en/articles/11175166-get-started-with-custom-connectors-using-remote-mcp). Register hosted callback **`https://claude.ai/api/mcp/auth_callback`** as documented there.

If you **cannot** expose Dynamic Client Registration (DCR) or Client ID Metadata (CIMD), Anthropic documents **`oauth_anthropic_creds`**: contact **`mcp-review@anthropic.com`** with your client id/secret per the same authentication doc.

---

## Auth0 (common pitfall)

Auth0’s wizard often steers you toward **Machine to Machine** and **Auth0 Management API**. That combination is for **server-to-server calls to Auth0’s admin API** — **not** for “Claude signs in and gets a token for my MCP.”

**Recommended direction:**

1. **APIs → Create API** — define **your** resource. The **Identifier** must equal **`GHOSTVAULT_OAUTH_AUDIENCE`** (which defaults to **`GHOSTVAULT_OAUTH_RESOURCE`**). Auth0 compares this string **byte-for-byte** — a trailing **`/`** vs no slash are two different identifiers. This is **not** “Auth0 Management API.”
2. **Applications → Create Application** — choose an interactive type suitable for **authorization code** / redirect flows (often **Regular Web Application**; follow Auth0’s current docs for OAuth redirect clients). On **Advanced → Grant Types**, enable `authorization_code` + `refresh_token`.
3. **Allowed Callback URLs** — include **`https://claude.ai/api/mcp/auth_callback`** (plus any others Auth0 or Anthropic require). Add this on the fallback confidential client **even if DCR is on**, because the connector UI accepts a pre-registered client_id.
4. **Authorize** the application to use **your** API from step 1 (scopes as you define them), **not** the Management API unless you have a separate admin use case.
5. **Dynamic Client Registration (optional but recommended):** **Tenant Settings → Advanced → OIDC Dynamic Application Registration = ON** and `default_directory` pointing at a database connection. Open DCR means Claude can POST **`/oidc/register`** without a token; the facade's **`/register`** endpoint forwards there.
6. Map Auth0’s **issuer**, **token introspection** URL, and **client** used for introspection (if confidential) into **`GHOSTVAULT_OAUTH_*`** on **`gvmcp`** per [Claude-mcp.md](./Claude-mcp.md). Set **`GHOSTVAULT_OAUTH_AUDIENCE`** to the API identifier from step 1 so the facade injects it into `/authorize` and `/token`.

Exact clicks change with Auth0’s UI; treat the above as **intent**, not a screenshot-level tutorial.

### The built-in facade (Claude-bug workaround)

When OAuth is enabled, **`gvmcp`** serves these paths on the MCP host:

| Path | Purpose | Upstream default |
|------|---------|------------------|
| `GET /.well-known/oauth-authorization-server` | RFC 8414 metadata that advertises the **facade** endpoints (all on the MCP host). | — |
| `GET /authorize` | 302 redirect to the upstream AS. **Does not forward** any client `audience` query param; always **`Set`s** `audience` to **`GHOSTVAULT_OAUTH_AUDIENCE`** when that env is set (avoids Claude sending a mismatched audience, e.g. missing trailing slash). | `{issuer}authorize` |
| `POST /token` | Reverse-proxy to the upstream token endpoint. **`Del`s** then **`Set`s** `audience` to the configured value. Forwards `Authorization` for confidential clients. | `{issuer}oauth/token` |
| `POST /register` | Reverse-proxy to Open DCR (Auth0's **`/oidc/register`**). Relays status + JSON verbatim. | `{issuer}oidc/register` |

Override the upstream URLs via **`GHOSTVAULT_OAUTH_UPSTREAM_AUTHORIZE_URL`**, **`GHOSTVAULT_OAUTH_TOKEN_URL`**, **`GHOSTVAULT_OAUTH_REGISTRATION_URL`** if your IdP does not use Auth0's path layout.

### Post-deploy sanity check

```bash
# Metadata should point back at the MCP host (not Auth0):
curl -sS https://YOUR-HOST/.well-known/oauth-authorization-server | jq .

# /authorize must 302 to Auth0; Location must use YOUR canonical audience (even if you pass audience=WRONG):
curl -sSI "https://YOUR-HOST/authorize?response_type=code&client_id=YOUR_CLIENT_ID&redirect_uri=https%3A%2F%2Fclaude.ai%2Fapi%2Fmcp%2Fauth_callback&code_challenge=abc&code_challenge_method=S256&state=s&audience=WRONG" \
  | grep -i '^location:'
# Decode the Location URL and confirm audience= matches GHOSTVAULT_OAUTH_AUDIENCE exactly.

# If DCR is on, Claude can register a fresh client without credentials:
curl -sS -X POST https://YOUR-HOST/register \
  -H 'content-type: application/json' \
  -d '{"client_name":"test","redirect_uris":["https://claude.ai/api/mcp/auth_callback"]}' \
  | jq .client_id
```

---

## Troubleshooting and learnings (Auth0 + connector)

These notes come from real integration runs; see [Claude-connector.md](./Claude-connector.md) for the combined guide and verification commands.

### What “working” looks like

- **`client_credentials`** (or authorization code + `/token` via the facade) succeeds against Auth0 with **`audience`** equal to your API **Identifier**.
- **`curl -sSI 'https://YOUR-HOST/authorize?...&audience=WRONG'`** returns a **`Location:`** to `https://TENANT.us.auth0.com/authorize?...` where **`audience=`** is **only** your **`GHOSTVAULT_OAUTH_AUDIENCE`** value (the facade must not forward `WRONG`).
- After login, **`claude.ai/api/mcp/auth_callback?code=...`** includes a **`code`** query param (not **`error=access_denied`**).

### Trailing slash vs no trailing slash (most common “Service not found”)

Auth0 treats the API **Identifier** as an **exact** string. These are **different** audiences:

- `https://host/mcp-<token>`  
- `https://host/mcp-<token>/`

Claude and some clients derive **`audience`** from the MCP **resource** URL and may **omit** the trailing slash. Auth0’s error text often shows the audience **without** a final `/` even when your `.env` used one.

**Practical fix that has worked in production:**

1. Pick **one** canonical string (often **no** trailing slash after the path token — it matches what appears in Auth0’s “Service not found” message).
2. Set **Auth0 → APIs → your API → Identifier** to that **exact** string.
3. Set **`GHOSTVAULT_OAUTH_RESOURCE`** and **`GHOSTVAULT_OAUTH_AUDIENCE`** in **`.env`** to the **same** string.
4. Re-run a **`client_credentials`** (or introspection) test with that **`audience`**.
5. **`docker compose up -d --build gvmcp`** after any **`.env`** or code change.

The **connector URL** in Claude can still end with **`/`** (e.g. `https://host/mcp-<token>/`) — that is the **HTTP path**. The **OAuth** resource / audience strings in **`.env`** are separate and must match Auth0’s Identifier **exactly**.

### Other pitfalls

- **Do not use** Auth0’s built-in **Management API** identifier (`https://TENANT.us.auth0.com/api/v2/`) as your MCP **audience**. Create **APIs → Create API** for Ghost Vault MCP. The Management API is for M2M admin calls, not end-user tokens to your MCP.
- **`Service not found: https://…`** means no API **Identifier** equals the **`audience`** Auth0 received. Align Auth0 + **`.env`**; verify with the **`curl … authorize … audience=WRONG`** check above.
- If **`client_credentials`** works but the **browser** flow still failed **before** upgrading **`gvmcp`**, Claude was likely sending a **wrong** `audience` on `/authorize`. Current **`gvmcp`** **drops** any incoming `audience` on `/authorize` and **`Del`s + `Set`s** `audience` on **`POST /token`** so only **`GHOSTVAULT_OAUTH_AUDIENCE`** is sent upstream — **rebuild and redeploy** **`gvmcp`**.
- **`code: Field required`** from Claude’s API after redirect means the callback had **`error=access_denied`** (no **`code`**). Fix Auth0 / audience first, then **disconnect** and **re-add** the connector in Claude.
- **API → Allow Offline Access:** **ON** if you want **`refresh_token`**; pair with **Application → Grant Types** (**Refresh Token**) and the app **authorized** for that API.
- **Auth0-only dashboard changes** apply immediately; **`docker compose` restart** is needed when you change **`.env`**, **edge/nginx**, or the **`gvmcp`** image.

---

## ZITADEL (self-hosted OIDC)

Field notes from integrating **ZITADEL** with the hosted connector and **`gvmcp`** (see also [Claude-connector.md — Self-hosted OIDC](./Claude-connector.md#5-self-hosted-oidc-eg-zitadel)):

### Discovery and `.env`

1. Fetch **`https://<your-zitadel-host>/.well-known/openid-configuration`** and copy **`issuer`**, **`authorization_endpoint`**, **`token_endpoint`**, and **`introspection_endpoint`**.
2. Set **`GHOSTVAULT_OAUTH_AUTH_SERVER_ISSUER`** to **`issuer`**.
3. Set **`GHOSTVAULT_OAUTH_INTROSPECTION_URL`** to **`introspection_endpoint`** (commonly **`…/oauth/v2/introspect`**).
4. **Required:** set **`GHOSTVAULT_OAUTH_UPSTREAM_AUTHORIZE_URL`** and **`GHOSTVAULT_OAUTH_TOKEN_URL`** to the discovery authorize and token URLs. ZITADEL uses **`/oauth/v2/authorize`** and **`/oauth/v2/token`** — **not** **`gvmcp`’s** Auth0-shaped defaults (`{issuer}authorize`, `{issuer}oauth/token`).
5. If discovery omits **`registration_endpoint`**, there is **no** Open DCR for the facade’s **`POST /register`** to proxy to. Use a **pre-registered** client and **Claude → Advanced** Client ID / Secret; do not assume **`/register`** works.

### Two applications in ZITADEL

| Application | Typical type | Client ID / secret used where |
|-------------|--------------|-------------------------------|
| **Claude / user login** | **Web** (or **User Agent**), authorization code + **PKCE**, redirect **`https://claude.ai/api/mcp/auth_callback`** | **Claude** custom connector **Advanced** (secret only if that app has one, e.g. not PKCE-only public client). |
| **Introspection** | **API** (machine), **Basic** or **private_key_jwt** per instance | **`GHOSTVAULT_OAUTH_INTROSPECTION_CLIENT_ID`** / **`SECRET`** in **`.env`** on the host that runs **`gvmcp`** only. |

Swapping these (e.g. putting the **API** app’s Client ID in the browser authorize URL) produces **`invalid_request`** / **`Errors.App.NotFound`** on **`/oauth/v2/authorize`**.

### Logout URL

**`post_logout_redirect_uri`** on the ZITADEL application is where the **browser lands after logout**. It is **not** the **`end_session_endpoint`** from discovery. Use **`https://`** (or enable **Development mode** for `http://` redirects during local dev).

### Facade `audience` parameter

**`gvmcp`** injects **`audience`** on **`/authorize`** and **`POST /token`** from **`GHOSTVAULT_OAUTH_AUDIENCE`** (defaults to **`GHOSTVAULT_OAUTH_RESOURCE`**). That mirrors **Auth0** “API identifier” behavior. ZITADEL may ignore or reject unknown parameters depending on version; if flows fail at authorize or token, check ZITADEL logs and project/app settings.

### Introspection smoke test

Load **Ghost Vault’s** **`.env`** (not another repo’s **direnv**), then:

```bash
cd /path/to/GhostVault
set -a && . ./.env && set +a
curl -sS -X POST "$GHOSTVAULT_OAUTH_INTROSPECTION_URL" \
  -u "${GHOSTVAULT_OAUTH_INTROSPECTION_CLIENT_ID}:${GHOSTVAULT_OAUTH_INTROSPECTION_CLIENT_SECRET}" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data "token=invalid&token_type_hint=access_token"
```

Expect **`{"active":false}`** for a bogus token; **`active":true`** for a real access token after Claude login.

### OAuth vs vault session (401 on `/v1/*`)

**Claude’s** access token is validated **only on `gvmcp`**. Requests **`gvmcp` → `gvsvd`** still use the **Ghost Vault session** bearer (**`.ghostvault-bearer`** / **`GHOSTVAULT_BEARER_TOKEN`**). After **`gvsvd`** or stack restarts, refresh the vault token (**`gvctl unlock`**, **`make rotate-token`**, etc.) or MCP tools return **401** from **`gvsvd`** even when the connector is “connected.”

### Deploy note

After changing **`GHOSTVAULT_OAUTH_*`** in **`.env`**, run **`docker compose up -d --force-recreate gvmcp`** so the container picks up new values.

Official docs: [ZITADEL OIDC endpoints](https://zitadel.com/docs/apis/openidoauth/endpoints), [token introspection](https://zitadel.com/docs/guides/integrate/token-introspection).

---

## Other platforms (Keycloak, Okta, Azure AD, …)

Same structure as ZITADEL where applicable:

1. Read **OIDC discovery** and align **`GHOSTVAULT_OAUTH_*`** (especially **non-Auth0** authorize/token paths via **`GHOSTVAULT_OAUTH_UPSTREAM_AUTHORIZE_URL`** / **`GHOSTVAULT_OAUTH_TOKEN_URL`**).
2. Define a **resource** / audience behavior your IdP expects (may differ from Auth0’s API Identifier model).
3. Create a **user-facing OAuth client** with **Claude’s redirect URI** registered; use a **separate** confidential client for **introspection** when required.
4. Enable **token introspection** (RFC 7662) if **`gvmcp`** validates tokens that way.

---

## Is path token alone “safe enough”?

For **solo / low exposure** use, **HTTPS + `GV_MCP_PATH_TOKEN` + server-side vault bearer + rotation if leaked** is a **reasonable** model documented as the practical default when OAuth is not enabled.

**Limitations:** URLs leak via logs, history, screenshots, and referrals easier than headers. OAuth adds **identity** and **standards-based** protection on the **Claude → `gvmcp`** hop. See [Claude-mcp.md — Hosted custom connector](./Claude-mcp.md#hosted-custom-connector) (security rationale) and the OAuth section linked above.

---

## See also

- [Claude-connector.md](./Claude-connector.md) — **combined** Claude + hosted connector + OAuth guide  
- [Claude-mcp.md](./Claude-mcp.md) — hosted connector steps, **`GHOSTVAULT_OAUTH_*`**, rotation  
- [CLAUDE.md](./CLAUDE.md) — short pointers into [Claude-connector.md](./Claude-connector.md)  
- [deploy.md](../deploy.md) — edge, Funnel, public HTTPS  
