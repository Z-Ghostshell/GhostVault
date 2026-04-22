# Clarification questions

**Status:** decisions below are **answered** as of the latest product pass. Open items are listed at the bottom as **deferred**.

Canonical overview: [**OVERVIEW.md**](./OVERVIEW.md) · delivery: [**ROADMAP.md**](./ROADMAP.md) · GitS engine: [**gis/docs/design.md**](../../gis/docs/design.md).

## Trust and operations

1. **Who may ever see plaintext memory?**  
   **Answer:** Plaintext exists **only on the user’s device** (local trusted process after unlock). **Remote LLM hosts** see **only what is sent in that request**—the query and any retrieved snippets you include—not the full vault or disk.

2. **Primary user device**  
   **Answer:** **Desktop first** for v1. **Goal:** eventually support **desktop, mobile, and browser** surfaces for ChatGPT, Claude, etc.; ship desktop, then expand.

3. **Desktop or browser extension for key custody**  
   **Answer:** **Yes**—ship whatever extension or sidecar **makes sense** when custody or UX requires it.

## Blockchain and signup

4. **Chain ecosystem (L1/L2)**  
   **Answer:** **Not required** for the product spine. No mandatory on-chain anchor in v1.

5. **What goes on-chain at signup**  
   **Answer:** **Nothing required for now.** Signup/on-chain/DID details are **not** part of the initial slice; treat as **signup-time concerns later** if ever needed.

6. **Self-custody wallets vs email/custodial**  
   **Answer:** **Deferred** with other signup/auth productization. **Authentication** may use **wallet-style cryptographic lock-in** (e.g. MetaMask-class flows) where it helps **without** requiring a specific chain or on-chain registration in v1.

## Product and integrations

7. **v1 must-have host ordering**  
   **Answer:** **ChatGPT first.** Then Claude (MCP), Gemini, MCP-local, etc., in follow-on milestones.

8. **Offline use**  
   **Answer:** **Not required**—no need for memory to work fully offline except when calling a model.

9. **Persona agent**  
   **Answer:** **Deferred** (see “Deferred topics” below).

## Deferred topics (no decision needed yet)

- **Launch jurisdictions** (e.g. GDPR-heavy regions)
- **Consent model** for ingest (per-source vs global)
- **Revenue** model (self-host only vs hosted vs enterprise)

---

When primitives are locked (vector backend, exact crypto libs), add **`adr/`** notes or pointers from [**future/**](./future/).
