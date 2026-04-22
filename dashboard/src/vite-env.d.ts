/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_BASE_URL?: string
  readonly VITE_DASHBOARD_BASE?: string
  readonly VITE_GHOSTVAULT_PROXY_URL?: string
  /** Optional dev: session token (same as .ghostvault-bearer). Inlined in dev bundle — localhost only. */
  readonly VITE_GHOSTVAULT_BEARER_TOKEN?: string
  /** When "true", show the Retrieve debugger nav item in production dashboard builds. Dev always shows it. */
  readonly VITE_SHOW_RETRIEVE_DEBUG?: string
}
