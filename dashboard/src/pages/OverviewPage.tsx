import { RefreshCw } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { apiGetJson, fetchErrorMessage } from '../api/client'
import type { VaultStatsResponse } from '../api/types'
import { gits } from '../theme/gvUi'

export default function OverviewPage() {
  const [data, setData] = useState<VaultStatsResponse | null>(null)
  const [err, setErr] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  const load = useCallback(async () => {
    setErr(null)
    setLoading(true)
    try {
      const j = await apiGetJson<VaultStatsResponse>('/v1/stats')
      setData(j)
    } catch (e) {
      setData(null)
      setErr(fetchErrorMessage(e))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  return (
    <div>
      <div className="mb-6 flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className={gits.pageTitle}>Overview</h1>
          <p className={gits.pageSub}>Vault memory chunks, ingest history, and session messages.</p>
        </div>
        <button
          type="button"
          onClick={() => void load()}
          disabled={loading}
          className={`inline-flex items-center gap-2 ${gits.btnGhostSm}`}
        >
          <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
          Refresh
        </button>
      </div>

      {err ? <div className={`mb-6 ${gits.configErrBanner}`}>{err}</div> : null}

      {loading && !data ? (
        <p className="text-slate-500">Loading…</p>
      ) : data ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <div className={gits.card + ' p-4'}>
            <div className={gits.hudLabel}>Vault</div>
            <div className="mt-2 break-all font-mono text-sm text-slate-200">{data.vault_id}</div>
            <div className="mt-2 text-sm text-slate-400">
              Encryption:{' '}
              <span className="text-cyan-300/90">{data.encryption_enabled ? 'on' : 'off'}</span>
            </div>
          </div>
          <div className={gits.card + ' p-4'}>
            <div className={gits.hudLabel}>Active chunks</div>
            <div className="mt-2 text-3xl font-semibold tabular-nums text-cyan-200">{data.chunks_total}</div>
          </div>
          <div className={gits.card + ' p-4'}>
            <div className={gits.hudLabel}>Ingest events</div>
            <div className="mt-2 tabular-nums text-slate-200">
              <div>Total: {data.ingest_events.total}</div>
              <div className="text-sm text-slate-400">
                24h: {data.ingest_events.last_24h} · 7d: {data.ingest_events.last_7d}
              </div>
            </div>
          </div>
          <div className={gits.card + ' p-4'}>
            <div className={gits.hudLabel}>Session messages</div>
            <div className="mt-2 text-3xl font-semibold tabular-nums text-cyan-200">{data.session_messages.total}</div>
          </div>
          <div className={`${gits.card} p-4 sm:col-span-2`}>
            <div className={gits.hudLabel}>Chunks by user_id</div>
            <div className="mt-3 max-h-40 overflow-y-auto">
              {Object.keys(data.chunks_by_user).length === 0 ? (
                <span className="text-sm text-slate-500">No chunks yet</span>
              ) : (
                <table className="w-full text-left text-sm">
                  <thead>
                    <tr className={gits.tableHead}>
                      <th className="px-2 py-1">user_id</th>
                      <th className="px-2 py-1">count</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800/80">
                    {Object.entries(data.chunks_by_user)
                      .sort((a, b) => b[1] - a[1])
                      .map(([uid, n]) => (
                        <tr key={uid} className={gits.tableRow}>
                          <td className="px-2 py-1 font-mono text-slate-300">{uid}</td>
                          <td className="px-2 py-1 tabular-nums text-slate-200">{n}</td>
                        </tr>
                      ))}
                  </tbody>
                </table>
              )}
            </div>
          </div>
          <div className={`${gits.card} p-4 sm:col-span-2 lg:col-span-3`}>
            <div className={gits.hudLabel}>Session messages by user_id</div>
            <div className="mt-3 max-h-40 overflow-y-auto">
              {Object.keys(data.session_messages.by_user).length === 0 ? (
                <span className="text-sm text-slate-500">None</span>
              ) : (
                <table className="w-full text-left text-sm">
                  <thead>
                    <tr className={gits.tableHead}>
                      <th className="px-2 py-1">user_id</th>
                      <th className="px-2 py-1">count</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800/80">
                    {Object.entries(data.session_messages.by_user)
                      .sort((a, b) => b[1] - a[1])
                      .map(([uid, n]) => (
                        <tr key={uid} className={gits.tableRow}>
                          <td className="px-2 py-1 font-mono text-slate-300">{uid}</td>
                          <td className="px-2 py-1 tabular-nums text-slate-200">{n}</td>
                        </tr>
                      ))}
                  </tbody>
                </table>
              )}
            </div>
          </div>
        </div>
      ) : null}
    </div>
  )
}
