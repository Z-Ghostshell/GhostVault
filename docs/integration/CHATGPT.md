# ChatGPT integration

Use Ghost Vault from **ChatGPT** through a **Custom GPT** with **Actions** (OpenAPI import). When the model calls a tool, **OpenAI’s servers** invoke your **public HTTPS** Ghost Vault URL — the browser and desktop apps do not call your laptop directly.

---

## Step 1 — OpenAPI integration (shared)

Complete **[OpenAPI integration](./OPENAPI.md)** first: expose **your Ghost Vault server URL** over **HTTPS** (tunnel or hosted), set `servers[0].url` in [`openapi/openapi.yaml`](../../openapi/openapi.yaml) to that origin, and obtain a **Bearer** token after unlock ([Get a Bearer token](./OPENAPI.md#get-a-bearer-token-ghost-vault)). Copy from [`openapi/openapi.example.yaml`](../../openapi/openapi.example.yaml) if you do not have a working `openapi.yaml` yet.

---

## Step 2 — Custom GPT with Actions

**How Actions work (official):** OpenAI documents GPT Actions — OpenAPI schemas, authentication, and examples — in [**Getting started with GPT Actions**](https://developers.openai.com/api/docs/actions/getting-started) and the [**GPT Actions** introduction](https://developers.openai.com/api/docs/actions/introduction).

Once that Custom GPT exists on your account, you can use it from **chatgpt.com (web)**, the **ChatGPT desktop app**, and **mobile** — same login, same GPT. Actions are **account-level**; there is no separate web-only vs desktop-only integration.

**Additional prerequisites:**

- **ChatGPT account** that can **create and use Custom GPTs** (capability depends on your OpenAI plan).

### Create the Custom GPT (usually in the browser)

OpenAI’s **GPT builder** runs in the **web** UI (you typically author the GPT in a browser, even if you later chat from the desktop or mobile app).

1. Go to ChatGPT → **Explore GPTs** → **Create** (wording may vary).
2. Name and describe the GPT (e.g. “Uses my Ghost Vault memory”).
3. Open **Configure** → **Actions** → **Create new action**.
4. **Import** `openapi/openapi.yaml` (or paste its contents). After you change `servers[0].url` or your tunnel host, **re-import** or update the action so ChatGPT keeps calling the right host.
5. Under **Authentication**, choose **API Key** / **Bearer** (or equivalent) and paste the **Ghost Vault session or actions token** ([Get a Bearer token](./OPENAPI.md#get-a-bearer-token-ghost-vault)).  
   - Custom GPTs store this server-side; anyone with access to the GPT configuration can see it. Use a **dedicated vault** or **rotating** token policy if others share the workspace.

### Use it from the ChatGPT **web** app

**Yes — the web app is a first-class way to use Ghost Vault with ChatGPT**, and it uses the **same** Custom GPT and Actions as everywhere else.

1. Open **chatgpt.com** (or your regional ChatGPT URL) and sign in with the **same account** that owns the Ghost Vault Custom GPT.
2. From the **sidebar** or GPT picker, select **your** Ghost Vault GPT (not only the default “ChatGPT” model unless your GPT is pinned as default).
3. Chat as usual. When the model calls **memory_search** / **memory_save**, OpenAI’s servers call your tunnel URL — no browser plugin and no direct connection from your PC to Ghost Vault for Actions.

**Important:** Ghost Vault tools run only when you are in a conversation with **that** Custom GPT (or a clone that imports the same Actions). The standard chat without selecting your GPT does **not** automatically attach your OpenAPI Actions.

### ChatGPT **desktop** app (optional)

1. Install and sign in to the **ChatGPT** desktop app with the **same account** that owns the Custom GPT.
2. Open the **sidebar** / GPT picker and select **your** Ghost Vault GPT.
3. Chat as usual; when the model calls tools, OpenAI’s servers call your tunnel URL.

You do **not** need extra “desktop-only” connector software.

### ChatGPT **mobile**

On **iOS/Android** ChatGPT apps, sign in with the same account, pick your Ghost Vault Custom GPT from the GPT list, and use it like on web — same Actions behavior.

---

## Troubleshooting (ChatGPT)

| Issue | What to check |
|--------|----------------|
| “Failed to contact API” / tool errors | Tunnel URL, TLS cert, firewall; `curl` healthz from outside your LAN ([OpenAPI integration](./OPENAPI.md)). |
| 401 on tools | Expired session token; unlock again and refresh Bearer in **Actions** authentication. |
| Wrong host in errors | `servers.url` in `openapi.yaml` must match the tunnel; re-import the schema in ChatGPT after edits. |
| ngrok hostname changed after restart | Update `servers[0].url` and GPT Actions; use a static ngrok domain on a paid plan if you need a stable URL. |
| Works on web, not desktop (or the reverse) | Same account? App updated? Try signing out and back in. |
| Tools never run on web | Are you chatting **inside** your Ghost Vault Custom GPT, not the default assistant without that GPT selected? |

---

## See also

- [OPENAPI.md](./OPENAPI.md) — HTTPS, OpenAPI `servers`, Bearer token (step 1)  
- [deploy.md](../deploy.md) — tunnels and deploy  
- [`openapi/openapi.yaml`](../../openapi/openapi.yaml) — spec to import into Actions  
