package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/z-ghostshell/ghostvault/internal/auth"
	"github.com/z-ghostshell/ghostvault/internal/authdebug"
	"github.com/z-ghostshell/ghostvault/internal/config"
	"github.com/z-ghostshell/ghostvault/internal/core"
	"github.com/z-ghostshell/ghostvault/internal/crypto"
	"github.com/z-ghostshell/ghostvault/internal/providers"
	"github.com/z-ghostshell/ghostvault/internal/store"
)

type Server struct {
	Cfg    *config.Config
	Tuning *config.TuningState
	Store  *store.Pool
	Sess   *auth.Manager
	Crypto *crypto.VaultCrypto
	OA     *providers.OpenAIClient
	Eng    *core.Engine
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(Recoverer)
	r.Use(RequestID)
	r.Use(AccessLogTuning(s.Tuning))
	r.Use(Timeout(120 * time.Second))
	r.Use(MaxBytesTuning(s.Tuning))

	r.Get("/healthz", s.healthz)
	r.Get("/readyz", s.readyz)

	r.Route("/v1", func(r chi.Router) {
		r.Post("/vault/init", s.vaultInit)
		r.Get("/vault", s.vaultInfo)
		r.Post("/vault/unlock", s.vaultUnlock)
		r.Post("/vault/lock", s.vaultLock)
		r.Post("/tokens/actions", s.mintActionToken)
		r.Post("/ingest", s.ingest)
		r.Post("/retrieve", s.retrieve)
		r.Get("/chunks/{id}", s.getChunk)
		r.Delete("/chunks/{id}", s.deleteChunk)
		r.Get("/stats", s.vaultStats)
		r.Get("/tuning", s.getTuning)
		r.Post("/tuning/reload", s.postTuningReload)
		r.Patch("/tuning", s.patchTuning)
	})
	return r
}

func (s *Server) vaultInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	v, err := s.Store.GetAnyVault(ctx)
	if err != nil {
		WriteProblem(w, r, http.StatusInternalServerError, "Vault lookup failed", err.Error(), "store")
		return
	}
	if v == nil {
		WriteProblem(w, r, http.StatusNotFound, "Not Found", "vault not initialized", "no_vault")
		return
	}
	// Public probe: do not expose vault_id, encryption, or auth_mode without a session.
	writeJSON(w, http.StatusOK, map[string]any{"initialized": true})
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.Ping(r.Context()); err != nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, "Not Ready", err.Error(), "db")
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

type initBody struct {
	Password string `json:"password"`
	AuthMode string `json:"auth_mode"`
}

func (s *Server) vaultInit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	existing, err := s.Store.GetAnyVault(ctx)
	if err != nil {
		WriteProblem(w, r, http.StatusInternalServerError, "Init failed", err.Error(), "store")
		return
	}
	if existing != nil {
		WriteProblem(w, r, http.StatusConflict, "Vault exists", "vault already initialized", "vault_exists")
		return
	}
	var body initBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body", "json")
		return
	}
	wantEnc := s.Cfg.Encryption == config.EncryptionOn
	if wantEnc && strings.TrimSpace(body.Password) == "" {
		WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "password required when encryption is on", "password")
		return
	}
	authMode := strings.TrimSpace(body.AuthMode)
	if authMode == "" {
		authMode = string(auth.AuthModeSession)
	}
	if !auth.IsValidAuthMode(authMode) {
		WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid auth_mode (must be session, auto_unlock, or api_keys)", "invalid_auth_mode")
		return
	}
	if auth.AuthMode(authMode) == auth.AuthModeAPIKeys {
		WriteProblem(w, r, http.StatusBadRequest, "Not supported", "auth_mode=api_keys is not yet implemented; use session or auto_unlock", "mode_not_implemented")
		return
	}
	vc := s.Crypto
	if wantEnc {
		dek, err := vc.NewDEK()
		if err != nil {
			WriteProblem(w, r, http.StatusInternalServerError, "Init failed", err.Error(), "crypto")
			return
		}
		wrapped, err := vc.WrapDEK(body.Password, dek)
		if err != nil {
			WriteProblem(w, r, http.StatusInternalServerError, "Init failed", err.Error(), "crypto")
			return
		}
		vid, err := s.Store.InsertVault(ctx, true, authMode, wrapped.Salt, wrapped.Blob, int32(wrapped.TimeCost), int32(wrapped.MemoryKB), int32(wrapped.Parallelism))
		if err != nil {
			WriteProblem(w, r, http.StatusInternalServerError, "Init failed", err.Error(), "store")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"vault_id": vid.String(), "encryption_enabled": true, "auth_mode": authMode})
		return
	}
	vid, err := s.Store.InsertVault(ctx, false, authMode, nil, nil, 0, 0, 0)
	if err != nil {
		WriteProblem(w, r, http.StatusInternalServerError, "Init failed", err.Error(), "store")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"vault_id": vid.String(), "encryption_enabled": false, "auth_mode": authMode})
}

