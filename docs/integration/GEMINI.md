# Gemini integration

The **[Google Gemini](https://gemini.google.com/app)** web app does **not** support registering **custom tools** from your own **OpenAPI** spec (there is no ChatGPT-style “import Actions URL” flow there).

Use Ghost Vault with Gemini in one of these ways instead:

- **[MCP integration](./Claude-mcp.md)** — Run **`gvmcp`** with an MCP-capable host (for example **Gemini CLI**); Ghost Vault stays behind the same REST contract the MCP server calls.
- **OpenAPI integration** — Follow **[OPENAPI.md](./OPENAPI.md)** for a public HTTPS base URL and Bearer token, then use **Gemini API [function calling](https://ai.google.dev/gemini-api/docs/function-calling)** with a small gateway that calls Ghost Vault (see **[OpenAPI baseline](#openapi-baseline-shared)** and **[Function calling](#function-calling-same-api-contract)** below). Shapes match [`openapi/openapi.yaml`](../../openapi/openapi.yaml).
- **Your own chat app** — Build a client or backend that calls the **Gemini API**, runs the tool loop, and forwards tool calls to Ghost Vault (same gateway idea as OpenAPI integration).

---

## OpenAPI baseline (shared)

Complete **[OpenAPI integration](./OPENAPI.md)** first: a **public HTTPS** **Ghost Vault server URL**, `servers[0].url` in [`openapi/openapi.yaml`](../../openapi/openapi.yaml) aligned with that origin, and a **Bearer** token after unlock ([Get a Bearer token](./OPENAPI.md#get-a-bearer-token-ghost-vault)). Copy from [`openapi/openapi.example.yaml`](../../openapi/openapi.example.yaml) if you need a template spec.

---

## Function calling (same API contract)

**Official guide:** Google documents tools and the request/response loop in [**Gemini API — Function calling**](https://ai.google.dev/gemini-api/docs/function-calling) (if the URL moves, search Google’s developer docs for “Gemini function calling”).

### Gemini API — function calling (recommended)

1. **Implement a small backend** (Cloud Run, your laptop + tunnel for tests, etc.) that:
   - Receives **function calls** from the Gemini API (`memory_search` / `memory_save` or names you define).
   - Calls Ghost Vault’s **`POST /v1/retrieve`** and **`POST /v1/ingest`** at your HTTPS base URL with the **Bearer** token.

2. In your Gemini request, declare **functions** whose parameters mirror the JSON bodies you send to Ghost Vault (`vault_id`, `user_id`, query text, chunk content, etc. — see the OpenAPI spec).

3. Run the usual **tool loop**: model returns function calls → your code executes them against Ghost Vault → you return structured results to the model.

If your gateway runs on the same machine as the tunnel (**ngrok**, **Tailscale Funnel**, etc.), it can call the same **`https://…` origin** as in the [OpenAPI baseline](#openapi-baseline-shared); use that base URL and Bearer on every request.

### Google AI Studio

You can prototype **system instructions + tools** in [Google AI Studio](https://aistudio.google.com/) against your **Gemini API key**, but **tool execution** still happens in **your** code: Studio does not replace the need for a backend that forwards to Ghost Vault unless you use a bridge that explicitly supports arbitrary HTTPS APIs.

### Enterprise (Gemini / Vertex)

**Gemini Enterprise** and related **custom connector** docs describe indexing external systems for search — a different shape than request-time memory with `memory_search` / `memory_save`. If you already use those products, align with your admin’s connector model; for the Ghost Vault REST contract, a **gateway** that maps connector or search actions to `/v1/retrieve` and `/v1/ingest` is still the straightforward approach.

---

## Troubleshooting (Gemini)

| Issue | What to check |
|--------|----------------|
| No custom OpenAPI in the Gemini web app | Expected: use **[Claude-mcp.md](./Claude-mcp.md)**, **OpenAPI + function calling** (this page), or **your own chat app** (see [introduction](#gemini-integration)). |
| Gateway cannot reach Ghost Vault | Tunnel URL, TLS, firewall; `curl` healthz from outside your LAN ([OpenAPI integration](./OPENAPI.md)). |
| 401 from Ghost Vault | Expired Bearer; unlock again and refresh the token your gateway sends on `Authorization`. |
| Functions never invoked | Prompt clarity, function names/descriptions, model version support for tools (see [function calling docs](https://ai.google.dev/gemini-api/docs/function-calling)). |
| Timeouts | Reduce payload size or split ingest; check Gemini and your gateway timeouts. |

---

## See also

- [Google Gemini (web app)](https://gemini.google.com/app) — consumer chat UI (`gemini.google.com`)  
- [Claude-mcp.md](./Claude-mcp.md) — `gvmcp`, Gemini CLI, and other MCP hosts  
- [OPENAPI.md](./OPENAPI.md) — HTTPS, OpenAPI `servers`, Bearer token (shared baseline)  
- [CHATGPT.md](./CHATGPT.md) — same REST contract via ChatGPT Actions (for comparison)  
- [`openapi/openapi.yaml`](../../openapi/openapi.yaml) — reference for request bodies and operations  
- [Gemini API — Function calling](https://ai.google.dev/gemini-api/docs/function-calling) — official tool loop  
