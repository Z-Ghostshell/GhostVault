import { Check, Pencil, RefreshCw, Sliders, X } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { apiFetch, apiGetJson, fetchErrorMessage } from '../api/client'
import type { TuningGetResponse, TuningSourceLabel } from '../api/types'
import { getApiDisplayLabel } from '../config'
import { gits } from '../theme/gvUi'

type FieldKind = 'bool' | 'int' | 'float' | 'string'

/** Mirrors server PatchableTuningKeys + value editor kind. */
const TUNING_FIELDS: Record<string, { kind: FieldKind; patchable: boolean }> = {
  semantic_threshold: { kind: 'float', patchable: true },
  fusion_lexical_weight: { kind: 'float', patchable: true },
  retrieve_default_max_chunks: { kind: 'int', patchable: true },
  retrieve_k_dense_min: { kind: 'int', patchable: true },
  retrieve_k_dense_multiplier: { kind: 'int', patchable: true },
  session_prefer_score_boost: { kind: 'float', patchable: true },
  chunk_rune_size: { kind: 'int', patchable: true },
  chunk_overlap: { kind: 'int', patchable: true },
  recent_session_messages_for_infer: { kind: 'int', patchable: true },
  prune_session_messages_keep: { kind: 'int', patchable: true },
  infer_llm_model: { kind: 'string', patchable: true },
  embedding_model: { kind: 'string', patchable: false },
  max_body_bytes: { kind: 'int', patchable: true },
  retrieve_debug: { kind: 'bool', patchable: true },
  http_access_log: { kind: 'bool', patchable: true },
}

function inferField(key: string, v: string | number | boolean): { kind: FieldKind; patchable: boolean } {
  const meta = TUNING_FIELDS[key]
  if (meta) return meta
  if (typeof v === 'boolean') return { kind: 'bool', patchable: key !== 'embedding_model' }
  if (typeof v === 'number') return { kind: Number.isInteger(v) ? 'int' : 'float', patchable: true }
  return { kind: 'string', patchable: true }
}

function formatValue(v: string | number | boolean): string {
  if (typeof v === 'boolean') return v ? 'true' : 'false'
  if (typeof v === 'number') return Number.isInteger(v) ? String(v) : String(v)
  return v
}

function toDraftString(v: string | number | boolean): string {
  return formatValue(v)
}

function parseDraft(
  _key: string,
  draft: string,
  kind: FieldKind
): string | number | boolean | null {
  const t = draft.trim()
  if (kind === 'bool') {
    if (t === 'true' || t === '1') return true
    if (t === 'false' || t === '0') return false
    return null
  }
  if (kind === 'int') {
    const n = parseInt(t, 10)
    if (Number.isNaN(n) || !Number.isFinite(n)) return null
    return n
  }
  if (kind === 'float') {
    const n = parseFloat(t)
    if (Number.isNaN(n) || !Number.isFinite(n)) return null
    return n
  }
  return t
}

function sourceBadge(s: TuningSourceLabel | undefined) {
  const c =
    s === 'db'
      ? 'bg-fuchsia-500/20 text-fuchsia-200 border-fuchsia-500/35'
      : s === 'file'
        ? 'bg-cyan-500/20 text-cyan-200 border-cyan-500/35'
        : 'bg-slate-600/30 text-slate-300 border-slate-500/30'
  return <span className={`rounded px-1.5 py-0.5 text-[10px] font-mono uppercase ${c}`}>{s ?? 'def'}</span>
}

const inputClass = `min-w-0 max-w-sm ${gits.input} py-1.5 font-mono text-sm`
const selectClass = `min-w-[7rem] ${gits.input} py-1.5 text-sm`

