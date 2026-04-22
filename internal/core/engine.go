package core

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/z-ghostshell/ghostvault/internal/auth"
	"github.com/z-ghostshell/ghostvault/internal/config"
	"github.com/z-ghostshell/ghostvault/internal/crypto"
	"github.com/z-ghostshell/ghostvault/internal/providers"
	"github.com/z-ghostshell/ghostvault/internal/store"
)

var ErrOpenAIRequired = errors.New("OPENAI_API_KEY required for this operation")

// RetrieveOpts configures optional session-scoped behavior for hybrid search.
// SessionMode is "all" (default), "only" (restrict to ingest_session_key), or "prefer" (boost matching session after fusion).
// ContentMode controls which text fields are returned: "auto" (default) uses abstract as primary when present, else body;
// "abstract", "full", and "both" match memory_save structured retrieval (see ParseChunkPlaintext).
type RetrieveOpts struct {
	SessionKey   string
	SessionMode  string
	ContentMode  string
}

func (e *Engine) boostSessionPreference(ranked *[]ScoredChunk, sessionKey string, byChunk map[uuid.UUID]*string) {
	if e.Tuning == nil || sessionKey == "" || ranked == nil || len(*ranked) == 0 {
		return
	}
	boost := e.Tuning.Current().SessionPreferScoreBoost
	if boost < 1 {
		boost = 1.15
	}
	s := *ranked
	for i := range s {
		sk, ok := byChunk[s[i].ID]
		if !ok || sk == nil || *sk != sessionKey {
			continue
		}
		s[i].Score *= boost
	}
	sort.Slice(s, func(i, j int) bool {
		if s[i].Score == s[j].Score {
			return s[i].ID.String() < s[j].ID.String()
		}
		return s[i].Score > s[j].Score
	})
	*ranked = s
}

type Engine struct {
	Cfg    *config.Config
	Tuning *config.TuningState
	Store  *store.Pool
	Sess   *auth.Manager
	Crypto *crypto.VaultCrypto
	OA     *providers.OpenAIClient
}

func (e *Engine) tuning() config.RuntimeTuning {
	if e.Tuning == nil {
		return config.DefaultRuntimeTuning()
	}
	return e.Tuning.Current()
}

func (e *Engine) Retrieve(ctx context.Context, vaultID uuid.UUID, userID, query string, maxChunks int, maxTokens int, session *auth.Session, opts RetrieveOpts) ([]RetrieveSnippet, error) {
	snips, _, err := e.RetrieveWithDebug(ctx, vaultID, userID, query, maxChunks, maxTokens, session, opts, false)
	return snips, err
}

// RetrieveDebugInfo is included in POST /v1/retrieve when `retrieve_debug` in runtime tuning is true and the client sets "debug": true.
type RetrieveDebugInfo struct {
	Params            RetrieveDebugParams `json:"params"`
	DenseCandidates   []DenseDebugRow     `json:"dense_candidates"`
	LexicalCandidates []LexicalDebugRow   `json:"lexical_candidates"`
	FusedRanked       []FusedDebugRow     `json:"fused_ranked"`
	Packing           PackingDebug        `json:"packing"`
}

// RetrieveDebugParams records fusion and fetch limits for the call.
type RetrieveDebugParams struct {
	SemanticThreshold   float64 `json:"semantic_threshold"`
	FusionLexicalWeight float64 `json:"fusion_lexical_weight"`
	EmbeddingModel      string  `json:"embedding_model"`
	KDense              int     `json:"k_dense"`
	MaxChunks           int     `json:"max_chunks"`
	MaxTokens           int     `json:"max_tokens"`
	SessionMode         string  `json:"session_mode"`
	SessionKey          string  `json:"session_key,omitempty"`
	ContentMode         string  `json:"content_mode,omitempty"`
}

// DenseDebugRow is one vector-search candidate.
type DenseDebugRow struct {
	ChunkID         string  `json:"chunk_id"`
	SemanticScore   float64 `json:"semantic_score"`
	PassedThreshold bool    `json:"passed_threshold"`
}

// LexicalDebugRow is one full-text hit.
type LexicalDebugRow struct {
	ChunkID     string  `json:"chunk_id"`
	RawLexRank  float64 `json:"raw_lex_rank"`
	LexicalNorm float64 `json:"lexical_norm"`
}

