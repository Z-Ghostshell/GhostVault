import { Bug, Play, Search } from 'lucide-react'
import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { apiFetch, fetchErrorMessage, getStoredBearer } from '../api/client'
import { isRetrieveDebugNavVisible, isRetrieveDebugPageEnabled, setRetrieveDebugOptIn } from '../config'
import { gits } from '../theme/gvUi'

type RetrieveDebugParams = {
  semantic_threshold: number
  fusion_lexical_weight: number
  embedding_model: string
  k_dense: number
  max_chunks: number
  max_tokens: number
  session_mode: string
  session_key?: string
  content_mode?: string
}

type DenseRow = {
  chunk_id: string
  semantic_score: number
  passed_threshold: boolean
}

type LexicalRow = {
  chunk_id: string
  raw_lex_rank: number
  lexical_norm: number
}

type FusedRow = {
  chunk_id: string
  semantic_score: number
  lexical_norm: number
  fused_score: number
  session_boost_applied: boolean
  score_after_session: number
}

type PackingIncluded = { chunk_id: string; estimated_tokens: number }
type PackingSkip = { chunk_id: string; estimated_tokens: number; reason: string }
type Packing = {
  included: PackingIncluded[]
  skipped_due_to_token_budget: PackingSkip[]
  skipped_due_to_max_chunks: string[]
  stopped_due_to_max_chunks: boolean
}

type RetrieveDebug = {
  params: RetrieveDebugParams
  dense_candidates: DenseRow[]
  lexical_candidates: LexicalRow[]
  fused_ranked: FusedRow[]
  packing: Packing
}

type Snippet = {
  chunk_id: string
  text: string
  score: number
  abstract?: string
  full_text?: string
  full_available?: boolean
  kind?: string
}

function TableShell({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="mb-6">
      <h2 className={`mb-2 ${gits.hudLabel}`}>{title}</h2>
      <div className="overflow-x-auto rounded-lg border border-slate-800/90 bg-slate-950/50">{children}</div>
    </div>
  )
}

