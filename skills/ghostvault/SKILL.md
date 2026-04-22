---
name: ghostvault
description: >-
  Use the Ghost Vault MCP tools (memory_search, memory_save, memory_stats) to recall and store
  durable user memory. Invoke when the user asks to remember something, recall prior facts, or
  ground answers in saved context. Assumes Ghost Vault MCP is already connected; no setup steps.
---

# Ghost Vault MCP (using memory tools)

**What this is:** Ghost Vault is the **retrieval and persistence** layer. The **host** (Claude, Cursor, etc.) **decides** when to call tools — possibly several times per turn and alongside other tools. *One-shot* use is a single **`memory_search`** then answer; *agentic* use is deliberate multi-step search/save. **Call MCP tools like any other tool** — do not shell out to `gvctl` unless the user wants CLI/API instead.

## Tools ↔ HTTP (for mental model)

| Tool | HTTP | Role |
|------|------|------|
| **`memory_search`** | `POST /v1/retrieve` | Hybrid retrieve (semantic + lexical + fusion). |
| **`memory_save`** | `POST /v1/ingest` | Ingest text and/or messages; optional **`infer`**, **`session_key`**. |
| **`memory_stats`** | `GET /v1/stats` | Chunk counts, ingest activity (vault implied by server session). |

Names align with **`operationId`** in the repo’s `openapi/openapi.yaml`.

## Defaults: `vault_id`, `user_id`, sessions

- **`vault_id`:** UUID from vault unlock. If you send **`"default"`** or omit it, a correctly configured **`gvmcp`** fills **`GHOSTVAULT_DEFAULT_VAULT_ID`** / **`-default-vault-id`**.
- **`user_id`:** Logical scope inside the vault (profile, thread, org convention). Omit only when **`GHOSTVAULT_DEFAULT_USER_ID`** is set on the host. **Wrong `user_id` → no hits** for data saved under another scope.
- **Vault bearer (`gvmcp` → `gvsvd`):** Separate from any OAuth the MCP client used to reach **`gvmcp`**. If tools return **401** on vault routes, the **host** must refresh the vault session (unlock / token file) — not fixable by changing tool arguments.

## When to call what

- **`memory_search`** — When the answer may depend on **stored** facts. **Before** heavy reasoning, one **clear** query (keywords + short paraphrase). Cap size with **`max_chunks`** / **`max_tokens`** if the host limits tool output.
- **`memory_save`** — User asked to **remember**, or you are **committing a durable fact**. Prefer **`idempotency_key`** on retries. **`infer: true`** only when you want server-side **extraction** into short facts (requires LLM on **`gvsvd`**); by default extracted strings go to **abstract** only — use **`infer_target: "body"`** to put them in the body field, or use structured fields / **`source_document`** (below) for long-form + slices.
- **`memory_stats`** — After **empty** search: read **`meta.hint`** first, then stats to see if the vault is empty, **`user_id`** is wrong, or you should rephrase. **Avoid blind retry loops.**

## `memory_search` — inputs

- **`query`** (required): Keywords + short paraphrase — not a full chat paste.
- **`max_chunks`** / **`max_tokens`** (optional): Bound snippet volume.
- **`content_mode`** (optional): **`auto`** (default) — returns short **abstract** as primary `text` when the row has one, else full body; **`abstract`**, **`full`**, **`both`**, or **`default`**. Each hit can include **`abstract`**, **`full_text`**, **`full_available`**, **`kind`** when present. Use **`full`** or **`both`** when you need the long form in the tool result; otherwise follow up with **`GET /v1/chunks/{id}`** (same vault) for the full record.
- **`vault_id`** / **`user_id`:** As above.
- **`session_key`** + **`session_mode`:** **`all`** (default), **`only`** (only chunks ingested with that key — requires **`session_key`**), **`prefer`** (boost after hybrid scoring). **`session_key`** must match **`memory_save`** for that conversation.

**Response:** JSON with **`results`** and **`meta`** (`result_count`, `hint` when there are no hits). **`meta.hint`** is there to cut empty-result loops — follow it (broaden query, fix scope, **`memory_stats`**, or **`memory_save`**).

## `memory_save` — inputs

- **`text`:** Raw prose to chunk and store (legacy path; ~2000-rune windows with overlap when not using structured fields).
- **`abstract`**, **`kind`**, **`metadata`:** Optional top-level structured single row (with **`text`** as body) when not using **`items`**.
- **`items`:** Array of `{ abstract, text, body, kind, metadata }` for multiple memories in one call. At least one of **abstract** or **text**/**body** per item (non-empty).
- **`source_document`:** `{ text, title? }` — stores **one** full document; any **`items`** entry with **no** per-item **body**/**text** links to that blob (no duplicate full text per slice). Use this for many abstracts over the same long note.
- **`infer`:** If true, server runs LLM extraction on recent session + **`text`**; each line goes to **abstract** unless **`infer_target`** is **`body`**.
- **`infer_target`:** **`abstract`** (default) or **`body`** — where each extracted string is stored.
- **`messages`** + **`session_key`:** Turn context; **`session_key`** tags chunks for later **`session_mode`** **`only`** / **`prefer`** on search. Server default for key is often **`default`** if omitted.
- **`idempotency_key`**, **`vault_id`**, **`user_id`:** As above.

## `memory_stats`

No arguments needed for typical calls. Use counts and recent activity to debug **empty** **`memory_search`**.

## Practices (host rules)

- **Thrash control:** Cap searches per user turn (e.g. 2–3) unless the first call failed for transport.
- **Scope:** Stable **`user_id`** across save and search for the same person/context.
- **Grounding:** Use retrieved chunks when relevant; if nothing fits, say so.
- **Alternative partitioning:** Without **`session_key`**, you can still scope by convention using **`user_id`** per thread (e.g. `profile:thread-abc`) — same filtering mechanics as any other **`user_id`**.

**Custom GPT / Actions:** Same tool names and JSON shapes as in the imported OpenAPI spec; mirror this guidance in the GPT **Instructions** field.

**Wiring MCP** (env, tokens, Cursor/Claude config): see repo **`docs/integration/Claude-mcp.md`**, **`CURSOR.md`**, **`OPENAPI.md`** — not repeated here.

**Vocabulary:** **RAG** (retrieve-augmented generation) usually means *retrieve stored chunks, then answer with them*. **CAG** is not standardized—often “context-augmented” or “cache-augmented” generation. For Ghost Vault, treat **`memory_search`** as retrieval and **`memory_save`** as what makes later retrieval possible; the tool behavior matters more than the acronym.
