import { getApiBase } from '../config'

const TOKEN_KEY = 'ghostvault_dashboard_bearer'
const VAULT_ID_KEY = 'ghostvault_dashboard_vault_id'

const DEFAULT_FETCH_TIMEOUT_MS = 30_000

function timeoutSignal(userSignal: AbortSignal | undefined | null): AbortSignal | undefined {
  if (userSignal) return userSignal
  if (typeof AbortSignal !== 'undefined' && typeof AbortSignal.timeout === 'function') {
    return AbortSignal.timeout(DEFAULT_FETCH_TIMEOUT_MS)
  }
  return undefined
}

export function fetchErrorMessage(err: unknown): string {
  if (!(err instanceof Error)) return 'Network error'
  if (err.name === 'AbortError') {
    return 'Request timed out — is gvsvd running and reachable?'
  }
  return err.message
}

export function getStoredBearer(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function getStoredVaultId(): string | null {
  return localStorage.getItem(VAULT_ID_KEY)
}

export function setStoredVaultId(id: string | null): void {
  if (id && id.trim() !== '') localStorage.setItem(VAULT_ID_KEY, id.trim())
  else localStorage.removeItem(VAULT_ID_KEY)
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new Event('ghostvault-vault-id-changed'))
  }
}

export function setStoredBearer(token: string | null): void {
  if (token) localStorage.setItem(TOKEN_KEY, token)
  else {
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(VAULT_ID_KEY)
  }
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new Event('ghostvault-bearer-changed'))
    if (!token) {
      window.dispatchEvent(new Event('ghostvault-vault-id-changed'))
    }
  }
}

/** Dev-only: `VITE_GHOSTVAULT_BEARER_TOKEN` in `.env` seeds localStorage once (after setStoredBearer exists). */
function seedBearerFromEnv(): void {
  if (typeof window === 'undefined') return
  if (localStorage.getItem(TOKEN_KEY)) return
  const raw = import.meta.env.VITE_GHOSTVAULT_BEARER_TOKEN
  if (raw && String(raw).trim() !== '') {
    setStoredBearer(String(raw).trim())
  }
}
seedBearerFromEnv()

export interface UnlockResponse {
  session_token: string
  vault_id: string
  encryption_enabled: boolean
}

/** Routes that must not send Bearer (unlock / init). */
export async function apiFetchPublic(path: string, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers)
  if (!headers.has('Content-Type') && init.body != null && typeof init.body === 'string') {
    headers.set('Content-Type', 'application/json')
  }
  headers.delete('Authorization')
  const signal = timeoutSignal(init.signal)
  return fetch(buildUrl(path), { ...init, headers, signal })
}

async function readProblemDetail(res: Response): Promise<string> {
  const text = await res.text()
  try {
    const j = JSON.parse(text) as { detail?: string; title?: string }
    if (typeof j.detail === 'string' && j.detail !== '') return j.detail
    if (typeof j.title === 'string') return j.title
  } catch {
    // ignore
  }
  return text ? text.slice(0, 300) : res.statusText
}

/** POST /v1/vault/unlock — no Bearer. Password required when vault encryption is on. */
export async function unlockVault(password: string | undefined): Promise<UnlockResponse> {
  const body =
    password !== undefined && String(password).trim() !== ''
      ? JSON.stringify({ password: String(password).trim() })
      : JSON.stringify({})
  const res = await apiFetchPublic('/v1/vault/unlock', {
    method: 'POST',
    body,
  })
  if (!res.ok) {
    const detail = await readProblemDetail(res)
    throw new Error(detail)
  }
  return res.json() as Promise<UnlockResponse>
}

function buildUrl(path: string): string {
  if (path.startsWith('http://') || path.startsWith('https://')) return path
  const base = getApiBase()
  if (!path.startsWith('/')) return `${base}/${path}`
  return base ? `${base}${path}` : path
}

/** Fetch with Bearer from localStorage (unless Authorization already set). */
export async function apiFetch(path: string, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers)
  if (!headers.has('Content-Type') && init.body != null && typeof init.body === 'string') {
    headers.set('Content-Type', 'application/json')
  }
  const token = getStoredBearer()
  if (token && !headers.has('Authorization')) {
    headers.set('Authorization', `Bearer ${token}`)
  }
  const signal = timeoutSignal(init.signal)
  return fetch(buildUrl(path), { ...init, headers, signal })
}

export async function apiGetJson<T>(path: string): Promise<T> {
  const res = await apiFetch(path, { method: 'GET' })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(`${res.status} ${res.statusText}${text ? `: ${text.slice(0, 200)}` : ''}`)
  }
  return res.json() as Promise<T>
}
