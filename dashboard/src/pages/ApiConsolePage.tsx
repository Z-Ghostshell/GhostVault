import { Send } from 'lucide-react'
import { useEffect, useState } from 'react'
import {
  apiFetch,
  apiGetJson,
  fetchErrorMessage,
  getStoredBearer,
  getStoredVaultId,
  setStoredVaultId,
} from '../api/client'
import type { VaultStatsResponse } from '../api/types'
import { gits } from '../theme/gvUi'

const METHODS = ['GET', 'POST', 'DELETE'] as const

const DEFAULT_RETRIEVE_BODY =
  '{\n  "vault_id": "",\n  "user_id": "default",\n  "query": "test"\n}'

/** If JSON has `vault_id` empty/missing, fill from session-bound vault (does not overwrite a non-empty value). */
function mergeVaultIdIntoBody(jsonStr: string, vaultId: string | null): string {
  if (!vaultId?.trim()) return jsonStr
  try {
    const o = JSON.parse(jsonStr) as Record<string, unknown>
    if (!('vault_id' in o)) return jsonStr
    const cur = o.vault_id
    if (cur !== '' && cur != null) return jsonStr
    o.vault_id = vaultId.trim()
    return JSON.stringify(o, null, 2)
  } catch {
    return jsonStr
  }
}

export default function ApiConsolePage() {
  const [method, setMethod] = useState<(typeof METHODS)[number]>('POST')
  const [path, setPath] = useState('/v1/retrieve')
  const [body, setBody] = useState(() =>
    mergeVaultIdIntoBody(DEFAULT_RETRIEVE_BODY, getStoredVaultId()),
  )
  const [status, setStatus] = useState<number | null>(null)
  const [responseText, setResponseText] = useState('')
  const [pending, setPending] = useState(false)
  const [err, setErr] = useState<string | null>(null)

  useEffect(() => {
    const onVaultId = () => {
      const vid = getStoredVaultId()
      if (vid?.trim()) setBody((b) => mergeVaultIdIntoBody(b, vid))
    }
    window.addEventListener('ghostvault-vault-id-changed', onVaultId)
    return () => window.removeEventListener('ghostvault-vault-id-changed', onVaultId)
  }, [])

  useEffect(() => {
    if (!getStoredBearer()?.trim()) return
    if (getStoredVaultId()?.trim()) return
    let cancelled = false
    void (async () => {
      try {
        const s = await apiGetJson<VaultStatsResponse>('/v1/stats')
        if (cancelled) return
        setStoredVaultId(s.vault_id)
        setBody((b) => mergeVaultIdIntoBody(b, s.vault_id))
      } catch {
        // e.g. env-seeded token before vault id cached
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

  const send = async () => {
    setErr(null)
    setResponseText('')
    setStatus(null)
    if (!getStoredBearer()?.trim()) {
      setErr('Set a Bearer token in the header first.')
      return
    }
    const p = path.startsWith('/') ? path : `/${path}`
    setPending(true)
    try {
      const init: RequestInit = { method }
      if (method !== 'GET' && body.trim() !== '') {
        const merged = mergeVaultIdIntoBody(body, getStoredVaultId())
        init.body = merged
        if (merged !== body) setBody(merged)
      }
      const res = await apiFetch(p, init)
      setStatus(res.status)
      const text = await res.text()
      try {
        const parsed = JSON.parse(text) as unknown
        setResponseText(JSON.stringify(parsed, null, 2))
      } catch {
        setResponseText(text)
      }
    } catch (e) {
      setErr(fetchErrorMessage(e))
    } finally {
      setPending(false)
    }
  }

  return (
    <div>
      <div className="mb-6">
        <h1 className={gits.pageTitle}>API console</h1>
        <p className={gits.pageSub}>
          Send authenticated requests to gvsvd through the Vite proxy (same Bearer as Overview). Paths are usually under{' '}
          <code className="text-slate-400">/v1/…</code>.
        </p>
      </div>

      {err ? <div className={`mb-4 ${gits.configErrBanner}`}>{err}</div> : null}

      <div className={`${gits.card} space-y-4 p-4`}>
        <div className="flex flex-wrap items-center gap-2">
          <select
            value={method}
            onChange={(e) => setMethod(e.target.value as (typeof METHODS)[number])}
            className={`${gits.inputSm} font-mono`}
          >
            {METHODS.map((m) => (
              <option key={m} value={m}>
                {m}
              </option>
            ))}
          </select>
          <input
            value={path}
            onChange={(e) => setPath(e.target.value)}
            placeholder="/v1/retrieve"
            className={`min-w-[12rem] flex-1 ${gits.input} font-mono text-xs`}
            spellCheck={false}
          />
          <button
            type="button"
            disabled={pending}
            onClick={() => void send()}
            className={`inline-flex items-center gap-2 ${gits.btnGhost}`}
          >
            <Send className="h-4 w-4" />
            Send
          </button>
        </div>
        <div>
          <div className={gits.hudLabel + ' mb-1'}>Body (JSON)</div>
          <textarea
            value={body}
            onChange={(e) => setBody(e.target.value)}
            rows={12}
            disabled={method === 'GET'}
            className="w-full resize-y rounded-lg border border-slate-700/80 bg-slate-950/80 p-3 font-mono text-xs text-slate-100 focus:border-cyan-500/40 focus:outline-none focus:ring-1 focus:ring-cyan-500/30 disabled:opacity-50"
            spellCheck={false}
          />
        </div>
        <div>
          <div className="mb-1 flex items-center gap-3">
            <span className={gits.hudLabel}>Response</span>
            {status !== null ? (
              <span className="font-mono text-xs text-slate-400">
                HTTP {status}
              </span>
            ) : null}
          </div>
          <pre className="max-h-[28rem] overflow-auto rounded-lg border border-slate-800/90 bg-slate-950/90 p-3 font-mono text-xs text-slate-200">
            {responseText || '—'}
          </pre>
        </div>
      </div>
    </div>
  )
}
