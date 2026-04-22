export interface VaultStatsResponse {
  vault_id: string
  encryption_enabled: boolean
  chunks_total: number
  chunks_by_user: Record<string, number>
  ingest_events: {
    total: number
    last_24h: number
    last_7d: number
  }
  session_messages: {
    total: number
    by_user: Record<string, number>
  }
  recent_activity: {
    id: number
    chunk_id?: string
    event: string
    user_id?: string
    created_at: string
  }[]
}

/** GET /v1/chunks/{id} */
export interface ChunkGetResponse {
  chunk_id: string
  user_id: string
  text: string
  abstract?: string
  kind?: string
  metadata?: Record<string, unknown>
  /** Present when the chunk shares full text from source_documents. */
  source_document_id?: string
  embedding_model_id: string
  created_at: string
  chunk_schema_version: number
  ingest_session_key?: string | null
}

/** GET /v1/tuning, POST /v1/tuning/reload, PATCH /v1/tuning */
export type TuningSourceLabel = 'def' | 'file' | 'db'

export interface TuningGetResponse {
  tuning: Record<string, string | number | boolean>
  sources: Record<string, TuningSourceLabel>
  tuning_file: string
  overrides: Record<string, unknown>
  reloaded?: boolean
  updated?: boolean
}
