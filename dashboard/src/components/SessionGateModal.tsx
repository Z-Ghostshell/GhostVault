import { Unlock } from 'lucide-react'
import { useEffect, useId, useRef, useState } from 'react'
import { apiGetJson, setStoredBearer, setStoredVaultId, unlockVault } from '../api/client'
import type { VaultStatsResponse } from '../api/types'
import { gits } from '../theme/gvUi'

type Props = {
  open: boolean
  /** When true, user must complete paste or unlock — no dismiss without a session. */
  forced: boolean
  onClose: () => void
}

export default function SessionGateModal({ open, forced, onClose }: Props) {
  const titleId = useId()
  const dialogRef = useRef<HTMLDivElement>(null)
  const [tokenDraft, setTokenDraft] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)

  useEffect(() => {
    if (!open) {
      setErr(null)
      return
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !forced) onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, forced, onClose])

  useEffect(() => {
    if (open) {
      dialogRef.current?.focus()
    }
  }, [open])

  if (!open) return null

  /** Persist token and reload so all routes refetch with the new Bearer. */
  const afterSessionSaved = () => {
    window.location.reload()
  }

  const saveToken = () => {
    const v = tokenDraft.trim()
    if (!v) {
      setErr('Paste a session token or unlock with your vault password.')
      return
    }
    setErr(null)
    setTokenDraft('')
    void (async () => {
      setStoredVaultId(null)
      setStoredBearer(v)
      try {
        const stats = await apiGetJson<VaultStatsResponse>('/v1/stats')
        setStoredVaultId(stats.vault_id)
      } catch {
        // Invalid token or gvsvd unreachable — leave vault_id unset
      }
      afterSessionSaved()
    })()
  }

  const doUnlock = async () => {
    setErr(null)
    setBusy(true)
    try {
      const trimmed = password.trim()
      const data = await unlockVault(trimmed === '' ? undefined : trimmed)
      setStoredBearer(data.session_token)
      setStoredVaultId(data.vault_id)
      setPassword('')
      afterSessionSaved()
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'Unlock failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div
      className="fixed inset-0 z-[200] flex items-center justify-center bg-black/75 p-4 backdrop-blur-md"
      role="presentation"
      onClick={forced ? undefined : () => onClose()}
    >
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        className={`${gits.card} max-h-[90vh] w-full max-w-lg overflow-y-auto p-6 shadow-[0_0_60px_-12px_rgba(34,211,238,0.25)]`}
        onClick={(e) => e.stopPropagation()}
        onKeyDown={(e) => e.stopPropagation()}
      >
        <h2 id={titleId} className="text-xl font-semibold tracking-tight text-slate-100">
          Sign in to Ghost Vault
        </h2>
        <p className="mt-2 text-sm text-slate-400">
          The dashboard needs a session token for <code className="text-slate-500">/v1</code> routes. Paste a token from{' '}
          <code className="text-slate-500">gvctl unlock</code>, or unlock with your vault password (encryption on requires a
          password).
        </p>

        {err ? <div className={`mt-4 ${gits.configErrBanner}`}>{err}</div> : null}

        <div className="mt-6 space-y-3">
          <label className="block">
            <span className={gits.hudLabel}>Session token (Bearer)</span>
            <textarea
              value={tokenDraft}
              onChange={(e) => setTokenDraft(e.target.value)}
              placeholder="Paste session_token…"
              rows={3}
              autoComplete="off"
              className={`mt-2 w-full resize-y ${gits.input} font-mono text-xs`}
              spellCheck={false}
            />
          </label>
          <button
            type="button"
            onClick={saveToken}
            disabled={busy}
            className={`w-full ${gits.btnGhost}`}
          >
            Save session
          </button>
        </div>

        <div className="relative my-6">
          <div className="absolute inset-0 flex items-center">
            <div className="w-full border-t border-slate-700/80" />
          </div>
          <div className="relative flex justify-center text-xs uppercase tracking-wider text-slate-500">
            <span className="bg-slate-950 px-2">or unlock</span>
          </div>
        </div>

        <div className="space-y-3">
          <label className="block">
            <span className={gits.hudLabel}>Vault password</span>
            <input
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="Empty if GV_ENCRYPTION=off"
              disabled={busy}
              className={`mt-2 w-full ${gits.input}`}
            />
          </label>
          <button
            type="button"
            onClick={() => void doUnlock()}
            disabled={busy}
            className="inline-flex w-full items-center justify-center gap-2 rounded-lg border border-cyan-500/40 bg-cyan-950/50 py-2.5 text-sm font-medium text-cyan-100 hover:bg-cyan-950/70 disabled:opacity-50"
          >
            {busy ? (
              'Unlocking…'
            ) : (
              <>
                <Unlock className="h-4 w-4" />
                Unlock vault
              </>
            )}
          </button>
        </div>

        {!forced ? (
          <div className="mt-6 flex justify-end border-t border-slate-800/80 pt-4">
            <button
              type="button"
              onClick={onClose}
              className="text-sm text-slate-400 hover:text-slate-200"
            >
              Cancel
            </button>
          </div>
        ) : (
          <p className="mt-6 border-t border-slate-800/80 pt-4 text-center text-xs text-slate-500">
            You can’t use the dashboard until a session is saved.
          </p>
        )}
      </div>
    </div>
  )
}