// FusedDebugRow is one chunk after hybrid fusion, in post-boost ranking order.
type FusedDebugRow struct {
	ChunkID             string  `json:"chunk_id"`
	SemanticScore       float64 `json:"semantic_score"`
	LexicalNorm         float64 `json:"lexical_norm"`
	FusedScore          float64 `json:"fused_score"`
	SessionBoostApplied bool    `json:"session_boost_applied"`
	ScoreAfterSession   float64 `json:"score_after_session"`
}

// PackingDebug describes which chunks were returned and which were skipped in the packing pass.
type PackingDebug struct {
	Included                []PackingIncludedRow `json:"included"`
	SkippedDueToTokenBudget []PackingSkipRow     `json:"skipped_due_to_token_budget"`
	SkippedDueToMaxChunks   []string             `json:"skipped_due_to_max_chunks"`
	StoppedDueToMaxChunks   bool                 `json:"stopped_due_to_max_chunks"`
}

// PackingIncludedRow is one snippet returned in results.
type PackingIncludedRow struct {
	ChunkID         string `json:"chunk_id"`
	EstimatedTokens int    `json:"estimated_tokens"`
}

// PackingSkipRow is a ranked chunk that was skipped because of max_tokens.
type PackingSkipRow struct {
	ChunkID         string `json:"chunk_id"`
	EstimatedTokens int    `json:"estimated_tokens"`
	Reason          string `json:"reason"`
}

