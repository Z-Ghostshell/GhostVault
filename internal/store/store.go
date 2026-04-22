package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type Pool struct {
	pool *pgxpool.Pool
}

func NewPool(ctx context.Context, dsn string) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	p, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &Pool{pool: p}, nil
}

func (p *Pool) Close() { p.pool.Close() }

func (p *Pool) Pool() *pgxpool.Pool { return p.pool }

func (p *Pool) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }

type VaultRow struct {
	ID                uuid.UUID
	EncryptionEnabled bool
	AuthMode          string
	ArgonTime         *int32
	ArgonMemoryKB     *int32
	ArgonParallelism  *int32
	ArgonSalt         []byte
	WrappedDEK        []byte
	CreatedAt         time.Time
}

func (p *Pool) GetAnyVault(ctx context.Context) (*VaultRow, error) {
	const q = `SELECT id, encryption_enabled, auth_mode, argon2_time_cost, argon2_memory_kb, argon2_parallelism, argon2_salt, wrapped_dek, created_at
		FROM vaults ORDER BY created_at LIMIT 1`
	var v VaultRow
	err := p.pool.QueryRow(ctx, q).Scan(
		&v.ID, &v.EncryptionEnabled, &v.AuthMode, &v.ArgonTime, &v.ArgonMemoryKB, &v.ArgonParallelism,
		&v.ArgonSalt, &v.WrappedDEK, &v.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (p *Pool) GetVault(ctx context.Context, id uuid.UUID) (*VaultRow, error) {
	const q = `SELECT id, encryption_enabled, auth_mode, argon2_time_cost, argon2_memory_kb, argon2_parallelism, argon2_salt, wrapped_dek, created_at
		FROM vaults WHERE id = $1`
	var v VaultRow
	err := p.pool.QueryRow(ctx, q, id).Scan(
		&v.ID, &v.EncryptionEnabled, &v.AuthMode, &v.ArgonTime, &v.ArgonMemoryKB, &v.ArgonParallelism,
		&v.ArgonSalt, &v.WrappedDEK, &v.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (p *Pool) InsertVault(ctx context.Context, encEnabled bool, authMode string, salt, wrapped []byte, t, mem int32, par int32) (uuid.UUID, error) {
	id := uuid.New()
	if encEnabled {
		const q = `INSERT INTO vaults (id, encryption_enabled, auth_mode, argon2_time_cost, argon2_memory_kb, argon2_parallelism, argon2_salt, wrapped_dek)
			VALUES ($1, true, $2, $3, $4, $5, $6, $7)`
		_, err := p.pool.Exec(ctx, q, id, authMode, t, mem, par, salt, wrapped)
		return id, err
	}
	const q = `INSERT INTO vaults (id, encryption_enabled, auth_mode) VALUES ($1, false, $2)`
	_, err := p.pool.Exec(ctx, q, id, authMode)
	return id, err
}

// SessionRow mirrors one row of the sessions table. DEK is never stored here — it's
// held in the gvsvd process VaultState and attached per-request.
type SessionRow struct {
	TokenHash []byte
	VaultID   uuid.UUID
	CreatedAt time.Time
	LastSeen  time.Time
	ExpiresAt time.Time
	MaxUntil  time.Time
}

func (p *Pool) InsertSession(ctx context.Context, tokenHash []byte, vaultID uuid.UUID, now, expiresAt, maxUntil time.Time) error {
	const q = `INSERT INTO sessions (token_hash, vault_id, created_at, last_seen, expires_at, max_until)
		VALUES ($1, $2, $3, $3, $4, $5)`
	_, err := p.pool.Exec(ctx, q, tokenHash, vaultID, now, expiresAt, maxUntil)
	return err
}

func (p *Pool) GetSessionByHash(ctx context.Context, tokenHash []byte) (*SessionRow, error) {
	const q = `SELECT token_hash, vault_id, created_at, last_seen, expires_at, max_until
		FROM sessions WHERE token_hash = $1`
	var s SessionRow
	err := p.pool.QueryRow(ctx, q, tokenHash).Scan(
		&s.TokenHash, &s.VaultID, &s.CreatedAt, &s.LastSeen, &s.ExpiresAt, &s.MaxUntil,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (p *Pool) TouchSession(ctx context.Context, tokenHash []byte, lastSeen, expiresAt time.Time) error {
	const q = `UPDATE sessions SET last_seen = $2, expires_at = $3 WHERE token_hash = $1`
	_, err := p.pool.Exec(ctx, q, tokenHash, lastSeen, expiresAt)
	return err
}

func (p *Pool) DeleteSession(ctx context.Context, tokenHash []byte) error {
	const q = `DELETE FROM sessions WHERE token_hash = $1`
	_, err := p.pool.Exec(ctx, q, tokenHash)
	return err
}

// PurgeSessionsForVault removes every persisted session for a vault. Called at gvsvd
// startup when auth_mode=session so restarts still invalidate all outstanding bearers.
func (p *Pool) PurgeSessionsForVault(ctx context.Context, vaultID uuid.UUID) error {
	const q = `DELETE FROM sessions WHERE vault_id = $1`
	_, err := p.pool.Exec(ctx, q, vaultID)
	return err
}

type ChunkInsert struct {
	ID                 uuid.UUID
	VaultID            uuid.UUID
	UserID             string
	Ciphertext         []byte
	Nonce              []byte
	KeyID              *string
	BodyText           *string
	ContentSHA256      []byte
	EmbeddingModelID   string
	ChunkSchemaVersion int
	PlainForTSV        string // raw text for tsvector update
	Embedding          []float32
	// IngestSessionKey, when set, tags chunks for optional session_scoped retrieve (see Engine.Retrieve).
	IngestSessionKey *string
	// ItemKind and ItemMetadata mirror structured ingest (v2); ItemMetadata is optional JSON.
	ItemKind     string
	ItemMetadata map[string]any
	// SourceDocumentID links to source_documents (shared full text); v2 body in chunk may be empty.
	SourceDocumentID *uuid.UUID
}

func (p *Pool) InsertChunk(ctx context.Context, c *ChunkInsert) (err error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	var meta any
	if c.ItemMetadata != nil {
		b, mErr := json.Marshal(c.ItemMetadata)
		if mErr != nil {
			return mErr
		}
		meta = b
	}

	const qChunk = `INSERT INTO memory_chunks (
		id, vault_id, user_id, ciphertext, nonce, key_id, body_text, content_sha256, embedding_model_id, chunk_schema_version, search_tsv, ingest_session_key,
		item_kind, item_metadata, source_document_id
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10, to_tsvector('english', $11), $12, $13, $14::jsonb, $15)`

	_, err = tx.Exec(ctx, qChunk,
		c.ID, c.VaultID, c.UserID, c.Ciphertext, c.Nonce, c.KeyID, c.BodyText,
		c.ContentSHA256, c.EmbeddingModelID, c.ChunkSchemaVersion, c.PlainForTSV, c.IngestSessionKey,
		c.ItemKind, meta, c.SourceDocumentID,
	)
	if err != nil {
		return err
	}

	vec := pgvectorVec(c.Embedding)
	const qEmb = `INSERT INTO chunk_embeddings (chunk_id, embedding, embedding_model_id) VALUES ($1, $2::vector, $3)`
	_, err = tx.Exec(ctx, qEmb, c.ID, vec, c.EmbeddingModelID)
	if err != nil {
		return err
	}

	const qHist = `INSERT INTO ingest_history (vault_id, chunk_id, event) VALUES ($1,$2,'ADD')`
	_, err = tx.Exec(ctx, qHist, c.VaultID, c.ID)
	if err != nil {
		return err
	}
	err = tx.Commit(ctx)
	return err
}

// pgvectorVec formats float32 slice for pgvector text input.
func pgvectorVec(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(fmt.Sprintf("%g", f))
	}
	b.WriteByte(']')
	return b.String()
}

type ChunkCandidate struct {
	ChunkID       uuid.UUID
	SemanticScore float64
	LexicalScore  float64
	RawLexRank    float64
}

func (p *Pool) SearchDense(ctx context.Context, vaultID uuid.UUID, userID string, modelID string, queryVec []float32, limit int, sessionKey string, onlySession bool) ([]ChunkCandidate, error) {
	qv := pgvectorVec(queryVec)
	var q string
	var rows pgx.Rows
	var err error
	if onlySession && sessionKey != "" {
		q = `
		SELECT mc.id,
			1 - (ce.embedding <=> $1::vector) AS sem
		FROM chunk_embeddings ce
		JOIN memory_chunks mc ON mc.id = ce.chunk_id
		WHERE mc.vault_id = $2 AND mc.user_id = $3 AND mc.deleted_at IS NULL
			AND ce.embedding_model_id = $4
			AND mc.ingest_session_key = $5
		ORDER BY ce.embedding <=> $1::vector
		LIMIT $6`
		rows, err = p.pool.Query(ctx, q, qv, vaultID, userID, modelID, sessionKey, limit)
	} else {
		q = `
		SELECT mc.id,
			1 - (ce.embedding <=> $1::vector) AS sem
		FROM chunk_embeddings ce
		JOIN memory_chunks mc ON mc.id = ce.chunk_id
		WHERE mc.vault_id = $2 AND mc.user_id = $3 AND mc.deleted_at IS NULL
			AND ce.embedding_model_id = $4
		ORDER BY ce.embedding <=> $1::vector
		LIMIT $5`
		rows, err = p.pool.Query(ctx, q, qv, vaultID, userID, modelID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChunkCandidate
	for rows.Next() {
		var id uuid.UUID
		var sem float64
		if err := rows.Scan(&id, &sem); err != nil {
			return nil, err
		}
		out = append(out, ChunkCandidate{ChunkID: id, SemanticScore: sem})
	}
	return out, rows.Err()
}

func (p *Pool) SearchLexical(ctx context.Context, vaultID uuid.UUID, userID string, query string, limit int, sessionKey string, onlySession bool) ([]ChunkCandidate, error) {
	var q string
	var rows pgx.Rows
	var err error
	if onlySession && sessionKey != "" {
		q = `
		SELECT mc.id, ts_rank_cd(mc.search_tsv, plainto_tsquery('english', $1)) AS r
		FROM memory_chunks mc
		WHERE mc.vault_id = $2 AND mc.user_id = $3 AND mc.deleted_at IS NULL
			AND mc.search_tsv @@ plainto_tsquery('english', $1)
			AND mc.ingest_session_key = $5
		ORDER BY r DESC
		LIMIT $4`
		rows, err = p.pool.Query(ctx, q, query, vaultID, userID, limit, sessionKey)
	} else {
		q = `
		SELECT mc.id, ts_rank_cd(mc.search_tsv, plainto_tsquery('english', $1)) AS r
		FROM memory_chunks mc
		WHERE mc.vault_id = $2 AND mc.user_id = $3 AND mc.deleted_at IS NULL
			AND mc.search_tsv @@ plainto_tsquery('english', $1)
		ORDER BY r DESC
		LIMIT $4`
		rows, err = p.pool.Query(ctx, q, query, vaultID, userID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChunkCandidate
	for rows.Next() {
		var id uuid.UUID
		var r float64
		if err := rows.Scan(&id, &r); err != nil {
			return nil, err
		}
		out = append(out, ChunkCandidate{ChunkID: id, RawLexRank: r})
	}
	return out, rows.Err()
}

type ChunkPayload struct {
	ID               uuid.UUID
	Ciphertext       []byte
	Nonce            []byte
	BodyText         *string
	EmbeddingModelID string
	// From DB row (denormalized; may be empty if only inside JSON body).
	ItemKind         string
	ItemMetadata     []byte
	SourceDocumentID *uuid.UUID
	VaultID          uuid.UUID // set when needed for source doc resolution; optional for some queries
}

// IngestSessionKeysByChunkIDs returns the ingest_session_key (may be NULL) for each id.
func (p *Pool) IngestSessionKeysByChunkIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*string, error) {
	out := make(map[uuid.UUID]*string)
	if len(ids) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	q := fmt.Sprintf(`SELECT id, ingest_session_key FROM memory_chunks WHERE id IN (%s) AND deleted_at IS NULL`, strings.Join(placeholders, ","))
	rows, err := p.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var sk pgtype.Text
		if err := rows.Scan(&id, &sk); err != nil {
			return nil, err
		}
		if sk.Valid {
			s := sk.String
			out[id] = &s
		} else {
			out[id] = nil
		}
	}
	return out, rows.Err()
}

func (p *Pool) GetChunkPayload(ctx context.Context, id uuid.UUID) (*ChunkPayload, error) {
	const q = `SELECT mc.id, mc.ciphertext, mc.nonce, mc.body_text, mc.embedding_model_id, COALESCE(mc.item_kind, ''), mc.item_metadata,
		mc.vault_id, mc.source_document_id
		FROM memory_chunks mc WHERE mc.id = $1 AND mc.deleted_at IS NULL`
	var c ChunkPayload
	var body pgtype.Text
	var metaBytes []byte
	var sdoc pgtype.UUID
	err := p.pool.QueryRow(ctx, q, id).Scan(&c.ID, &c.Ciphertext, &c.Nonce, &body, &c.EmbeddingModelID, &c.ItemKind, &metaBytes, &c.VaultID, &sdoc)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if body.Valid {
		s := body.String
		c.BodyText = &s
	}
	if len(metaBytes) > 0 {
		c.ItemMetadata = metaBytes
	}
	if sdoc.Valid {
		uu := uuid.UUID(sdoc.Bytes)
		c.SourceDocumentID = &uu
	}
	return &c, nil
}

// ChunkForVaultRow is GetChunkForVault’s metadata plus the encrypted/plain payload.
type ChunkForVaultRow struct {
	Payload            *ChunkPayload
	UserID             string
	CreatedAt          time.Time
	ChunkSchemaVersion int
	IngestSessionKey   *string
}

// GetChunkForVault returns payload and row metadata for a non-deleted chunk that belongs to vaultID.
func (p *Pool) GetChunkForVault(ctx context.Context, vaultID, chunkID uuid.UUID) (*ChunkForVaultRow, error) {
	const q = `SELECT id, user_id, created_at, ciphertext, nonce, body_text, embedding_model_id,
		chunk_schema_version, ingest_session_key, COALESCE(item_kind, ''), item_metadata, vault_id, source_document_id
		FROM memory_chunks WHERE id = $1 AND vault_id = $2 AND deleted_at IS NULL`
	var c ChunkPayload
	var userID string
	var body pgtype.Text
	var ca time.Time
	var schemaVer int
	var sk pgtype.Text
	var metaBytes []byte
	var sdoc pgtype.UUID
	err := p.pool.QueryRow(ctx, q, chunkID, vaultID).Scan(
		&c.ID, &userID, &ca, &c.Ciphertext, &c.Nonce, &body, &c.EmbeddingModelID, &schemaVer, &sk, &c.ItemKind, &metaBytes, &c.VaultID, &sdoc,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if body.Valid {
		s := body.String
		c.BodyText = &s
	}
	if len(metaBytes) > 0 {
		c.ItemMetadata = metaBytes
	}
	if sdoc.Valid {
		uu := uuid.UUID(sdoc.Bytes)
		c.SourceDocumentID = &uu
	}
	var isk *string
	if sk.Valid {
		s := sk.String
		isk = &s
	}
	return &ChunkForVaultRow{
		Payload:            &c,
		UserID:             userID,
		CreatedAt:          ca,
		ChunkSchemaVersion: schemaVer,
		IngestSessionKey:   isk,
	}, nil
}

// SourceDocumentRow is one row in source_documents (full shared document text).
type SourceDocumentRow struct {
	ID             uuid.UUID
	VaultID        uuid.UUID
	UserID         string
	Title          *string
	Ciphertext     []byte
	Nonce          []byte
	BodyText       *string
	ContentSHA256  []byte
	CreatedAt      time.Time
}

// SourceDocumentInsert is used to persist a shared source blob once per unique hash in (vault, user).
type SourceDocumentInsert struct {
	ID            uuid.UUID
	VaultID       uuid.UUID
	UserID        string
	Title         *string
	ContentSHA256 []byte
	BodyText      *string
	Ciphertext    []byte
	Nonce         []byte
	KeyID         *string
}

// GetSourceDocumentIDByHash returns an existing source document id if the same bytes were stored.
func (p *Pool) GetSourceDocumentIDByHash(ctx context.Context, vaultID uuid.UUID, userID string, hash []byte) (uuid.UUID, error) {
	const q = `SELECT id FROM source_documents WHERE vault_id = $1 AND user_id = $2 AND content_sha256 = $3`
	var id uuid.UUID
	err := p.pool.QueryRow(ctx, q, vaultID, userID, hash).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// InsertSourceDocument stores one shared document and returns its id.
func (p *Pool) InsertSourceDocument(ctx context.Context, d *SourceDocumentInsert) error {
	const q = `INSERT INTO source_documents (id, vault_id, user_id, title, ciphertext, nonce, key_id, body_text, content_sha256)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	_, err := p.pool.Exec(ctx, q, d.ID, d.VaultID, d.UserID, d.Title, d.Ciphertext, d.Nonce, d.KeyID, d.BodyText, d.ContentSHA256)
	return err
}

// GetSourceDocumentForVault returns a source document that belongs to vaultID, or ErrNotFound.
func (p *Pool) GetSourceDocumentForVault(ctx context.Context, vaultID, docID uuid.UUID) (*SourceDocumentRow, error) {
	const q = `SELECT id, vault_id, user_id, title, ciphertext, nonce, body_text, content_sha256, created_at
		FROM source_documents WHERE id = $1 AND vault_id = $2`
	var r SourceDocumentRow
	var title pgtype.Text
	var body pgtype.Text
	err := p.pool.QueryRow(ctx, q, docID, vaultID).Scan(
		&r.ID, &r.VaultID, &r.UserID, &title, &r.Ciphertext, &r.Nonce, &body, &r.ContentSHA256, &r.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if title.Valid {
		s := title.String
		r.Title = &s
	}
	if body.Valid {
		s := body.String
		r.BodyText = &s
	}
	return &r, nil
}

func (p *Pool) ChunkExistsHash(ctx context.Context, vault uuid.UUID, user string, hash []byte) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM memory_chunks WHERE vault_id=$1 AND user_id=$2 AND content_sha256=$3 AND deleted_at IS NULL)`
	var ok bool
	err := p.pool.QueryRow(ctx, q, vault, user, hash).Scan(&ok)
	return ok, err
}

func (p *Pool) AppendSessionMessage(ctx context.Context, vault uuid.UUID, user, sessionKey, role, content string) error {
	const q = `INSERT INTO session_messages (vault_id, user_id, session_key, role, content) VALUES ($1,$2,$3,$4,$5)`
	_, err := p.pool.Exec(ctx, q, vault, user, sessionKey, role, content)
	return err
}

func (p *Pool) RecentSessionMessages(ctx context.Context, vault uuid.UUID, user, sessionKey string, limit int) ([]struct{ Role, Content string }, error) {
	const q = `SELECT role, content FROM session_messages WHERE vault_id=$1 AND user_id=$2 AND session_key=$3 ORDER BY created_at DESC LIMIT $4`
	rows, err := p.pool.Query(ctx, q, vault, user, sessionKey, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct{ Role, Content string }
	for rows.Next() {
		var r, c string
		if err := rows.Scan(&r, &c); err != nil {
			return nil, err
		}
		out = append(out, struct{ Role, Content string }{r, c})
	}
	// reverse to chronological
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}

func (p *Pool) PruneSessionMessages(ctx context.Context, vault uuid.UUID, user, sessionKey string, keep int) error {
	const q = `
		DELETE FROM session_messages sm
		WHERE sm.ctid IN (
			SELECT ctid FROM session_messages
			WHERE vault_id=$1 AND user_id=$2 AND session_key=$3
			ORDER BY created_at DESC OFFSET $4
		)`
	_, err := p.pool.Exec(ctx, q, vault, user, sessionKey, keep)
	return err
}

func (p *Pool) InsertActionToken(ctx context.Context, vault uuid.UUID, tokenHash []byte, scopes []string, exp time.Time) (uuid.UUID, error) {
	id := uuid.New()
	const q = `INSERT INTO action_tokens (id, vault_id, token_hash, scopes, expires_at) VALUES ($1,$2,$3,$4,$5)`
	_, err := p.pool.Exec(ctx, q, id, vault, tokenHash, scopes, exp)
	return id, err
}

func (p *Pool) ValidActionToken(ctx context.Context, tokenHash []byte) (*uuid.UUID, error) {
	const q = `SELECT vault_id FROM action_tokens WHERE token_hash = $1 AND expires_at > now()`
	var vid uuid.UUID
	err := p.pool.QueryRow(ctx, q, tokenHash).Scan(&vid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &vid, nil
}

func (p *Pool) SoftDeleteChunk(ctx context.Context, vaultID, chunkID uuid.UUID) error {
	const q = `UPDATE memory_chunks SET deleted_at = now() WHERE id = $1 AND vault_id = $2 AND deleted_at IS NULL`
	tag, err := p.pool.Exec(ctx, q, chunkID, vaultID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	const q2 = `DELETE FROM chunk_embeddings WHERE chunk_id = $1`
	_, _ = p.pool.Exec(ctx, q2, chunkID)
	return nil
}

// VaultStats aggregates read-only metrics for an authenticated vault (dashboard / GET /v1/stats).
type VaultStats struct {
	ChunksTotal           int64
	ChunksByUser          map[string]int64
	IngestEventsTotal     int64
	IngestEventsLast24h   int64
	IngestEventsLast7d    int64
	SessionMessagesTotal  int64
	SessionMessagesByUser map[string]int64
	RecentActivity        []IngestActivityRow
}

// IngestActivityRow is one row from ingest_history with optional user from the chunk.
type IngestActivityRow struct {
	ID        int64
	ChunkID   *uuid.UUID
	Event     string
	CreatedAt time.Time
	UserID    string
}

// StatsForVault loads aggregate counts and recent ingest activity for one vault.
func (p *Pool) StatsForVault(ctx context.Context, vaultID uuid.UUID, recentLimit int) (*VaultStats, error) {
	if recentLimit <= 0 {
		recentLimit = 50
	}
	if recentLimit > 500 {
		recentLimit = 500
	}
	out := &VaultStats{
		ChunksByUser:          map[string]int64{},
		SessionMessagesByUser: map[string]int64{},
	}

	const qChunks = `
		SELECT user_id, COUNT(*)::bigint FROM memory_chunks
		WHERE vault_id = $1 AND deleted_at IS NULL GROUP BY user_id`
	rows, err := p.pool.Query(ctx, qChunks, vaultID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var uid string
		var n int64
		if err := rows.Scan(&uid, &n); err != nil {
			rows.Close()
			return nil, err
		}
		out.ChunksByUser[uid] = n
		out.ChunksTotal += n
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	const qIngest = `
		SELECT
			(SELECT COUNT(*)::bigint FROM ingest_history WHERE vault_id = $1),
			(SELECT COUNT(*)::bigint FROM ingest_history WHERE vault_id = $1 AND created_at >= now() - interval '24 hours'),
			(SELECT COUNT(*)::bigint FROM ingest_history WHERE vault_id = $1 AND created_at >= now() - interval '7 days')`
	err = p.pool.QueryRow(ctx, qIngest, vaultID).Scan(
		&out.IngestEventsTotal, &out.IngestEventsLast24h, &out.IngestEventsLast7d)
	if err != nil {
		return nil, err
	}

	const qSessTot = `SELECT COUNT(*)::bigint FROM session_messages WHERE vault_id = $1`
	if err := p.pool.QueryRow(ctx, qSessTot, vaultID).Scan(&out.SessionMessagesTotal); err != nil {
		return nil, err
	}

	const qSessBy = `
		SELECT user_id, COUNT(*)::bigint FROM session_messages
		WHERE vault_id = $1 GROUP BY user_id`
	srows, err := p.pool.Query(ctx, qSessBy, vaultID)
	if err != nil {
		return nil, err
	}
	for srows.Next() {
		var uid string
		var n int64
		if err := srows.Scan(&uid, &n); err != nil {
			srows.Close()
			return nil, err
		}
		out.SessionMessagesByUser[uid] = n
	}
	if err := srows.Err(); err != nil {
		return nil, err
	}
	srows.Close()

	const qRecent = `
		SELECT ih.id, ih.chunk_id, ih.event, ih.created_at, COALESCE(mc.user_id, '') AS user_id
		FROM ingest_history ih
		LEFT JOIN memory_chunks mc ON mc.id = ih.chunk_id
		WHERE ih.vault_id = $1
		ORDER BY ih.created_at DESC
		LIMIT $2`
	rrows, err := p.pool.Query(ctx, qRecent, vaultID, recentLimit)
	if err != nil {
		return nil, err
	}
	defer rrows.Close()
	for rrows.Next() {
		var row IngestActivityRow
		var chunkID pgtype.UUID
		if err := rrows.Scan(&row.ID, &chunkID, &row.Event, &row.CreatedAt, &row.UserID); err != nil {
			return nil, err
		}
		if chunkID.Valid {
			u := uuid.UUID(chunkID.Bytes)
			row.ChunkID = &u
		}
		out.RecentActivity = append(out.RecentActivity, row)
	}
	return out, rrows.Err()
}

// GetServerTuning returns raw JSON for server_tuning.overrides (global, not per-vault).
func (p *Pool) GetServerTuning(ctx context.Context) ([]byte, error) {
	const q = `SELECT overrides FROM server_tuning WHERE id = 1`
	var b []byte
	err := p.pool.QueryRow(ctx, q).Scan(&b)
	if errors.Is(err, pgx.ErrNoRows) {
		return []byte("{}"), nil
	}
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return []byte("{}"), nil
	}
	return b, nil
}

// SetServerTuningOverrides replaces the entire overrides JSONB object.
func (p *Pool) SetServerTuningOverrides(ctx context.Context, raw []byte) error {
	const q = `UPDATE server_tuning SET overrides = $1::jsonb, updated_at = now() WHERE id = 1`
	tag, err := p.pool.Exec(ctx, q, raw)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("server_tuning row missing")
	}
	return nil
}