type unlockBody struct {
	Password string `json:"password"`
}

func (s *Server) vaultUnlock(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	v, err := s.Store.GetAnyVault(ctx)
	if err != nil || v == nil {
		WriteProblem(w, r, http.StatusNotFound, "Not Found", "vault not initialized", "no_vault")
		return
	}
	if auth.AuthMode(v.AuthMode) == auth.AuthModeAPIKeys {
		WriteProblem(w, r, http.StatusConflict, "Mode mismatch", "vault auth_mode=api_keys does not use /v1/vault/unlock", "mode_mismatch")
		return
	}
	now := time.Now()
	if !v.EncryptionEnabled {
		tok, err := s.Sess.CreateSession(ctx, v.ID, nil, now)
		if err != nil {
			WriteProblem(w, r, http.StatusInternalServerError, "Unlock failed", err.Error(), "session")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"session_token": tok, "vault_id": v.ID.String(), "encryption_enabled": false, "auth_mode": v.AuthMode})
		return
	}
	var body unlockBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Password) == "" {
		WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "password required", "password")
		return
	}
	if v.ArgonTime == nil || v.ArgonMemoryKB == nil || v.ArgonParallelism == nil {
		WriteProblem(w, r, http.StatusInternalServerError, "Vault corrupt", "missing KDF parameters", "vault")
		return
	}
	wd := &crypto.WrappedDEK{
		Salt:        v.ArgonSalt,
		TimeCost:    uint32(*v.ArgonTime),
		MemoryKB:    uint32(*v.ArgonMemoryKB),
		Parallelism: uint8(*v.ArgonParallelism),
		Blob:        v.WrappedDEK,
	}
	dek, err := s.Crypto.UnwrapDEK(body.Password, wd)
	if err != nil {
		WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "invalid password", "auth")
		return
	}
	tok, err := s.Sess.CreateSession(ctx, v.ID, dek, now)
	if err != nil {
		WriteProblem(w, r, http.StatusInternalServerError, "Unlock failed", err.Error(), "session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"session_token": tok, "vault_id": v.ID.String(), "encryption_enabled": true, "auth_mode": v.AuthMode})
}

func (s *Server) vaultLock(w http.ResponseWriter, r *http.Request) {
	tok := sessionFromRequest(r)
	if tok == "" {
		WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "missing session", "session")
		return
	}
	s.Sess.Invalidate(r.Context(), tok)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) mintActionToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess, err := s.requireSession(r)
	if err != nil {
		s.writeAuthProblem(w, r, err)
		return
	}
	vrow, err := s.Store.GetVault(ctx, sess.VaultID)
	if err == nil && vrow != nil && vrow.EncryptionEnabled {
		WriteProblem(w, r, http.StatusBadRequest, "Not supported", "use session token as Bearer for encrypted vaults", "token")
		return
	}
	var body struct {
		Scopes []string `json:"scopes"`
		TTLSec int      `json:"ttl_seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body", "json")
		return
	}
	if len(body.Scopes) == 0 {
		body.Scopes = []string{"retrieve", "ingest"}
	}
	ttl := time.Duration(body.TTLSec) * time.Second
	if ttl <= 0 {
		ttl = time.Hour
	}
	raw, err := auth.NewRandomToken()
	if err != nil {
		WriteProblem(w, r, http.StatusInternalServerError, "Token mint failed", err.Error(), "token")
		return
	}
	hash := auth.HashActionToken(raw)
	_, err = s.Store.InsertActionToken(ctx, sess.VaultID, hash, body.Scopes, time.Now().Add(ttl))
	if err != nil {
		WriteProblem(w, r, http.StatusInternalServerError, "Token mint failed", err.Error(), "store")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"token": raw, "expires_in_seconds": int(ttl.Seconds())})
}

type ingestItemReq struct {
	Abstract string         `json:"abstract"`
	Text     string         `json:"text"`
	Body     string         `json:"body"`
	Kind     string         `json:"kind"`
	Metadata map[string]any `json:"metadata"`
}

type sourceDocumentReq struct {
	Text  string `json:"text"`
	Title string `json:"title"`
}

type ingestReq struct {
	VaultID          string               `json:"vault_id"`
	UserID           string               `json:"user_id"`
	SessionKey       string               `json:"session_key"`
	Text             string               `json:"text"`
	Abstract         string               `json:"abstract"`
	Kind             string               `json:"kind"`
	Metadata         map[string]any       `json:"metadata"`
	Items            []ingestItemReq      `json:"items"`
	SourceDocument   *sourceDocumentReq   `json:"source_document"`
	Infer            bool                 `json:"infer"`
	InferTarget      string               `json:"infer_target"`
	Messages         []core.IngestMessage `json:"messages"`
	IdempotencyKey   string               `json:"idempotency_key"`
	Debug            bool                 `json:"debug"`
}

func (s *Server) ingest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess, vaultID, err := s.requireVaultAuth(r, ctx)
	if err != nil {
		s.writeAuthProblem(w, r, err)
		return
	}
	var req ingestReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error(), "json")
		return
	}
	if req.VaultID == "" || req.UserID == "" {
		WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "vault_id and user_id required", "validate")
		return
	}
	vid, err := uuid.Parse(req.VaultID)
	if err != nil {
		WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid vault_id", "validate")
		return
	}
	if vid != vaultID {
		WriteProblem(w, r, http.StatusForbidden, "Forbidden", "vault mismatch", "vault")
		return
	}
	if req.SessionKey == "" {
		req.SessionKey = "default"
	}
	for _, m := range req.Messages {
		_ = s.Store.AppendSessionMessage(ctx, vid, req.UserID, req.SessionKey, m.Role, m.Content)
	}
	items := make([]core.IngestItem, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, core.IngestItem{
			Abstract: it.Abstract,
			Text:     it.Text,
			Body:     it.Body,
			Kind:     it.Kind,
			Metadata: it.Metadata,
		})
	}
	var src *core.SourceDocumentIn
	if req.SourceDocument != nil && strings.TrimSpace(req.SourceDocument.Text) != "" {
		src = &core.SourceDocumentIn{
			Text:  strings.TrimSpace(req.SourceDocument.Text),
			Title: strings.TrimSpace(req.SourceDocument.Title),
		}
	}
	in := &core.IngestInput{
		VaultID:        vid,
		UserID:         req.UserID,
		SessionKey:     req.SessionKey,
		Text:           req.Text,
		Abstract:       req.Abstract,
		Kind:           req.Kind,
		Metadata:       req.Metadata,
		Items:          items,
		SourceDocument: src,
		Infer:          req.Infer,
		InferTarget:    req.InferTarget,
		Messages:       req.Messages,
		Session:        sess,
		Idempotency:    req.IdempotencyKey,
	}
	wantIngestDbg := s.Tuning.Current().RetrieveDebug && req.Debug
	ids, ingestDbg, err := s.Eng.IngestWithDebug(ctx, in, wantIngestDbg)
	if err != nil {
		WriteProblem(w, r, http.StatusInternalServerError, "Ingest failed", err.Error(), "ingest")
		return
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	resp := map[string]any{"chunk_ids": out}
	if wantIngestDbg && ingestDbg != nil {
		resp["debug"] = ingestDbg
	}
	writeJSON(w, http.StatusOK, resp)
}

type retrieveReq struct {
	VaultID   string `json:"vault_id"`
	UserID    string `json:"user_id"`
	Query     string `json:"query"`
	MaxChunks int    `json:"max_chunks"`
	MaxTokens int    `json:"max_tokens"`
	// SessionKey with SessionMode "only" restricts search to chunks ingested with this key; "prefer" boosts them after hybrid fusion. Optional for "all".
	SessionKey  string `json:"session_key"`
	SessionMode string `json:"session_mode"`
	// ContentMode: auto (default), abstract, full, or both — which fields appear in each hit (see API docs).
	ContentMode string `json:"content_mode"`
	Debug       bool   `json:"debug"`
}

// buildRetrieveMeta is additive response metadata for clients and agents; always includes result_count.
func buildRetrieveMeta(resultCount int) map[string]any {
	m := map[string]any{
		"result_count": resultCount,
	}
	if resultCount == 0 {
		m["hint"] = "No matching memories. Try different keywords, confirm user_id matches ingested data, call memory_stats to see activity, or ingest with memory_save."
	}
	return m
}

func (s *Server) retrieve(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess, vaultID, err := s.requireVaultAuth(r, ctx)
	if err != nil {
		s.writeAuthProblem(w, r, err)
		return
	}
	var req retrieveReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error(), "json")
		return
	}
	if req.VaultID == "" || req.UserID == "" || strings.TrimSpace(req.Query) == "" {
		WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "vault_id, user_id, query required", "validate")
		return
	}
	vid, err := uuid.Parse(req.VaultID)
	if err != nil || vid != vaultID {
		WriteProblem(w, r, http.StatusForbidden, "Forbidden", "vault mismatch", "vault")
		return
	}
	mode := strings.ToLower(strings.TrimSpace(req.SessionMode))
	if mode == "" {
		mode = "all"
	}
	switch mode {
	case "all", "only", "prefer":
	default:
		WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "session_mode must be all, only, or prefer", "validate")
		return
	}
	if mode == "only" && strings.TrimSpace(req.SessionKey) == "" {
		WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "session_key is required when session_mode is only", "validate")
		return
	}
	cm := strings.ToLower(strings.TrimSpace(req.ContentMode))
	if cm == "" {
		cm = "auto"
	}
	switch cm {
	case "auto", "abstract", "full", "both", "default":
	default:
		WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "content_mode must be auto, abstract, full, both, or default", "validate")
		return
	}
	opts := core.RetrieveOpts{SessionKey: req.SessionKey, SessionMode: mode, ContentMode: cm}
	wantDbg := s.Tuning.Current().RetrieveDebug && req.Debug
	snips, retDbg, err := s.Eng.RetrieveWithDebug(ctx, vid, req.UserID, req.Query, req.MaxChunks, req.MaxTokens, sess, opts, wantDbg)
	if err != nil {
		WriteProblem(w, r, http.StatusInternalServerError, "Retrieve failed", err.Error(), "retrieve")
		return
	}
	resp := map[string]any{
		"results": snips,
		"meta":    buildRetrieveMeta(len(snips)),
	}
	if wantDbg && retDbg != nil {
		resp["debug"] = retDbg
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) getChunk(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess, vaultID, err := s.requireVaultAuth(r, ctx)
	if err != nil {
		s.writeAuthProblem(w, r, err)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid id", "validate")
		return
	}
	res, err := s.Eng.GetChunkByID(ctx, vaultID, id, sess)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			WriteProblem(w, r, http.StatusNotFound, "Not Found", "chunk not found", "not_found")
			return
		}
		if strings.Contains(err.Error(), "vault must be unlocked") {
			s.writeAuthProblem(w, r, auth.ErrVaultLocked)
			return
		}
		WriteProblem(w, r, http.StatusInternalServerError, "Get failed", err.Error(), "chunk")
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) deleteChunk(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_, vaultID, err := s.requireVaultAuth(r, ctx)
	if err != nil {
		s.writeAuthProblem(w, r, err)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid id", "validate")
		return
	}
	if err := s.Store.SoftDeleteChunk(ctx, vaultID, id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			WriteProblem(w, r, http.StatusNotFound, "Not Found", "chunk not found", "not_found")
			return
		}
		WriteProblem(w, r, http.StatusInternalServerError, "Delete failed", err.Error(), "store")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) vaultStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_, vaultID, err := s.requireVaultAuth(r, ctx)
	if err != nil {
		s.writeAuthProblem(w, r, err)
		return
	}
	vrow, err := s.Store.GetVault(ctx, vaultID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			WriteProblem(w, r, http.StatusNotFound, "Not Found", "vault not found", "not_found")
			return
		}
		WriteProblem(w, r, http.StatusInternalServerError, "Stats failed", err.Error(), "store")
		return
	}
	st, err := s.Store.StatsForVault(ctx, vaultID, 50)
	if err != nil {
		WriteProblem(w, r, http.StatusInternalServerError, "Stats failed", err.Error(), "store")
		return
	}
	recent := make([]map[string]any, 0, len(st.RecentActivity))
	for _, row := range st.RecentActivity {
		m := map[string]any{
			"id":         row.ID,
			"event":      row.Event,
			"created_at": row.CreatedAt.UTC().Format(time.RFC3339Nano),
		}
		if row.ChunkID != nil {
			m["chunk_id"] = row.ChunkID.String()
		}
		if row.UserID != "" {
			m["user_id"] = row.UserID
		}
		recent = append(recent, m)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"vault_id":           vaultID.String(),
		"encryption_enabled": vrow.EncryptionEnabled,
		"chunks_total":       st.ChunksTotal,
		"chunks_by_user":     st.ChunksByUser,
		"ingest_events": map[string]any{
			"total":    st.IngestEventsTotal,
			"last_24h": st.IngestEventsLast24h,
			"last_7d":  st.IngestEventsLast7d,
		},
		"session_messages": map[string]any{
			"total":   st.SessionMessagesTotal,
			"by_user": st.SessionMessagesByUser,
		},
		"recent_activity": recent,
	})
}

func sessionFromRequest(r *http.Request) string {
	if h := r.Header.Get("X-Ghost-Session"); h != "" {
		return h
	}
	if authz := r.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		return strings.TrimSpace(authz[7:])
	}
	return ""
}

func (s *Server) writeAuthProblem(w http.ResponseWriter, r *http.Request, err error) {
	s.authDebugLog(r, err)
	if errors.Is(err, auth.ErrAuthBackend) {
		WriteProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "authentication backend unavailable", "auth_backend")
		return
	}
	switch {
	case errors.Is(err, auth.ErrVaultLocked):
		WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "vault is locked or session expired", "auth")
	default:
		WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "invalid or missing credentials", "auth")
	}
}

func (s *Server) authDebugLog(r *http.Request, authErr error) {
	if s == nil || s.Cfg == nil || !s.Cfg.DebugAuthLog {
		return
	}
	rid, _ := r.Context().Value(requestIDKey).(string)
	raw := sessionFromRequest(r)
	authz := r.Header.Get("Authorization")
	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		xff = "(none)"
	}
	reasonClass := "other"
	switch {
	case errors.Is(authErr, auth.ErrVaultLocked):
		reasonClass = "vault_locked_or_session_expired"
	case errors.Is(authErr, auth.ErrUnauthorized):
		reasonClass = "unauthorized_bad_or_unknown_token"
	}
	log.Printf("auth debug request_id=%s path=%s method=%s remote=%s x_forwarded_for=%s user_agent=%q err=%q reason_class=%s authorization_set=%v authorization_len=%d x_ghost_session_set=%v token_fp=%s token=%s",
		rid, r.URL.Path, r.Method, r.RemoteAddr, xff, r.Header.Get("User-Agent"),
		authErr.Error(), reasonClass,
		authz != "", len(authz), r.Header.Get("X-Ghost-Session") != "",
		authdebug.Fingerprint(raw), authdebug.ForLog(raw, s.Cfg.DebugAuthFull))
	log.Printf("auth debug request_id=%s note=%s",
		rid, "this route expects Ghost Vault session bearer (gvmcp→gvsvd). Claude OAuth bearer is validated only on gvmcp; empty/mismatched vault token yields 401 here.")
	if s.Cfg.DebugAuthFull && strings.TrimSpace(raw) != "" {
		log.Printf("auth debug request_id=%s WARNING GV_DEBUG_AUTH_FULL=true: full token logged above", rid)
	}
}

func (s *Server) requireSession(r *http.Request) (*auth.Session, error) {
	tok := sessionFromRequest(r)
	if tok == "" {
		return nil, auth.ErrUnauthorized
	}
	return s.Sess.Touch(r.Context(), tok, time.Now())
}

func (s *Server) requireVaultAuth(r *http.Request, ctx context.Context) (*auth.Session, uuid.UUID, error) {
	authz := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		raw := strings.TrimSpace(authz[7:])
		if sess, err := s.Sess.Touch(ctx, raw, time.Now()); err == nil {
			return sess, sess.VaultID, nil
		}
		vid, err := s.actionTokenVault(ctx, raw)
		if err != nil {
			return nil, uuid.Nil, fmt.Errorf("%w: %v", auth.ErrAuthBackend, err)
		}
		if vid == nil {
			return nil, uuid.Nil, auth.ErrUnauthorized
		}
		row, err := s.Store.GetVault(ctx, *vid)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return nil, uuid.Nil, auth.ErrUnauthorized
			}
			return nil, uuid.Nil, fmt.Errorf("%w: %v", auth.ErrAuthBackend, err)
		}
		if row.EncryptionEnabled {
			return nil, uuid.Nil, auth.ErrUnauthorized
		}
		return &auth.Session{VaultID: *vid}, *vid, nil
	}
	sess, err := s.requireSession(r)
	if err != nil {
		return nil, uuid.Nil, err
	}
	return sess, sess.VaultID, nil
}

func (s *Server) actionTokenVault(ctx context.Context, raw string) (*uuid.UUID, error) {
	h := auth.HashActionToken(raw)
	return s.Store.ValidActionToken(ctx, h)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
