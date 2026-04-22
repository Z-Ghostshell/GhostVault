# OpenAPI integration (step 1: HTTP API & HTTPS)

This is the **shared foundation** for HTTP tool integrations: Ghost Vault’s **REST API** with **Bearer** authentication, described in [`openapi/openapi.yaml`](../../openapi/openapi.yaml) (copy from [`openapi/openapi.example.yaml`](../../openapi/openapi.example.yaml) if you do not have a local file yet).

**Ghost Vault server URL:** Set **`servers[0].url`** in that OpenAPI file to the **HTTP origin** clients will use — the same logical host as **`GHOSTVAULT_BASE_URL`** for **`gvctl`** / **`gvmcp`**. It can be local loopback during dev, a **public `https://` tunnel**, or another reachable origin ([deploy.md](../deploy.md)).

**Step 2** depends on the product you use:

- **[ChatGPT integration](./CHATGPT.md)** — Custom GPT → **Actions** imports this OpenAPI spec; OpenAI’s servers call your URL.
- **[Gemini integration](./GEMINI.md)** — **Gemini API function calling** (or a gateway) mirrors the same operations and request bodies as the spec.
- **[Cursor integration](./CURSOR.md)** — **MCP** via **`gvmcp`** (recommended), plus project rules/skills and the **OpenAPI** contract for raw HTTP; Cursor does not use OpenAPI import for built-in tools the way ChatGPT Actions does.
- **[Agent skill (gvctl)](./skills/README.md)** — copy **`skills/ghostvault/`** into your project; same **`SKILL.md`** for Cursor or Claude Code.

