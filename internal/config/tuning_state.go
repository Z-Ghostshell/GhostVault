package config

import (
	"context"
	"encoding/json"
	"sync"
)

// TuningState holds the effective runtime tuning. Reload re-reads the YAML file and DB.
type TuningState struct {
	mu        sync.RWMutex
	def       RuntimeTuning
	filePath  string
	fileLayer RuntimeTuning
	dbJSON    []byte
	current   RuntimeTuning
	loader    func(ctx context.Context) ([]byte, error) // load DB override JSON; nil = no db
}

// NewTuningState loads def + file + optional DB. loader returns raw JSON (may be nil/empty), or an error.
func NewTuningState(tuningFile string, dbLoader func(ctx context.Context) ([]byte, error)) (*TuningState, error) {
	s := &TuningState{
		def:      DefaultRuntimeTuning(),
		filePath: tuningFile,
		loader:   dbLoader,
	}
	if err := s.Reload(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

// Current returns a copy of the effective tuning.
func (s *TuningState) Current() RuntimeTuning {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current
}

// FilePath is the on-disk path used for the last reload.
func (s *TuningState) FilePath() string { return s.filePath }

// Reload merges defaults + file + DB.
func (s *TuningState) Reload(ctx context.Context) error {
	var db []byte
	if s.loader != nil {
		b, err := s.loader(ctx)
		if err != nil {
			return err
		}
		db = b
	}
	eff, fileL, err := MergeTuningFull(s.def, s.filePath, db)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.current = eff
	s.fileLayer = fileL
	s.dbJSON = append([]byte(nil), db...)
	s.mu.Unlock()
	return nil
}

// APISnapshot is used for GET /v1/tuning: effective, sources, raw db overrides.
func (s *TuningState) APISnapshot() (effective RuntimeTuning, fileLayer RuntimeTuning, def RuntimeTuning, dbRaw []byte, sources map[string]string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	eff := s.current
	fl := s.fileLayer
	d := s.def
	raw := append([]byte(nil), s.dbJSON...)
	src := ComputeSources(d, fl, eff)
	return eff, fl, d, raw, src
}

// MergedAPIMap flattens effective tuning to JSON-like values for the dashboard.
func MergedAPIMap(t RuntimeTuning) map[string]any {
	return map[string]any{
		"semantic_threshold":                t.SemanticThreshold,
		"fusion_lexical_weight":             t.FusionLexicalWeight,
		"retrieve_default_max_chunks":       t.RetrieveDefaultMaxChunks,
		"retrieve_k_dense_min":              t.RetrieveKDenseMin,
		"retrieve_k_dense_multiplier":       t.RetrieveKDenseMultiplier,
		"session_prefer_score_boost":        t.SessionPreferScoreBoost,
		"chunk_rune_size":                   t.ChunkRuneSize,
		"chunk_overlap":                     t.ChunkOverlap,
		"recent_session_messages_for_infer": t.RecentSessionMessagesInfer,
		"prune_session_messages_keep":       t.PruneSessionMessagesKeep,
		"infer_llm_model":                   t.InferLLMModel,
		"embedding_model":                   t.EmbeddingModel,
		"max_body_bytes":                    t.MaxBodyBytes,
		"retrieve_debug":                    t.RetrieveDebug,
		"http_access_log":                   t.HTTPAccessLog,
	}
}

// PatchJSONToMap decodes a PATCH body into a map of keys for DB merge.
func PatchJSONToMap(b []byte) (map[string]json.RawMessage, error) {
	if len(b) == 0 {
		return nil, nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}
