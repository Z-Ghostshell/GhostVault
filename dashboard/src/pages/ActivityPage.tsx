import { RefreshCw, X } from 'lucide-react'
import { useCallback, useEffect, useId, useState } from 'react'
import { apiFetch, apiGetJson, fetchErrorMessage, getStoredBearer } from '../api/client'
import type { ChunkGetResponse, VaultStatsResponse } from '../api/types'
import { gits } from '../theme/gvUi'

export default function ActivityPage() {
  const [data, setData] = useState<VaultStatsResponse | null>(null)
  const [err, setErr] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [chunkOpen, setChunkOpen] = useState(false)
  const [chunkLoading, setChunkLoading] = useState(false)
  const [chunkErr, setChunkErr] = useState<string | null>(null)
  const [chunkDetail, setChunkDetail] = useState<ChunkGetResponse | null>(null)
  const [selectedChunkId, setSelectedChunkId] = useState<string | null>(null)
  const chunkModalTitleId = useId()

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

  const loadChunk = useCallback(async (id: string) => {
    if (!getStoredBearer()?.trim()) {
      setChunkErr('Session required — use Session / unlock in the header.')
      setChunkDetail(null)
      return
    }
    setChunkLoading(true)
    setChunkErr(null)
    setChunkDetail(null)
    try {
      const res = await apiFetch(`/v1/chunks/${encodeURIComponent(id)}`, { method: 'GET' })
      const text = await res.text()
      if (!res.ok) {
        let detail = text.slice(0, 300)
        try {
          const j = JSON.parse(text) as { detail?: string }
          if (typeof j.detail === 'string') {
            detail = j.detail
          }
        } catch {
          // ignore
        }
        throw new Error(detail)
      }
      setChunkDetail(JSON.parse(text) as ChunkGetResponse)
    } catch (e) {
      setChunkErr(fetchErrorMessage(e))
    } finally {
      setChunkLoading(false)
    }
  }, [])

  const openChunk = useCallback(
    (id: string) => {
      setSelectedChunkId(id)
      setChunkOpen(true)
      void loadChunk(id)
    },
    [loadChunk],
  )

  const closeChunk = useCallback(() => {
    setChunkOpen(false)
    setSelectedChunkId(null)
    setChunkDetail(null)
    setChunkErr(null)
  }, [])

  useEffect(() => {
    if (!chunkOpen) {
      return
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        closeChunk()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [chunkOpen, closeChunk])

  const rows = data?.recent_activity ?? []

  return (
    <div>
      <div className="mb-6 flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className={gits.pageTitle}>Activity</h1>
          <p className={gits.pageSub}>
            Recent ingest_history rows (newest first). Click a <span className="text-slate-400">chunk_id</span> to open the
            stored text for that memory piece.
          </p>
          {data ? (
            <p className="mt-2 max-w-3xl text-xs text-slate-500">
              This list is for{' '}
              <span className="font-mono text-slate-400">{data.vault_id}</span>
              {' — '}
              <span className="tabular-nums">{data.chunks_total}</span> chunks,{' '}
              <span className="tabular-nums">{data.ingest_events.total}</span> ingest events (stats).
              If those stay 0 after an MCP ingest, the dashboard token or API base URL points at a different
              gvsvd instance or vault than your client — align{' '}
              <span className="font-mono">VITE_API_BASE_URL</span> / Vite proxy with{' '}
              <span className="font-mono">GHOSTVAULT_BASE_URL</span> and use the same Bearer as ingest.
            </p>
          ) : null}
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
      ) : (
        <div className={`${gits.card} overflow-x-auto`}>
          <table className="w-full min-w-[32rem] text-left text-sm">
            <thead>
              <tr className={gits.tableHead}>
                <th className="px-3 py-2">time (UTC)</th>
                <th className="px-3 py-2">event</th>
                <th className="px-3 py-2">user_id</th>
                <th className="px-3 py-2">chunk_id (click to view text)</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/90">
              {rows.length === 0 ? (
                <tr>
                  <td colSpan={4} className="px-3 py-6 text-center text-slate-500">
                    No ingest activity yet
                  </td>
                </tr>
              ) : (
                rows.map((r) => (
                  <tr key={r.id} className={gits.tableRow}>
                    <td className="px-3 py-2 font-mono text-xs text-slate-300">{r.created_at}</td>
                    <td className="px-3 py-2 text-slate-200">{r.event}</td>
                    <td className="px-3 py-2 font-mono text-xs text-slate-400">{r.user_id ?? '—'}</td>
                    <td className="max-w-[18rem] px-3 py-2">
                      {r.chunk_id ? (
                        <button
                          type="button"
                          onClick={() => void openChunk(r.chunk_id as string)}
                          className="w-full cursor-pointer truncate text-left font-mono text-xs text-cyan-400/95 underline decoration-cyan-500/30 decoration-dotted underline-offset-2 hover:text-cyan-300 hover:decoration-cyan-400/50"
                        >
                          {r.chunk_id}
                        </button>
                      ) : (
                        <span className="text-slate-500">—</span>
                      )}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      )}

      {chunkOpen ? (
        <div
          className="fixed inset-0 z-[150] flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm"
          role="presentation"
          onClick={closeChunk}
        >
          <div
            role="dialog"
            aria-modal="true"
            aria-labelledby={chunkModalTitleId}
            className="flex max-h-[min(90vh,40rem)] w-full max-w-3xl flex-col overflow-hidden rounded-xl border border-cyan-500/25 bg-slate-950/95 shadow-[0_0_40px_-10px_rgba(34,211,238,0.35)]"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex shrink-0 items-center justify-between border-b border-slate-800/90 px-4 py-3">
              <h2 id={chunkModalTitleId} className="pr-2 font-mono text-sm font-semibold text-cyan-200/90">
                {selectedChunkId ? `Chunk ${selectedChunkId}` : 'Chunk'}
              </h2>
              <button
                type="button"
                onClick={closeChunk}
                className="rounded-lg p-1.5 text-slate-400 hover:bg-slate-800/80 hover:text-slate-200"
                aria-label="Close"
              >
                <X className="h-5 w-5" />
              </button>
            </div>
            <div className="min-h-0 flex-1 overflow-y-auto px-4 py-3">
              {chunkLoading ? <p className="text-sm text-slate-500">Loading…</p> : null}
              {chunkErr ? <div className={`${gits.configErrBanner} text-sm`}>{chunkErr}</div> : null}
              {chunkDetail && !chunkLoading ? (
                <div className="space-y-3">
                  <dl className="grid grid-cols-1 gap-1 text-[11px] text-slate-500 sm:grid-cols-2">
                    <div>
                      <dt className="text-slate-600">user_id</dt>
                      <dd className="font-mono text-slate-400">{chunkDetail.user_id}</dd>
                    </div>
                    <div>
                      <dt className="text-slate-600">ingest_session_key</dt>
                      <dd className="font-mono text-slate-400">
                        {chunkDetail.ingest_session_key ?? '—'}
                      </dd>
                    </div>
                    <div>
                      <dt className="text-slate-600">embedding_model_id</dt>
                      <dd className="font-mono text-slate-400">{chunkDetail.embedding_model_id}</dd>
                    </div>
                    <div>
                      <dt className="text-slate-600">created_at</dt>
                      <dd className="font-mono text-slate-400">{chunkDetail.created_at}</dd>
                    </div>
                  </dl>
                  {chunkDetail.source_document_id ? (
                    <div className="text-[11px] text-slate-500">
                      <span className="text-slate-600">source_document_id</span>{' '}
                      <span className="font-mono text-slate-400">{chunkDetail.source_document_id}</span>
                    </div>
                  ) : null}
                  {chunkDetail.kind ? (
                    <div className="text-[11px] text-slate-500">
                      <span className="text-slate-600">kind</span>{' '}
                      <span className="font-mono text-slate-400">{chunkDetail.kind}</span>
                    </div>
                  ) : null}
                  {chunkDetail.abstract ? (
                    <div>
                      <div className={gits.hudLabel + ' mb-1'}>Abstract</div>
                      <pre className="max-h-[30vh] overflow-auto whitespace-pre-wrap rounded-lg border border-slate-800/90 bg-slate-950/90 p-3 font-sans text-sm leading-relaxed text-slate-200">
                        {chunkDetail.abstract}
                      </pre>
                    </div>
                  ) : null}
                  <div>
                    <div className={gits.hudLabel + ' mb-1'}>Body</div>
                    <pre className="max-h-[50vh] overflow-auto whitespace-pre-wrap rounded-lg border border-slate-800/90 bg-slate-950/90 p-3 font-sans text-sm leading-relaxed text-slate-200">
                      {chunkDetail.text}
                    </pre>
                  </div>
                </div>
              ) : null}
            </div>
          </div>
        </div>
      ) : null}
    </div>
  )
}