export default function ConfigPage() {
  const [data, setData] = useState<TuningGetResponse | null>(null)
  const [err, setErr] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [patchText, setPatchText] = useState('{"semantic_threshold":0.3}')
  const [bulkSaving, setBulkSaving] = useState(false)
  const [savingKey, setSavingKey] = useState<string | null>(null)
  const [editingKey, setEditingKey] = useState<string | null>(null)
  const [draft, setDraft] = useState('')

  const submitPatch = useCallback(async (body: Record<string, unknown>, rowKey: string | null) => {
    setErr(null)
    if (rowKey) setSavingKey(rowKey)
    else setBulkSaving(true)
    try {
      const res = await apiFetch('/v1/tuning', {
        method: 'PATCH',
        body: JSON.stringify(body),
      })
      if (!res.ok) {
        const t = await res.text()
        throw new Error(t || res.statusText)
      }
      const j = (await res.json()) as TuningGetResponse
      setData(j)
      setEditingKey(null)
      setDraft('')
    } catch (e) {
      setErr(fetchErrorMessage(e))
    } finally {
      if (rowKey) setSavingKey(null)
      else setBulkSaving(false)
    }
  }, [])

  const load = useCallback(async () => {
    setErr(null)
    setLoading(true)
    try {
      const j = await apiGetJson<TuningGetResponse>('/v1/tuning')
      setData(j)
    } catch (e) {
      setData(null)
      setErr(fetchErrorMessage(e))
    } finally {
      setLoading(false)
    }
  }, [])

  const reload = useCallback(async () => {
    setErr(null)
    setLoading(true)
    setEditingKey(null)
    setDraft('')
    try {
      const res = await apiFetch('/v1/tuning/reload', { method: 'POST' })
      if (!res.ok) {
        const t = await res.text()
        throw new Error(t || res.statusText)
      }
      const j = (await res.json()) as TuningGetResponse
      setData(j)
    } catch (e) {
      setErr(fetchErrorMessage(e))
    } finally {
      setLoading(false)
    }
  }, [])

  const applyPatchFromText = useCallback(async () => {
    const body = patchText.trim() === '' ? '{}' : patchText
    let parsed: Record<string, unknown>
    try {
      parsed = JSON.parse(body) as Record<string, unknown>
    } catch {
      setErr('Invalid JSON in patch area')
      return
    }
    await submitPatch(parsed, null)
  }, [patchText, submitPatch])

  const startEdit = (key: string, value: string | number | boolean) => {
    setEditingKey(key)
    setDraft(toDraftString(value))
  }

  const cancelEdit = () => {
    setEditingKey(null)
    setDraft('')
  }

  const saveField = (key: string, kind: FieldKind, patchable: boolean) => {
    if (!patchable) {
      setErr('This value can only be changed in the YAML file on the server, then reloaded.')
      return
    }
    const parsed = parseDraft(key, draft, kind)
    if (parsed === null) {
      setErr(`Invalid value for ${key}`)
      return
    }
    void submitPatch({ [key]: parsed }, key)
  }

  useEffect(() => {
    void load()
  }, [load])

  const rows = data
    ? Object.keys(data.tuning)
        .sort()
        .map((k) => {
          const v = data.tuning[k] as string | number | boolean
          return {
            key: k,
            value: v,
            source: data.sources[k] as TuningSourceLabel,
            ...inferField(k, v),
          }
        })
    : []

  return (
    <div>
      <div className="mb-6 flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className={gits.pageTitle}>Config</h1>
          <p className={gits.pageSub}>
            Runtime tuning (defaults → YAML file → database overrides). API {getApiDisplayLabel()}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <button
            type="button"
            onClick={() => void load()}
            disabled={loading}
            className={`inline-flex items-center gap-2 ${gits.btnGhostSm}`}
          >
            <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </button>
          <button
            type="button"
            onClick={() => void reload()}
            disabled={loading}
            className={`inline-flex items-center gap-2 ${gits.btnGhostSm}`}
          >
            <Sliders className="h-4 w-4" />
            Reload from disk + DB
          </button>
        </div>
      </div>

      {err ? <div className={`mb-6 ${gits.configErrBanner}`}>{err}</div> : null}

      {data ? (
        <p className="mb-4 font-mono text-xs text-slate-500">
          <span className="text-cyan-500/80">tuning_file</span> {data.tuning_file || '(default path)'}
        </p>
      ) : null}

      {loading && !data ? (
        <div className="text-slate-500">Loading…</div>
      ) : data ? (
        <div className="space-y-6">
          <div className={`overflow-hidden ${gits.card}`}>
            <div className="border-b border-cyan-500/15 px-4 py-3">
              <div className={gits.hudLabel}>Effective values</div>
            </div>
            <div className="overflow-x-auto">
              <table className="w-full min-w-[760px] text-sm text-slate-200">
                <thead>
                  <tr className={gits.tableHead}>
                    <th className="p-3">Key</th>
                    <th className="p-3">Value</th>
                    <th className="p-3">Source</th>
                    <th className="p-3 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800/80">
                  {rows.map((r) => {
                    const isEditing = editingKey === r.key
                    return (
                      <tr key={r.key} className={gits.tableRow}>
                        <td className="p-3 font-mono text-xs text-cyan-200/90">{r.key}</td>
                        <td className="p-3 align-middle">
                          {isEditing && r.patchable ? (
                            r.kind === 'string' ? (
                              <input
                                type="text"
                                className={`w-full max-w-md ${gits.input} py-1.5 font-mono text-sm`}
                                value={draft}
                                onChange={(e) => setDraft(e.target.value)}
                                onKeyDown={(e) => {
                                  if (e.key === 'Enter') saveField(r.key, r.kind, r.patchable)
                                  if (e.key === 'Escape') cancelEdit()
                                }}
                                autoFocus
                              />
                            ) : r.kind === 'bool' ? (
                              <select
                                className={selectClass}
                                value={draft}
                                onChange={(e) => setDraft(e.target.value)}
                                autoFocus
                              >
                                <option value="true">true</option>
                                <option value="false">false</option>
                              </select>
                            ) : (
                              <input
                                type="number"
                                className={inputClass}
                                step={r.kind === 'int' ? 1 : 'any'}
                                value={draft}
                                onChange={(e) => setDraft(e.target.value)}
                                onKeyDown={(e) => {
                                  if (e.key === 'Enter') saveField(r.key, r.kind, r.patchable)
                                  if (e.key === 'Escape') cancelEdit()
                                }}
                                autoFocus
                              />
                            )
                          ) : (
                            <span className="font-mono text-xs">{formatValue(r.value)}</span>
                          )}
                        </td>
                        <td className="p-3 align-middle">{sourceBadge(r.source)}</td>
                        <td className="p-3 text-right">
                          {isEditing && r.patchable ? (
                            <div className="inline-flex items-center justify-end gap-1">
                              <button
                                type="button"
                                className="inline-flex items-center gap-1 rounded border border-emerald-500/35 bg-emerald-950/40 px-2 py-1 text-xs text-emerald-200 hover:bg-emerald-950/60"
                                disabled={savingKey === r.key}
                                onClick={() => saveField(r.key, r.kind, r.patchable)}
                              >
                                <Check className="h-3.5 w-3.5" />
                                Save
                              </button>
                              <button
                                type="button"
                                className="inline-flex items-center gap-1 rounded border border-slate-600 bg-slate-900/60 px-2 py-1 text-xs text-slate-300 hover:bg-slate-800"
                                onClick={cancelEdit}
                                disabled={savingKey === r.key}
                              >
                                <X className="h-3.5 w-3.5" />
                                Cancel
                              </button>
                            </div>
                          ) : r.patchable ? (
                            <button
                              type="button"
                              className={`inline-flex items-center gap-1.5 ${gits.btnGhostSm}`}
                              onClick={() => startEdit(r.key, r.value)}
                              disabled={savingKey !== null || bulkSaving}
                            >
                              <Pencil className="h-3.5 w-3.5" />
                              Edit
                            </button>
                          ) : (
                            <span
                              className="text-[10px] text-slate-500"
                              title="Edit configs/gvsvd.yaml on the server, then use Reload from disk + DB"
                            >
                              YAML only
                            </span>
                          )}
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          </div>

          <div className={`p-4 ${gits.card}`}>
            <div className={`mb-2 ${gits.hudLabel}`}>DB overrides (raw JSON, merge)</div>
            <pre className="mb-3 max-h-40 overflow-auto rounded border border-slate-700/60 bg-slate-950/80 p-3 font-mono text-xs text-slate-400">
              {JSON.stringify(data.overrides ?? {}, null, 2)}
            </pre>
            <p className="mb-2 text-xs text-slate-500">
              Use Edit in the table for single keys, or paste multiple keys below.
            </p>
            <textarea
              className={`mb-2 min-h-[88px] w-full font-mono text-xs ${gits.input}`}
              value={patchText}
              onChange={(e) => setPatchText(e.target.value)}
              rows={4}
            />
            <button
              type="button"
              onClick={() => void applyPatchFromText()}
              disabled={bulkSaving || savingKey !== null}
              className={gits.btnGhost}
            >
              {bulkSaving ? 'Applying…' : 'Apply patch'}
            </button>
          </div>
        </div>
      ) : null}
    </div>
  )
}