// RetrieveWithDebug runs hybrid retrieval. When wantDebug is false, dbg is always nil.
func (e *Engine) RetrieveWithDebug(ctx context.Context, vaultID uuid.UUID, userID, query string, maxChunks int, maxTokens int, session *auth.Session, opts RetrieveOpts, wantDebug bool) ([]RetrieveSnippet, *RetrieveDebugInfo, error) {
	if e.Cfg.Encryption == config.EncryptionOn && (session == nil || len(session.DEK) == 0) {
		return nil, nil, fmt.Errorf("vault must be unlocked")
	}
	if e.OA == nil || e.Cfg.OpenAIAPIKey == "" {
		return nil, nil, ErrOpenAIRequired
	}
	tn := e.tuning()
	if maxChunks <= 0 {
		maxChunks = tn.RetrieveDefaultMaxChunks
		if maxChunks <= 0 {
			maxChunks = 8
		}
	}
	limit := maxChunks
	mult := tn.RetrieveKDenseMultiplier
	if mult < 1 {
		mult = 4
	}
	kDense := limit * mult
	if m := tn.RetrieveKDenseMin; m > 0 && kDense < m {
		kDense = m
	}
	vecs, err := e.OA.Embed(ctx, tn.EmbeddingModel, []string{query})
	if err != nil {
		return nil, nil, err
	}
	if len(vecs) == 0 {
		return nil, nil, fmt.Errorf("empty embedding")
	}
	qv := vecs[0]
	mode := strings.ToLower(strings.TrimSpace(opts.SessionMode))
	if mode == "" {
		mode = "all"
	}
	sKey := strings.TrimSpace(opts.SessionKey)
	onlySession := mode == "only" && sKey != ""
	semT := tn.SemanticThreshold
	lw := tn.FusionLexicalWeight
	dense, err := e.Store.SearchDense(ctx, vaultID, userID, tn.EmbeddingModel, qv, kDense, sKey, onlySession)
	if err != nil {
		return nil, nil, err
	}
	lex, err := e.Store.SearchLexical(ctx, vaultID, userID, query, kDense, sKey, onlySession)
	if err != nil {
		return nil, nil, err
	}
	fusionDetail := RankChunksFusionDetail(dense, lex, semT, lw)
	ranked := make([]ScoredChunk, len(fusionDetail))
	for i, fr := range fusionDetail {
		ranked[i] = ScoredChunk{ID: fr.ID, Score: fr.FusedScore}
	}
	fusionByID := make(map[uuid.UUID]FusionRankRow, len(fusionDetail))
	for _, fr := range fusionDetail {
		fusionByID[fr.ID] = fr
	}
	var keyMap map[uuid.UUID]*string
	if mode == "prefer" && sKey != "" && len(ranked) > 0 {
		ids := make([]uuid.UUID, 0, len(ranked))
		for _, r := range ranked {
			ids = append(ids, r.ID)
		}
		var kerr error
		keyMap, kerr = e.Store.IngestSessionKeysByChunkIDs(ctx, ids)
		if kerr == nil {
			e.boostSessionPreference(&ranked, sKey, keyMap)
		}
	}
	var dbg *RetrieveDebugInfo
	reqCM := strings.ToLower(strings.TrimSpace(opts.ContentMode))
	if reqCM == "" {
		reqCM = "auto"
	}
	if wantDebug {
		dbg = &RetrieveDebugInfo{
			Params: RetrieveDebugParams{
				SemanticThreshold:   semT,
				FusionLexicalWeight: lw,
				EmbeddingModel:      tn.EmbeddingModel,
				KDense:              kDense,
				MaxChunks:           maxChunks,
				MaxTokens:           maxTokens,
				SessionMode:         mode,
				SessionKey:          sKey,
				ContentMode:         reqCM,
			},
			Packing: PackingDebug{},
		}
		for _, c := range dense {
			dbg.DenseCandidates = append(dbg.DenseCandidates, DenseDebugRow{
				ChunkID:         c.ChunkID.String(),
				SemanticScore:   c.SemanticScore,
				PassedThreshold: c.SemanticScore >= semT,
			})
		}
		for _, c := range lex {
			n := NormalizeLexical(c.RawLexRank)
			dbg.LexicalCandidates = append(dbg.LexicalCandidates, LexicalDebugRow{
				ChunkID:     c.ChunkID.String(),
				RawLexRank:  c.RawLexRank,
				LexicalNorm: n,
			})
		}
		for _, sc := range ranked {
			fr := fusionByID[sc.ID]
			boosted := false
			if keyMap != nil {
				if sk, ok := keyMap[sc.ID]; ok && sk != nil && *sk == sKey && mode == "prefer" && sKey != "" {
					boosted = true
				}
			}
			dbg.FusedRanked = append(dbg.FusedRanked, FusedDebugRow{
				ChunkID:             sc.ID.String(),
				SemanticScore:       fr.SemanticScore,
				LexicalNorm:         fr.LexicalNorm,
				FusedScore:          fr.FusedScore,
				SessionBoostApplied: boosted,
				ScoreAfterSession:   sc.Score,
			})
		}
	}
	tokensUsed := 0
	var out []RetrieveSnippet
	for i, r := range ranked {
		if len(out) >= maxChunks {
			if wantDebug {
				for _, rest := range ranked[i:] {
					dbg.Packing.SkippedDueToMaxChunks = append(dbg.Packing.SkippedDueToMaxChunks, rest.ID.String())
				}
				dbg.Packing.StoppedDueToMaxChunks = true
			}
			break
		}
		payload, err := e.Store.GetChunkPayload(ctx, r.ID)
		if err != nil {
			continue
		}
		cb, err := e.payloadToChunkBody(ctx, session, payload)
		if err != nil {
			continue
		}
		resMode := resolveRetrieveContentMode(reqCM, cb)
		est := cb.EstimatedReturnTokens(resMode)
		if maxTokens > 0 && tokensUsed+est > maxTokens {
			if wantDebug {
				dbg.Packing.SkippedDueToTokenBudget = append(dbg.Packing.SkippedDueToTokenBudget, PackingSkipRow{
					ChunkID:         r.ID.String(),
					EstimatedTokens: est,
					Reason:          "exceeds max_tokens budget with prior snippets",
				})
			}
			continue
		}
		tokensUsed += est
		out = append(out, e.buildRetrieveSnippet(r.ID.String(), cb, r.Score, resMode))
		if wantDebug {
			dbg.Packing.Included = append(dbg.Packing.Included, PackingIncludedRow{ChunkID: r.ID.String(), EstimatedTokens: est})
		}
	}
	if wantDebug {
		return out, dbg, nil
	}
	return out, nil, nil
}