export default function RetrieveDebugPage() {
  const [vaultId, setVaultId] = useState('')
  const [userId, setUserId] = useState('default')
  const [query, setQuery] = useState('')
  const [maxChunks, setMaxChunks] = useState(8)
  const [maxTokens, setMaxTokens] = useState(0)
  const [sessionKey, setSessionKey] = useState('default')
  const [sessionMode, setSessionMode] = useState('all')
  const [contentMode, setContentMode] = useState('auto')
  const [pending, setPending] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const [rawStatus, setRawStatus] = useState<number | null>(null)
  const [snippets, setSnippets] = useState<Snippet[] | null>(null)
  const [debug, setDebug] = useState<RetrieveDebug | null>(null)
  const [showOptIn, setShowOptIn] = useState(() => isRetrieveDebugNavVisible())

  useEffect(() => {
    const on = () => setShowOptIn(isRetrieveDebugNavVisible())
    window.addEventListener('ghostvault-retrieve-debug-changed', on)
    return () => window.removeEventListener('ghostvault-retrieve-debug-changed', on)
  }, [])

  const run = useCallback(async () => {
    setErr(null)
    setDebug(null)
    setSnippets(null)
    setRawStatus(null)
    if (!getStoredBearer()?.trim()) {
      setErr('Set a Bearer token via Session / unlock first.')
      return
    }
    if (!vaultId.trim() || !query.trim()) {
      setErr('vault_id and query are required.')
      return
    }
    setPending(true)
    try {
      const body = {
        vault_id: vaultId.trim(),
        user_id: userId.trim() || 'default',
        query: query.trim(),
        max_chunks: maxChunks,
        max_tokens: maxTokens,
        session_key: sessionKey.trim() || 'default',
        session_mode: sessionMode,
        content_mode: contentMode.trim() || 'auto',
        debug: true,
      }
      const res = await apiFetch('/v1/retrieve', {
        method: 'POST',
        body: JSON.stringify(body),
      })
      setRawStatus(res.status)
      const text = await res.text()
      let j: unknown
      try {
        j = JSON.parse(text) as unknown
      } catch {
        setErr(text.slice(0, 400))
        return
      }
      if (!res.ok) {
        setErr(typeof j === 'object' && j !== null && 'detail' in j ? String((j as { detail: string }).detail) : text.slice(0, 400))
        return
      }
      const obj = j as {
        results?: Snippet[]
        debug?: RetrieveDebug
      }
      setSnippets(obj.results ?? [])
      setDebug(obj.debug ?? null)
      if (!obj.debug) {
        setErr(
          'Response had no debug object. Set retrieve_debug: true in configs/gvsvd.yaml (or DB override) and reload tuning; the client already sent "debug": true.',
        )
      }
    } catch (e) {
      setErr(fetchErrorMessage(e))
    } finally {
      setPending(false)
    }
  }, [vaultId, userId, query, maxChunks, maxTokens, sessionKey, sessionMode, contentMode])

  if (!isRetrieveDebugPageEnabled()) {
    return (
      <div>
        <h1 className={gits.pageTitle}>Retrieve debug</h1>
        <p className={gits.pageSub}>
          This page is not included in this build. Enable <code className="text-slate-400">VITE_SHOW_RETRIEVE_DEBUG=true</code>{' '}
          when building the dashboard image, or use <code className="text-slate-400">make dashboard-dev</code>.
        </p>
      </div>
    )
  }

  return (
    <div>
      <div className="mb-6">
        <h1 className={`inline-flex items-center gap-2 ${gits.pageTitle}`}>
          <Search className="h-7 w-7 text-fuchsia-400/80" />
          Retrieve debug
        </h1>
        <p className={gits.pageSub}>
          Hybrid retrieval pipeline: dense HNSW → lexical FTS → fusion (see server docs). Requires{' '}
          <code className="text-slate-400">retrieve_debug</code> in tuning (YAML/DB) and{' '}
          <code className="text-slate-400">&quot;debug&quot;: true</code> in the request (set below).
        </p>
      </div>

      {err ? <div className={`mb-4 ${gits.configErrBanner}`}>{err}</div> : null}

      <div className={`${gits.card} mb-6 space-y-3 p-4`}>
        <div className="grid gap-3 md:grid-cols-2">
          <label className="block">
            <span className={gits.hudLabel}>vault_id</span>
            <input
              value={vaultId}
              onChange={(e) => setVaultId(e.target.value)}
              className={`mt-1 w-full ${gits.input} font-mono text-xs`}
              placeholder="UUID from unlock"
              spellCheck={false}
            />
          </label>
          <label className="block">
            <span className={gits.hudLabel}>user_id</span>
            <input
              value={userId}
              onChange={(e) => setUserId(e.target.value)}
              className={`mt-1 w-full ${gits.input} font-mono text-xs`}
              spellCheck={false}
            />
          </label>
        </div>
        <label className="block">
          <span className={gits.hudLabel}>query</span>
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            className={`mt-1 w-full ${gits.input} font-mono text-xs`}
            spellCheck={false}
          />
        </label>
        <div className="grid gap-3 md:grid-cols-4">
          <label className="block">
            <span className={gits.hudLabel}>max_chunks</span>
            <input
              type="number"
              value={maxChunks}
              onChange={(e) => setMaxChunks(Number(e.target.value))}
              className={`mt-1 w-full ${gits.input} font-mono text-xs`}
            />
          </label>
          <label className="block">
            <span className={gits.hudLabel}>max_tokens (0 = no cap)</span>
            <input
              type="number"
              value={maxTokens}
              onChange={(e) => setMaxTokens(Number(e.target.value))}
              className={`mt-1 w-full ${gits.input} font-mono text-xs`}
            />
          </label>
          <label className="block">
            <span className={gits.hudLabel}>session_key</span>
            <input
              value={sessionKey}
              onChange={(e) => setSessionKey(e.target.value)}
              className={`mt-1 w-full ${gits.input} font-mono text-xs`}
            />
          </label>
          <label className="block">
            <span className={gits.hudLabel}>session_mode</span>
            <select
              value={sessionMode}
              onChange={(e) => setSessionMode(e.target.value)}
              className={`mt-1 w-full ${gits.input} font-mono text-xs`}
            >
              <option value="all">all</option>
              <option value="only">only</option>
              <option value="prefer">prefer</option>
            </select>
          </label>
        </div>
        <label className="mb-1 block max-w-xs">
          <span className={gits.hudLabel}>content_mode</span>
          <select
            value={contentMode}
            onChange={(e) => setContentMode(e.target.value)}
            className={`mt-1 w-full ${gits.input} font-mono text-xs`}
          >
            <option value="auto">auto</option>
            <option value="abstract">abstract</option>
            <option value="full">full</option>
            <option value="both">both</option>
            <option value="default">default</option>
          </select>
        </label>
        <button
          type="button"
          disabled={pending}
          onClick={() => void run()}
          className={`inline-flex items-center gap-2 ${gits.btnGhost}`}
        >
          <Play className="h-4 w-4" />
          Run retrieve (debug)
        </button>
        {rawStatus !== null ? (
          <span className="ml-3 font-mono text-xs text-slate-500">HTTP {rawStatus}</span>
        ) : null}
      </div>

      <label className="mb-6 flex max-w-lg cursor-pointer items-center gap-2 text-sm text-slate-400">
        <input
          type="checkbox"
          checked={showOptIn}
          onChange={(e) => {
            setRetrieveDebugOptIn(e.target.checked)
            setShowOptIn(e.target.checked)
          }}
        />
        Show &quot;Retrieve debug&quot; in the sidebar (uncheck to hide; you can return here via the URL to turn it back on)
      </label>

      {snippets && snippets.length > 0 ? (
        <TableShell title="Results (snippets)">
          <table className="w-full min-w-[32rem] border-collapse text-left text-xs">
            <thead>
              <tr className="border-b border-slate-800 bg-slate-900/50">
                <th className="px-2 py-2">score</th>
                <th className="px-2 py-2">chunk_id</th>
                <th className="px-2 py-2">text (preview)</th>
              </tr>
            </thead>
            <tbody>
              {snippets.map((s) => (
                <tr key={s.chunk_id} className="border-b border-slate-800/80">
                  <td className="px-2 py-1.5 font-mono tabular-nums text-cyan-200/90">{s.score.toFixed(6)}</td>
                  <td className="px-2 py-1.5 font-mono text-slate-400">{s.chunk_id}</td>
                  <td className="max-w-md truncate px-2 py-1.5 text-slate-300">{s.text}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </TableShell>
      ) : null}

      {debug ? (
        <>
          <TableShell title="Params">
            <pre className="max-h-48 overflow-auto p-3 font-mono text-[11px] text-slate-300">
              {JSON.stringify(debug.params, null, 2)}
            </pre>
          </TableShell>
          <TableShell title="Dense candidates">
            <table className="w-full min-w-[28rem] border-collapse text-left text-xs">
              <thead>
                <tr className="border-b border-slate-800 bg-slate-900/50">
                  <th className="px-2 py-2">chunk_id</th>
                  <th className="px-2 py-2">semantic s</th>
                  <th className="px-2 py-2">passed τ</th>
                </tr>
              </thead>
              <tbody>
                {debug.dense_candidates.map((r) => (
                  <tr key={r.chunk_id} className="border-b border-slate-800/80">
                    <td className="px-2 py-1 font-mono text-slate-400">{r.chunk_id}</td>
                    <td className="px-2 py-1 font-mono tabular-nums">{r.semantic_score.toFixed(6)}</td>
                    <td className="px-2 py-1">{r.passed_threshold ? 'yes' : 'no'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </TableShell>
          <TableShell title="Lexical candidates">
            <table className="w-full min-w-[32rem] border-collapse text-left text-xs">
              <thead>
                <tr className="border-b border-slate-800 bg-slate-900/50">
                  <th className="px-2 py-2">chunk_id</th>
                  <th className="px-2 py-2">raw rank</th>
                  <th className="px-2 py-2">norm</th>
                </tr>
              </thead>
              <tbody>
                {debug.lexical_candidates.map((r) => (
                  <tr key={r.chunk_id} className="border-b border-slate-800/80">
                    <td className="px-2 py-1 font-mono text-slate-400">{r.chunk_id}</td>
                    <td className="px-2 py-1 font-mono tabular-nums">{r.raw_lex_rank.toFixed(6)}</td>
                    <td className="px-2 py-1 font-mono tabular-nums">{r.lexical_norm.toFixed(6)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </TableShell>
          <TableShell title="Fused (post-boost order)">
            <table className="w-full min-w-[40rem] border-collapse text-left text-xs">
              <thead>
                <tr className="border-b border-slate-800 bg-slate-900/50">
                  <th className="px-2 py-2">chunk_id</th>
                  <th className="px-2 py-2">s</th>
                  <th className="px-2 py-2">lex</th>
                  <th className="px-2 py-2">fused</th>
                  <th className="px-2 py-2">session boost</th>
                  <th className="px-2 py-2">score after</th>
                </tr>
              </thead>
              <tbody>
                {debug.fused_ranked.map((r) => (
                  <tr key={r.chunk_id} className="border-b border-slate-800/80">
                    <td className="px-2 py-1 font-mono text-slate-400">{r.chunk_id}</td>
                    <td className="px-2 py-1 font-mono tabular-nums">{r.semantic_score.toFixed(6)}</td>
                    <td className="px-2 py-1 font-mono tabular-nums">{r.lexical_norm.toFixed(6)}</td>
                    <td className="px-2 py-1 font-mono tabular-nums">{r.fused_score.toFixed(6)}</td>
                    <td className="px-2 py-1">{r.session_boost_applied ? 'yes' : 'no'}</td>
                    <td className="px-2 py-1 font-mono tabular-nums text-cyan-200/80">{r.score_after_session.toFixed(6)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </TableShell>
          <TableShell title="Packing">
            <div className="p-3 text-xs text-slate-400">
              stopped_due_to_max_chunks: {debug.packing.stopped_due_to_max_chunks ? 'yes' : 'no'}
            </div>
            <div className="px-3 pb-2 font-mono text-[11px] text-slate-500">Included</div>
            <table className="w-full min-w-[24rem] border-collapse text-left text-xs">
              <thead>
                <tr className="border-b border-slate-800 bg-slate-900/50">
                  <th className="px-2 py-2">chunk_id</th>
                  <th className="px-2 py-2">est. tokens</th>
                </tr>
              </thead>
              <tbody>
                {debug.packing.included.map((r) => (
                  <tr key={`in-${r.chunk_id}`} className="border-b border-slate-800/80">
                    <td className="px-2 py-1 font-mono text-slate-400">{r.chunk_id}</td>
                    <td className="px-2 py-1 tabular-nums">{r.estimated_tokens}</td>
                  </tr>
                ))}
              </tbody>
            </table>
            {debug.packing.skipped_due_to_token_budget.length > 0 ? (
              <>
                <div className="px-3 pt-3 pb-2 font-mono text-[11px] text-amber-400/80">Skipped (token budget)</div>
                <table className="w-full min-w-[28rem] border-collapse text-left text-xs">
                  <thead>
                    <tr className="border-b border-slate-800 bg-slate-900/50">
                      <th className="px-2 py-2">chunk_id</th>
                      <th className="px-2 py-2">est. tokens</th>
                      <th className="px-2 py-2">reason</th>
                    </tr>
                  </thead>
                  <tbody>
                    {debug.packing.skipped_due_to_token_budget.map((r) => (
                      <tr key={`skip-${r.chunk_id}`} className="border-b border-slate-800/80">
                        <td className="px-2 py-1 font-mono text-slate-400">{r.chunk_id}</td>
                        <td className="px-2 py-1 tabular-nums">{r.estimated_tokens}</td>
                        <td className="px-2 py-1 text-slate-400">{r.reason}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </>
            ) : null}
            {debug.packing.skipped_due_to_max_chunks.length > 0 ? (
              <div className="p-3 font-mono text-[11px] text-slate-500">
                Skipped (max_chunks): {debug.packing.skipped_due_to_max_chunks.join(', ')}
              </div>
            ) : null}
          </TableShell>
        </>
      ) : null}

      <div className="mt-8 border-t border-slate-800/80 pt-4">
        <h2 className={`mb-2 flex items-center gap-2 ${gits.hudLabel}`}>
          <Bug className="h-4 w-4" />
          Ingest chunking (separate)
        </h2>
        <p className="text-sm text-slate-500">
          Non-infer ingest splits text with size 2000 / overlap 200. Use the API console to POST{' '}
          <code className="text-slate-400">/v1/ingest</code> with <code className="text-slate-400">&quot;debug&quot;: true</code> (same
          server flag) to see <code className="text-slate-400">debug.segments</code> in the JSON response.
        </p>
      </div>
    </div>
  )
}
