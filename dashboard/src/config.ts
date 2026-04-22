/** API path prefix without trailing slash. Empty = same-origin `/v1` (Vite proxy to gvsvd). */
export function getApiBase(): string {
  const raw = import.meta.env.VITE_API_BASE_URL as string | undefined
  if (raw == null || raw === '') return ''
  return raw.replace(/\/$/, '')
}

export function getApiDisplayLabel(): string {
  const b = getApiBase()
  return b === '' ? 'same-origin (/v1 → proxy)' : b
}

const RETRIEVE_DEBUG_LS = 'ghostvault-retrieve-debug-nav'

/** True when the Retrieve debug feature exists in this build (dev or VITE_SHOW_RETRIEVE_DEBUG). */
export function isRetrieveDebugPageEnabled(): boolean {
  if (import.meta.env.DEV) {
    return true
  }
  return import.meta.env.VITE_SHOW_RETRIEVE_DEBUG === 'true'
}

/** Sidebar link: same availability as the page, minus explicit opt-out (localStorage). */
export function isRetrieveDebugNavVisible(): boolean {
  if (!isRetrieveDebugPageEnabled()) {
    return false
  }
  return localStorage.getItem(RETRIEVE_DEBUG_LS) !== '0'
}

export function setRetrieveDebugOptIn(enabled: boolean): void {
  localStorage.setItem(RETRIEVE_DEBUG_LS, enabled ? '1' : '0')
  window.dispatchEvent(new Event('ghostvault-retrieve-debug-changed'))
}

export function getRetrieveDebugOptIn(): boolean {
  return localStorage.getItem(RETRIEVE_DEBUG_LS) !== '0'
}