// ChunkGetResult is the JSON body for GET /v1/chunks/{id}.
type ChunkGetResult struct {
	ChunkID            string         `json:"chunk_id"`
	UserID             string         `json:"user_id"`
	Text               string         `json:"text"` // full body (same as pre-abstract API)
	Abstract           string         `json:"abstract,omitempty"`
	Kind               string         `json:"kind,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
	SourceDocumentID   string         `json:"source_document_id,omitempty"`
	EmbeddingModelID   string         `json:"embedding_model_id"`
	CreatedAt          time.Time      `json:"created_at"`
	IngestSessionKey   *string        `json:"ingest_session_key,omitempty"`
	ChunkSchemaVersion int            `json:"chunk_schema_version"`
}

// GetChunkByID returns decrypted/plaintext content for a chunk in the vault.
func (e *Engine) GetChunkByID(ctx context.Context, vaultID, chunkID uuid.UUID, session *auth.Session) (*ChunkGetResult, error) {
	if e.Cfg.Encryption == config.EncryptionOn && (session == nil || len(session.DEK) == 0) {
		return nil, fmt.Errorf("vault must be unlocked")
	}
	row, err := e.Store.GetChunkForVault(ctx, vaultID, chunkID)
	if err != nil {
		return nil, err
	}
	p := row.Payload
	cb, err := e.payloadToChunkBody(ctx, session, p)
	if err != nil {
		return nil, err
	}
	res := &ChunkGetResult{
		ChunkID:            p.ID.String(),
		UserID:             row.UserID,
		Text:               cb.Body,
		Abstract:           cb.Abstract,
		Kind:               cb.Kind,
		Metadata:           cb.Metadata,
		EmbeddingModelID:   p.EmbeddingModelID,
		CreatedAt:          row.CreatedAt,
		IngestSessionKey:   row.IngestSessionKey,
		ChunkSchemaVersion: row.ChunkSchemaVersion,
	}
	if p.SourceDocumentID != nil {
		res.SourceDocumentID = p.SourceDocumentID.String()
	}
	return res, nil
}

type RetrieveSnippet struct {
	ChunkID       string  `json:"chunk_id"`
	Text          string  `json:"text"`
	Score         float64 `json:"score"`
	Abstract      string  `json:"abstract,omitempty"`
	FullText      string  `json:"full_text,omitempty"`
	FullAvailable bool    `json:"full_available"`
	Kind          string  `json:"kind,omitempty"`
}

func resolveRetrieveContentMode(req string, cb ChunkBody) string {
	r := strings.ToLower(strings.TrimSpace(req))
	if r == "" || r == "auto" {
		if strings.TrimSpace(cb.Abstract) != "" {
			return "abstract"
		}
		return "full"
	}
	if r == "default" {
		if strings.TrimSpace(cb.Abstract) != "" {
			return "abstract"
		}
		return "full"
	}
	return r
}

func (e *Engine) buildRetrieveSnippet(id string, cb ChunkBody, score float64, resMode string) RetrieveSnippet {
	s := RetrieveSnippet{ChunkID: id, Score: score, Kind: cb.Kind}
	switch resMode {
	case "full":
		s.Text = cb.Body
		s.FullAvailable = false
		return s
	case "both":
		s.Text = strings.TrimSpace(cb.Abstract)
		if s.Text == "" {
			s.Text = cb.Body
		}
		s.Abstract = cb.Abstract
		s.FullText = cb.Body
		s.FullAvailable = strings.TrimSpace(cb.Body) != ""
		return s
	default: // abstract (incl. auto when chunk has an abstract)
		if strings.TrimSpace(cb.Abstract) != "" {
			s.Text = cb.Abstract
			s.Abstract = cb.Abstract
			s.FullAvailable = strings.TrimSpace(cb.Body) != "" &&
				strings.TrimSpace(cb.Abstract) != strings.TrimSpace(cb.Body)
		} else {
			s.Text = cb.Body
			s.FullAvailable = false
		}
		return s
	}
}

func (e *Engine) decryptPayloadString(session *auth.Session, p *store.ChunkPayload) (string, error) {
	if p.BodyText != nil {
		return *p.BodyText, nil
	}
	if len(p.Ciphertext) == 0 {
		return "", fmt.Errorf("empty payload")
	}
	if session == nil || len(session.DEK) == 0 {
		return "", fmt.Errorf("missing dek")
	}
	plain, err := e.Crypto.OpenChunk(session.DEK, p.Nonce, p.Ciphertext)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (e *Engine) decryptPayload(session *auth.Session, p *store.ChunkPayload) (string, error) {
	return e.decryptPayloadString(session, p)
}

func (e *Engine) sourceDocumentPlaintext(session *auth.Session, d *store.SourceDocumentRow) (string, error) {
	if d.BodyText != nil {
		return *d.BodyText, nil
	}
	if len(d.Ciphertext) == 0 {
		return "", fmt.Errorf("empty source document payload")
	}
	if e.Cfg.Encryption == config.EncryptionOn && (session == nil || len(session.DEK) == 0) {
		return "", fmt.Errorf("vault must be unlocked")
	}
	plain, err := e.Crypto.OpenChunk(session.DEK, d.Nonce, d.Ciphertext)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (e *Engine) payloadToChunkBody(ctx context.Context, session *auth.Session, p *store.ChunkPayload) (ChunkBody, error) {
	plain, err := e.decryptPayloadString(session, p)
	if err != nil {
		return ChunkBody{}, err
	}
	cb := ParseChunkPlaintext(plain)
	if strings.TrimSpace(p.ItemKind) != "" && strings.TrimSpace(cb.Kind) == "" {
		cb.Kind = p.ItemKind
	}
	if len(p.ItemMetadata) > 0 {
		var rowMeta map[string]any
		if err := json.Unmarshal(p.ItemMetadata, &rowMeta); err == nil && len(rowMeta) > 0 {
			if cb.Metadata == nil {
				cb.Metadata = rowMeta
			} else {
				for k, v := range rowMeta {
					cb.Metadata[k] = v
				}
			}
		}
	}
	if p.SourceDocumentID == nil {
		return cb, nil
	}
	if p.VaultID == (uuid.UUID{}) {
		return cb, nil
	}
	srow, err := e.Store.GetSourceDocumentForVault(ctx, p.VaultID, *p.SourceDocumentID)
	if err != nil {
		return cb, err
	}
	full, err := e.sourceDocumentPlaintext(session, srow)
	if err != nil {
		return cb, err
	}
	if strings.TrimSpace(cb.Body) == "" {
		cb.Body = full
	}
	return cb, nil
}

func estimateTokens(s string) int {
	if s == "" {
		return 0
	}
	r := utf8.RuneCountInString(s)
	return max(1, r/4)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// IngestItem is one explicit memory to store (used with top-level Ingest or items list).
// Body can be set via "body" or "text" in JSON; at least one of Abstract or body text should be set.
type IngestItem struct {
	Abstract string
	Text     string
	Body     string
	Kind     string
	Metadata map[string]any
}

// SourceDocumentIn is optional: one full source blob shared by all structured items with empty per-item body.
// Items then store only an abstract; the body is read from the source_documents row.
type SourceDocumentIn struct {
	Text  string
	Title string
}

// IngestInput is POST /v1/ingest. When Items or a structured top-level (Abstract, Kind, or Metadata) is
// present, the server stores abstract/body; otherwise the legacy text chunking or infer path runs.
// InferTarget: where LLM-extracted strings go: "abstract" (default) or "body".
// SourceDocument: if set, items with no body get source_document_id and resolve full text on GET/retrieve.
type IngestInput struct {
	VaultID     uuid.UUID
	UserID      string
	SessionKey  string
	Text        string
	Abstract    string
	Kind        string
	Metadata    map[string]any
	Items       []IngestItem
	SourceDocument *SourceDocumentIn
	Infer       bool
	InferTarget string // "abstract" | "body"
	Messages    []IngestMessage
	Session     *auth.Session
	Idempotency string
}

type IngestMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (e *Engine) Ingest(ctx context.Context, in *IngestInput) ([]uuid.UUID, error) {
	ids, _, err := e.IngestWithDebug(ctx, in, false)
	return ids, err
}

// IngestDebugInfo is included in POST /v1/ingest when `retrieve_debug` in runtime tuning is true and the client sets "debug": true.
type IngestDebugInfo struct {
	Segments          []string `json:"segments,omitempty"`
	ExtractedFacts    []string `json:"extracted_facts,omitempty"`
	StructuredSummary []string `json:"structured_summary,omitempty"`
}

// IngestWithDebug persists memories from text or infer; when wantDebug is false, dbg is always nil.
func (e *Engine) IngestWithDebug(ctx context.Context, in *IngestInput, wantDebug bool) ([]uuid.UUID, *IngestDebugInfo, error) {
	if e.OA == nil || e.Cfg.OpenAIAPIKey == "" {
		return nil, nil, ErrOpenAIRequired
	}
	if e.Cfg.Encryption == config.EncryptionOn && (in.Session == nil || len(in.Session.DEK) == 0) {
		return nil, nil, fmt.Errorf("vault must be unlocked")
	}
	tn := e.tuning()

	// 1) Explicit structured items (items[] or top-level abstract/kind/metadata)
	work := collectExplicitStructuredWork(in)
	if len(work) > 0 {
		if err := e.attachSharedSource(ctx, in, &work); err != nil {
			return nil, nil, err
		}
		return e.ingestStructuredWork(ctx, in, work, wantDebug)
	}

	// 2) LLM infer → structured rows (per InferTarget on abstract or body)
	if in.Infer {
		var sb strings.Builder
		lim := tn.RecentSessionMessagesInfer
		if lim < 1 {
			lim = 20
		}
		msgs, _ := e.Store.RecentSessionMessages(ctx, in.VaultID, in.UserID, in.SessionKey, lim)
		for _, m := range msgs {
			sb.WriteString(m.Role)
			sb.WriteString(": ")
			sb.WriteString(m.Content)
			sb.WriteString("\n")
		}
		if in.Text != "" {
			sb.WriteString(in.Text)
		}
		model := strings.TrimSpace(tn.InferLLMModel)
		if model == "" {
			model = "gpt-4o-mini"
		}
		facts, err := e.OA.ExtractMemories(ctx, model, sb.String())
		if err != nil {
			return nil, nil, err
		}
		target := strings.ToLower(strings.TrimSpace(in.InferTarget))
		if target != "body" {
			target = "abstract"
		}
		var inferWork []ingestWorkItem
		for _, f := range facts {
			f = strings.TrimSpace(f)
			if f == "" {
				continue
			}
			w := ingestWorkItem{Kind: strings.TrimSpace(in.Kind), Metadata: in.Metadata}
			if target == "body" {
				w.Body = f
			} else {
				w.Abstract = f
			}
			inferWork = append(inferWork, w)
		}
		ids, dbg, err := e.ingestStructuredWork(ctx, in, inferWork, wantDebug)
		if err != nil {
			return nil, nil, err
		}
		if wantDebug && dbg != nil {
			dbg.ExtractedFacts = append([]string(nil), facts...)
		}
		return ids, dbg, nil
	}

	// 3) Legacy: chunk text only
	size, over := tn.ChunkRuneSize, tn.ChunkOverlap
	if size < 1 {
		size = 2000
	}
	if over < 0 || over >= size {
		over = 200
	}
	facts := chunkText(in.Text, size, over)
	var dbg *IngestDebugInfo
	if wantDebug {
		dbg = &IngestDebugInfo{Segments: append([]string(nil), facts...)}
	}
	var inserted []uuid.UUID
	seen := map[[32]byte]struct{}{}
	for _, f := range facts {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		h := sha256.Sum256([]byte(strings.ToLower(f)))
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		id, err := e.persistV1TextChunk(ctx, in, f, tn)
		if err != nil {
			return inserted, dbg, err
		}
		if id == uuid.Nil {
			continue
		}
		inserted = append(inserted, id)
	}
	keep := e.tuning().PruneSessionMessagesKeep
	if keep < 1 {
		keep = 100
	}
	_ = e.Store.PruneSessionMessages(ctx, in.VaultID, in.UserID, in.SessionKey, keep)
	return inserted, dbg, nil
}

func chunkText(s string, size, overlap int) []string {
	if s == "" {
		return nil
	}
	runes := []rune(s)
	if len(runes) <= size {
		return []string{s}
	}
	var out []string
	step := size - overlap
	if step < 1 {
		step = size
	}
	for i := 0; i < len(runes); i += step {
		end := i + size
		if end > len(runes) {
			end = len(runes)
		}
		out = append(out, string(runes[i:end]))
		if end == len(runes) {
			break
		}
	}
	return out
}