Remote callers need a **public `https://` URL** (tunnel or hosted deploy). Unlock the vault and use the **session or actions token** as `Authorization: Bearer …`. Tunnels and ports: [deploy.md — Expose `gvsvd` over HTTPS](../deploy.md#expose-gvsvd-over-https-chatgpt-actions-remote-testing).

---

## Prerequisites

- **Ghost Vault (`gvsvd`) running** at **your Ghost Vault server URL** — a **`https://` origin** when a hosted vendor calls you (ChatGPT Actions), or whatever HTTP(S) origin your **Gemini gateway** / **`gvctl`** session uses to reach **`gvsvd`**.
- **OpenAPI schema**: [`openapi/openapi.yaml`](../../openapi/openapi.yaml) — copy from [`openapi/openapi.example.yaml`](../../openapi/openapi.example.yaml) if needed; set `servers[0].url` to that HTTPS base (no trailing slash beyond what your paths expect).
- **Bearer token** after unlock (see [Get a Bearer token](#get-a-bearer-token-ghost-vault) below).

---

## 1. Expose Ghost Vault over HTTPS

Point your tunnel or reverse proxy at **edge** (Compose default **`http://127.0.0.1:8989`**) or **whatever host:port `gvsvd` listens on** for your deploy. **Examples:** Docker Compose **`/api`** on **`8989`**; **`go run ./cmd/gvsvd`** is often **`http://127.0.0.1:8080`**. Use **ngrok** or **Cloudflare** — copy/paste commands in [deploy.md](../deploy.md#expose-gvsvd-over-https-chatgpt-actions-remote-testing), or follow the [ngrok integration](#ngrok-integration) checklist below. For **Tailscale**, use [Tailscale Funnel](#tailscale-funnel-integration) (public HTTPS). Set **`servers[0].url`** to the **`https://…/api`** origin (no trailing slash) when using edge. Other options: **Tailscale Serve** (tailnet-only; not enough for hosted ChatGPT Actions), or another TLS reverse proxy in front of **your Ghost Vault server URL**.

Confirm from the internet (Docker Compose **edge** prefixes REST with **`/api`**):

```bash
curl -sS "https://YOUR-TUNNEL.example.com/api/healthz"
```

If you expose **`gvsvd`** without edge (no **`/api`** prefix), use **`/healthz`** at that origin instead.

---

## 2. Point the OpenAPI spec at that URL

Edit `openapi/openapi.yaml`:

```yaml
servers:
  - url: https://YOUR-TUNNEL.example.com/api
```

Use the **same** origin your integration will call (with **`/api`** when using Docker **edge**; paths in the spec stay **`/v1/…`**). Omit **`/api`** only if you tunnel **`gvsvd`** directly.

---

## ngrok integration

Use this when you want a **public `https://` URL** to **`gvsvd`** on your machine for **ChatGPT Actions**, **Gemini gateways**, or any client that uses the same REST contract. Install and one-line tunnel details also live under [deploy.md — ngrok](../deploy.md#a-ngrok).

### 1. Install and sign in

1. Install the [ngrok agent](https://ngrok.com/download).
2. Add your authtoken once (from the [ngrok dashboard](https://dashboard.ngrok.com/get-started/your-authtoken)):
   ```bash
   ngrok config add-authtoken <YOUR_TOKEN>
   ```

### 2. Run Ghost Vault locally

Start **`gvsvd`** the way you usually do (Docker Compose or `go run`). Note the **host** port you publish to the machine running ngrok:

| How you run | Typical host URL |
|-------------|------------------|
| Docker Compose (edge) | `https://…/api` or tunnel **`8989`** (paths **`/api/v1/…`**) |
| `go run` / custom port | Match **`gvsvd`**’s host port |

### 3. Open the tunnel

Point ngrok at that **same** port (example for default Compose):

```bash
ngrok http 8989
```

In the ngrok terminal UI or [dashboard](https://dashboard.ngrok.com/), copy the **Forwarding** URL that starts with `https://` (e.g. `https://abc123.ngrok-free.app`). Use **only** the origin — scheme + host (and port if ngrok ever shows one) — **no** trailing slash for `servers.url`.

### 4. Apply the tunnel URL and continue

1. Set `servers[0].url` in [`openapi/openapi.yaml`](../../openapi/openapi.yaml) to that `https://…` origin (same as [section 2](#2-point-the-openapi-spec-at-that-url)).
2. [Unlock](#get-a-bearer-token-ghost-vault) the vault and configure your HTTP client with the **Bearer** token (session or actions token).
3. Finish **step 2** for your product: **[ChatGPT integration](./CHATGPT.md)** (re-import Actions when the host changes) or **[Gemini integration](./GEMINI.md)** (point your gateway at the same base URL).

### 5. Smoke-test from the internet

```bash
curl -sS "https://YOUR-SUBDOMAIN.ngrok-free.app/api/healthz"
```

Then exercise `POST /v1/vault/unlock` with the same base URL if you have not already.

### Free tier and changing hostnames

- **Browser interstitial (free):** ngrok may show a warning page for **browser** visits; **server-to-server** calls (e.g. OpenAI Actions) usually work unchanged. If tools fail in odd ways, try a **paid** [reserved domain](https://ngrok.com/docs/guides/how-to-set-up-a-custom-domain) or another tunnel (e.g. Cloudflare in [deploy.md](../deploy.md#b-cloudflare-tunnel-quick--ephemeral)).
- **URL changes:** On the free tier, restarting ngrok often yields a **new** hostname. After each change, update `servers[0].url` and refresh vendor config (for ChatGPT, re-import Actions — see [ChatGPT integration](./CHATGPT.md)). Paid plans can use a **static** domain so configuration stays stable.

---

## Tailscale Funnel integration

Use **[Tailscale Funnel](https://tailscale.com/kb/1223/tailscale-funnel)** when you already run **Tailscale** and want a **stable `https://` URL** on your tailnet’s DNS name (for example `https://your-machine.your-tailnet.ts.net`) without ngrok or Cloudflare. **OpenAI’s servers** must reach you over the public internet, so you need **Funnel**, not only [Tailscale Serve](https://tailscale.com/docs/features/tailscale-serve) (Serve is for other devices on your tailnet).

Official references: [Funnel overview](https://tailscale.com/kb/1223/tailscale-funnel), [`tailscale funnel` CLI](https://tailscale.com/kb/1311/tailscale-funnel), [examples](https://tailscale.com/docs/reference/examples/funnel).

### 1. Prerequisites (tailnet)

Funnel needs a recent **Tailscale** client (v1.38.3+), **[MagicDNS](https://tailscale.com/docs/features/magicdns)** on the tailnet, **[HTTPS / certs](https://tailscale.com/docs/how-to/set-up-https-certificates)** for your tailnet name, and permission to use Funnel (a **`funnel` [node attribute](https://tailscale.com/docs/reference/syntax/policy-file#node-attributes)** in policy). The first time you run `tailscale funnel`, Tailscale usually walks you through approval and policy updates; admins can also add Funnel under **Access controls** in the [admin console](https://login.tailscale.com/admin/acls).

### 2. Run Ghost Vault locally

Same as [ngrok integration](#ngrok-integration): default Compose edge **`8989`**, or your **`gvsvd`** port when not using edge.

### 3. Start Funnel to the API port

On the **same machine** as `gvsvd`, proxy the host port Ghost Vault listens on (Compose default):

```bash
tailscale funnel 8989
```

The command prints an **internet** URL, for example:

```text
Available on the internet:
https://your-machine.example-tailnet.ts.net
|-- / proxy http://127.0.0.1:8989
```

Use that **`https://…` origin** (no trailing slash) as `servers[0].url`. Add `--bg` if you want the configuration to **persist** across restarts until you turn it off (see [CLI docs](https://tailscale.com/kb/1311/tailscale-funnel)). To clear Funnel: `tailscale funnel reset`.

If you already use **Serve** on the same listener port, remember **Serve and Funnel cannot share the same port configuration**; whichever command you ran last wins for that port ([Funnel vs Serve](https://tailscale.com/docs/reference/funnel-vs-sharing)).

### 4. Apply the Funnel URL and continue

Same as [ngrok — Apply the tunnel URL](#4-apply-the-tunnel-url-and-continue): set [`openapi/openapi.yaml`](../../openapi/openapi.yaml) `servers[0].url` to the Funnel URL, [unlock](#get-a-bearer-token-ghost-vault), set **Bearer** auth on your client, then **[ChatGPT integration](./CHATGPT.md)** or **[Gemini integration](./GEMINI.md)** for product-specific steps.

### 5. Verify

From **outside** your LAN (or any machine that can resolve public DNS):

```bash
curl -sS "https://your-machine.example-tailnet.ts.net/api/healthz"
```

New Funnel hostnames can take a few minutes to appear in **public DNS**; if `curl` fails immediately, wait and retry (Tailscale documents delays on the order of **up to ~10 minutes** in some cases).

---

## Get a Bearer token (Ghost Vault)

**Encrypted vault (`GV_ENCRYPTION=on`):**

1. Call unlock **locally or via tunnel** (same API as in the spec):

   ```bash
   curl -sS -X POST "https://YOUR-TUNNEL.example.com/v1/vault/unlock" \
     -H "Content-Type: application/json" \
     -d '{"password":"YOUR_PASSPHRASE"}'
   ```

2. Copy `session_token` from the JSON response.
3. Use it as **`Authorization: Bearer …`** wherever your HTTP client needs it ([ChatGPT integration](./CHATGPT.md), [Gemini integration](./GEMINI.md), or other callers).

Session length follows `GV_SESSION_IDLE_MINUTES` and `GV_SESSION_MAX_HOURS` in `.env`. When the token expires, unlock again and **refresh** the token wherever it is configured.

**Plaintext vault (`GV_ENCRYPTION=off`):**

After unlock, you can mint a scoped token with `POST /v1/tokens/actions` (see spec) and use that as Bearer, or still use the session token — see [deploy.md](../deploy.md).

Easiest from the shell (any vault mode): **`gvctl unlock`** / **`gvctl unlock -token-only`** against your **`GHOSTVAULT_BASE_URL`**.

---

## Next step (product-specific)

- **[ChatGPT integration](./CHATGPT.md)** — Custom GPT → Actions (OpenAPI import), web / desktop / mobile.
- **[Gemini integration](./GEMINI.md)** — Gemini API function calling and gateways (same REST contract).

---

## Operation tips (HTTP tools)

- **Stable `vault_id` and `user_id`**: pass consistent identifiers in tool payloads so retrieval stays scoped (see operation descriptions in the OpenAPI file).
- **Tunnel uptime**: if the tunnel is down, hosted tools fail until the URL is reachable again.
- **Privacy**: only the **chunks you retrieve and send** in a given turn leave Ghost Vault in that request; each model provider may log tool I/O per its policies.
- **Token hygiene:** Store API keys and Bearer tokens only where appropriate; rotate if a key leaks.

---

## Troubleshooting

| Issue | What to check |
|--------|----------------|
| “Failed to contact API” / remote client cannot reach Ghost Vault | Tunnel URL, TLS cert, firewall; `curl` healthz from outside your LAN. |
| 401 on HTTP tools | Expired session token; unlock again and refresh Bearer everywhere it is configured. |
| Wrong host in errors | `servers.url` in `openapi.yaml` must match the tunnel; update vendor config after edits ([ChatGPT integration](./CHATGPT.md), [Gemini integration](./GEMINI.md)). |
| ngrok hostname changed after restart | Update `servers[0].url` and any imported Actions / gateway config; use a static ngrok domain on a paid plan if you need a stable URL. |
| Tailscale Funnel: “permission denied” / funnel won’t start | Tailnet policy must allow Funnel for your user/node (`funnel` node attribute); MagicDNS and HTTPS for the tailnet must be enabled. See [Tailscale Funnel troubleshooting](https://tailscale.com/kb/1223/tailscale-funnel#troubleshooting). |
| Tailscale URL not resolving yet | Public DNS for `*.ts.net` can lag; wait a few minutes and retry `curl` from an external network. |
| ChatGPT- or Gemini-specific behavior | See [ChatGPT integration](./CHATGPT.md#troubleshooting-chatgpt) and [Gemini integration](./GEMINI.md#troubleshooting-gemini). |

---

## See also

- [CHATGPT.md](./CHATGPT.md) — step 2 for ChatGPT (Custom GPT Actions)  
- [GEMINI.md](./GEMINI.md) — step 2 for Gemini (function calling)  
- [deploy.md](../deploy.md) — Docker, encryption flag, HTTPS tunnel checklist (includes ngrok install)  
- [Tailscale Funnel](https://tailscale.com/kb/1223/tailscale-funnel) — public HTTPS via your tailnet (this doc: [Tailscale Funnel integration](#tailscale-funnel-integration))  
- [Claude-connector.md](./Claude-connector.md) · [CLAUDE.md](./CLAUDE.md) · [Claude-mcp.md](./Claude-mcp.md) — Claude hosted connector, OAuth, and other MCP clients (`gvmcp`)  
- [`openapi/openapi.yaml`](../../openapi/openapi.yaml) — machine-readable contract for `memory_search` and `memory_save` (working copy you edit and import)  
- [`openapi/openapi.example.yaml`](../../openapi/openapi.example.yaml) — committed example you can copy to `openapi.yaml`  
