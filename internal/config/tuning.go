package config

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// RuntimeTuning holds algorithm and operational settings merged from defaults, file, and DB.
// See configs/gvsvd.yaml. Secrets and endpoint URLs are not in this struct.
type RuntimeTuning struct {
	SemanticThreshold          float64 `json:"semantic_threshold" yaml:"semantic_threshold"`
	FusionLexicalWeight        float64 `json:"fusion_lexical_weight" yaml:"fusion_lexical_weight"`
	RetrieveDefaultMaxChunks   int     `json:"retrieve_default_max_chunks" yaml:"retrieve_default_max_chunks"`
	RetrieveKDenseMin          int     `json:"retrieve_k_dense_min" yaml:"retrieve_k_dense_min"`
	RetrieveKDenseMultiplier   int     `json:"retrieve_k_dense_multiplier" yaml:"retrieve_k_dense_multiplier"`
	SessionPreferScoreBoost    float64 `json:"session_prefer_score_boost" yaml:"session_prefer_score_boost"`
	ChunkRuneSize              int     `json:"chunk_rune_size" yaml:"chunk_rune_size"`
	ChunkOverlap               int     `json:"chunk_overlap" yaml:"chunk_overlap"`
	RecentSessionMessagesInfer int     `json:"recent_session_messages_for_infer" yaml:"recent_session_messages_for_infer"`
	PruneSessionMessagesKeep   int     `json:"prune_session_messages_keep" yaml:"prune_session_messages_keep"`
	InferLLMModel              string  `json:"infer_llm_model" yaml:"infer_llm_model"`
	EmbeddingModel             string  `json:"embedding_model" yaml:"embedding_model"`
	MaxBodyBytes               int64   `json:"max_body_bytes" yaml:"max_body_bytes"`
	RetrieveDebug              bool    `json:"retrieve_debug" yaml:"retrieve_debug"`
	HTTPAccessLog              bool    `json:"http_access_log" yaml:"http_access_log"`
}

// tuningPartial uses pointers so YAML/JSON can distinguish unset vs set-to-zero.
type tuningPartial struct {
	SemanticThreshold          *float64 `json:"semantic_threshold" yaml:"semantic_threshold"`
	FusionLexicalWeight        *float64 `json:"fusion_lexical_weight" yaml:"fusion_lexical_weight"`
	RetrieveDefaultMaxChunks   *int     `json:"retrieve_default_max_chunks" yaml:"retrieve_default_max_chunks"`
	RetrieveKDenseMin          *int     `json:"retrieve_k_dense_min" yaml:"retrieve_k_dense_min"`
	RetrieveKDenseMultiplier   *int     `json:"retrieve_k_dense_multiplier" yaml:"retrieve_k_dense_multiplier"`
	SessionPreferScoreBoost    *float64 `json:"session_prefer_score_boost" yaml:"session_prefer_score_boost"`
	ChunkRuneSize              *int     `json:"chunk_rune_size" yaml:"chunk_rune_size"`
	ChunkOverlap               *int     `json:"chunk_overlap" yaml:"chunk_overlap"`
	RecentSessionMessagesInfer *int     `json:"recent_session_messages_for_infer" yaml:"recent_session_messages_for_infer"`
	PruneSessionMessagesKeep   *int     `json:"prune_session_messages_keep" yaml:"prune_session_messages_keep"`
	InferLLMModel              *string  `json:"infer_llm_model" yaml:"infer_llm_model"`
	EmbeddingModel             *string  `json:"embedding_model" yaml:"embedding_model"`
	MaxBodyBytes               *int64   `json:"max_body_bytes" yaml:"max_body_bytes"`
	RetrieveDebug              *bool    `json:"retrieve_debug" yaml:"retrieve_debug"`
	HTTPAccessLog              *bool    `json:"http_access_log" yaml:"http_access_log"`
}

// DefaultRuntimeTuning is the program default before file or DB.
func DefaultRuntimeTuning() RuntimeTuning {
	return RuntimeTuning{
		SemanticThreshold:          0.25,
		FusionLexicalWeight:        0.35,
		RetrieveDefaultMaxChunks:   16,
		RetrieveKDenseMin:          60,
		RetrieveKDenseMultiplier:   4,
		SessionPreferScoreBoost:    1.15,
		ChunkRuneSize:              2000,
		ChunkOverlap:               200,
		RecentSessionMessagesInfer: 20,
		PruneSessionMessagesKeep:   100,
		InferLLMModel:              "gpt-4o-mini",
		EmbeddingModel:             "text-embedding-3-small",
		MaxBodyBytes:               1 << 20,
		RetrieveDebug:              false,
		HTTPAccessLog:              false,
	}
}

// PatchFields lists JSON keys allowed in PATCH /v1/tuning and DB JSONB overrides.
// embedding_model is excluded from hot DB patches (set via file; reload to apply).
var PatchableTuningKeys = map[string]struct{}{
	"semantic_threshold":                {},
	"fusion_lexical_weight":             {},
	"retrieve_default_max_chunks":       {},
	"retrieve_k_dense_min":              {},
	"retrieve_k_dense_multiplier":       {},
	"session_prefer_score_boost":        {},
	"chunk_rune_size":                   {},
	"chunk_overlap":                     {},
	"recent_session_messages_for_infer": {},
	"prune_session_messages_keep":       {},
	"infer_llm_model":                   {},
	"max_body_bytes":                    {},
	"retrieve_debug":                    {},
	"http_access_log":                   {},
}

// ApplyPartial overlays non-nil fields from p onto a copy of base.
func ApplyPartial(base RuntimeTuning, p *tuningPartial) RuntimeTuning {
	if p == nil {
		return base
	}
	if p.SemanticThreshold != nil {
		base.SemanticThreshold = *p.SemanticThreshold
	}
	if p.FusionLexicalWeight != nil {
		base.FusionLexicalWeight = *p.FusionLexicalWeight
	}
	if p.RetrieveDefaultMaxChunks != nil {
		base.RetrieveDefaultMaxChunks = *p.RetrieveDefaultMaxChunks
	}
	if p.RetrieveKDenseMin != nil {
		base.RetrieveKDenseMin = *p.RetrieveKDenseMin
	}
	if p.RetrieveKDenseMultiplier != nil {
		base.RetrieveKDenseMultiplier = *p.RetrieveKDenseMultiplier
	}
	if p.SessionPreferScoreBoost != nil {
		base.SessionPreferScoreBoost = *p.SessionPreferScoreBoost
	}
	if p.ChunkRuneSize != nil {
		base.ChunkRuneSize = *p.ChunkRuneSize
	}
	if p.ChunkOverlap != nil {
		base.ChunkOverlap = *p.ChunkOverlap
	}
	if p.RecentSessionMessagesInfer != nil {
		base.RecentSessionMessagesInfer = *p.RecentSessionMessagesInfer
	}
	if p.PruneSessionMessagesKeep != nil {
		base.PruneSessionMessagesKeep = *p.PruneSessionMessagesKeep
	}
	if p.InferLLMModel != nil {
		base.InferLLMModel = *p.InferLLMModel
	}
	if p.EmbeddingModel != nil {
		base.EmbeddingModel = *p.EmbeddingModel
	}
	if p.MaxBodyBytes != nil {
		base.MaxBodyBytes = *p.MaxBodyBytes
	}
	if p.RetrieveDebug != nil {
		base.RetrieveDebug = *p.RetrieveDebug
	}
	if p.HTTPAccessLog != nil {
		base.HTTPAccessLog = *p.HTTPAccessLog
	}
	return base
}

// DefaultPlusFilePath loads defaults, merges on-disk file if it exists, validates.
func DefaultPlusFilePath(path string) (RuntimeTuning, error) {
	base := DefaultRuntimeTuning()
	if strings.TrimSpace(path) == "" {
		return base, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return base, nil
		}
		return RuntimeTuning{}, err
	}
	return MergeYAMLBytes(base, b)
}

// MergeYAMLBytes parses YAML and overlays onto base.
func MergeYAMLBytes(base RuntimeTuning, b []byte) (RuntimeTuning, error) {
	if len(bytesTrim(b)) == 0 {
		return base, nil
	}
	var p tuningPartial
	if err := yaml.Unmarshal(b, &p); err != nil {
		return RuntimeTuning{}, err
	}
	t := ApplyPartial(base, &p)
	return t, t.Validate()
}

// MergeFromJSONMap merges only PatchableTuningKeys from m into a JSON partial then ApplyPartial.
func MergeFromJSONMap(base RuntimeTuning, m map[string]json.RawMessage) (RuntimeTuning, error) {
	p := &tuningPartial{}
	for k, v := range m {
		if _, ok := PatchableTuningKeys[k]; !ok {
			return RuntimeTuning{}, fmt.Errorf("unknown or non-overridable tuning key: %q", k)
		}
		switch k {
		case "semantic_threshold":
			if err := json.Unmarshal(v, &p.SemanticThreshold); err != nil {
				return RuntimeTuning{}, err
			}
		case "fusion_lexical_weight":
			if err := json.Unmarshal(v, &p.FusionLexicalWeight); err != nil {
				return RuntimeTuning{}, err
			}
		case "retrieve_default_max_chunks":
			if err := json.Unmarshal(v, &p.RetrieveDefaultMaxChunks); err != nil {
				return RuntimeTuning{}, err
			}
		case "retrieve_k_dense_min":
			if err := json.Unmarshal(v, &p.RetrieveKDenseMin); err != nil {
				return RuntimeTuning{}, err
			}
		case "retrieve_k_dense_multiplier":
			if err := json.Unmarshal(v, &p.RetrieveKDenseMultiplier); err != nil {
				return RuntimeTuning{}, err
			}
		case "session_prefer_score_boost":
			if err := json.Unmarshal(v, &p.SessionPreferScoreBoost); err != nil {
				return RuntimeTuning{}, err
			}
		case "chunk_rune_size":
			if err := json.Unmarshal(v, &p.ChunkRuneSize); err != nil {
				return RuntimeTuning{}, err
			}
		case "chunk_overlap":
			if err := json.Unmarshal(v, &p.ChunkOverlap); err != nil {
				return RuntimeTuning{}, err
			}
		case "recent_session_messages_for_infer":
			if err := json.Unmarshal(v, &p.RecentSessionMessagesInfer); err != nil {
				return RuntimeTuning{}, err
			}
		case "prune_session_messages_keep":
			if err := json.Unmarshal(v, &p.PruneSessionMessagesKeep); err != nil {
				return RuntimeTuning{}, err
			}
		case "infer_llm_model":
			if err := json.Unmarshal(v, &p.InferLLMModel); err != nil {
				return RuntimeTuning{}, err
			}
		case "max_body_bytes":
			if err := json.Unmarshal(v, &p.MaxBodyBytes); err != nil {
				return RuntimeTuning{}, err
			}
		case "retrieve_debug":
			if err := json.Unmarshal(v, &p.RetrieveDebug); err != nil {
				return RuntimeTuning{}, err
			}
		case "http_access_log":
			if err := json.Unmarshal(v, &p.HTTPAccessLog); err != nil {
				return RuntimeTuning{}, err
			}
		}
	}
	t := ApplyPartial(base, p)
	return t, t.Validate()
}

// MergeTuningWithPartial merges after unmarshaling JSON object into partial (DB layer).
func MergeTuningWithPartial(base RuntimeTuning, raw []byte) (RuntimeTuning, error) {
	if len(bytesTrim(raw)) == 0 {
		return base, nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return RuntimeTuning{}, err
	}
	return MergeFromJSONMap(base, m)
}

// MergeTuningOverrideJSON merges patch into existing DB-stored override JSON. Keys must be in PatchableTuningKeys.
func MergeTuningOverrideJSON(existing []byte, patch map[string]json.RawMessage) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(bytesTrimForJSON(existing), &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = make(map[string]json.RawMessage)
	}
	for k, v := range patch {
		if _, ok := PatchableTuningKeys[k]; !ok {
			return nil, fmt.Errorf("unknown or non-overridable tuning key: %q", k)
		}
		m[k] = v
	}
	return json.Marshal(m)
}

func bytesTrimForJSON(b []byte) []byte {
	b = []byte(strings.TrimSpace(string(b)))
	if len(b) == 0 {
		return []byte("{}")
	}
	return b
}

func bytesTrim(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

// FileLayerForSources returns default merged with the YAML file only (for validation and source attribution).
func FileLayerForSources(def RuntimeTuning, filePath string) (RuntimeTuning, error) {
	if strings.TrimSpace(filePath) == "" {
		return def, nil
	}
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return def, nil
		}
		return RuntimeTuning{}, err
	}
	b, err := os.ReadFile(filePath)
	if err != nil {
		return RuntimeTuning{}, err
	}
	return MergeYAMLBytes(def, b)
}

// MergeTuningFull computes effective tuning: def -> file -> optional DB override JSON.
func MergeTuningFull(def RuntimeTuning, filePath string, dbOverrides []byte) (eff RuntimeTuning, fileLayerOut RuntimeTuning, err error) {
	fileLayer, err := FileLayerForSources(def, filePath)
	if err != nil {
		return RuntimeTuning{}, RuntimeTuning{}, err
	}
	eff, err = MergeTuningWithPartial(fileLayer, dbOverrides)
	if err != nil {
		return RuntimeTuning{}, RuntimeTuning{}, err
	}
	if vErr := eff.Validate(); vErr != nil {
		return RuntimeTuning{}, RuntimeTuning{}, vErr
	}
	return eff, fileLayer, nil
}

func tripletFloat(def, fileL, eff float64) string {
	if !floatsClose(eff, fileL) {
		return "db"
	}
	if !floatsClose(fileL, def) {
		return "file"
	}
	return "def"
}

func tripletInt(def, fileL, eff int) string {
	if eff != fileL {
		return "db"
	}
	if fileL != def {
		return "file"
	}
	return "def"
}

func tripletStr(def, fileL, eff string) string {
	if eff != fileL {
		return "db"
	}
	if fileL != def {
		return "file"
	}
	return "def"
}

func tripletBool(def, fileL, eff bool) string {
	if eff != fileL {
		return "db"
	}
	if fileL != def {
		return "file"
	}
	return "def"
}

func tripletI64(def, fileL, eff int64) string {
	if eff != fileL {
		return "db"
	}
	if fileL != def {
		return "file"
	}
	return "def"
}

func floatsClose(a, b float64) bool {
	if a == b {
		return true
	}
	return math.Abs(a-b) < 1e-9
}

// ComputeSources returns map JSON key -> source label {def|file|db} for the effective value.
func ComputeSources(def, fileL, eff RuntimeTuning) map[string]string {
	out := make(map[string]string, 20)
	out["semantic_threshold"] = tripletFloat(def.SemanticThreshold, fileL.SemanticThreshold, eff.SemanticThreshold)
	out["fusion_lexical_weight"] = tripletFloat(def.FusionLexicalWeight, fileL.FusionLexicalWeight, eff.FusionLexicalWeight)
	out["retrieve_default_max_chunks"] = tripletInt(def.RetrieveDefaultMaxChunks, fileL.RetrieveDefaultMaxChunks, eff.RetrieveDefaultMaxChunks)
	out["retrieve_k_dense_min"] = tripletInt(def.RetrieveKDenseMin, fileL.RetrieveKDenseMin, eff.RetrieveKDenseMin)
	out["retrieve_k_dense_multiplier"] = tripletInt(def.RetrieveKDenseMultiplier, fileL.RetrieveKDenseMultiplier, eff.RetrieveKDenseMultiplier)
	out["session_prefer_score_boost"] = tripletFloat(def.SessionPreferScoreBoost, fileL.SessionPreferScoreBoost, eff.SessionPreferScoreBoost)
	out["chunk_rune_size"] = tripletInt(def.ChunkRuneSize, fileL.ChunkRuneSize, eff.ChunkRuneSize)
	out["chunk_overlap"] = tripletInt(def.ChunkOverlap, fileL.ChunkOverlap, eff.ChunkOverlap)
	out["recent_session_messages_for_infer"] = tripletInt(def.RecentSessionMessagesInfer, fileL.RecentSessionMessagesInfer, eff.RecentSessionMessagesInfer)
	out["prune_session_messages_keep"] = tripletInt(def.PruneSessionMessagesKeep, fileL.PruneSessionMessagesKeep, eff.PruneSessionMessagesKeep)
	out["infer_llm_model"] = tripletStr(def.InferLLMModel, fileL.InferLLMModel, eff.InferLLMModel)
	out["embedding_model"] = tripletStr(def.EmbeddingModel, fileL.EmbeddingModel, eff.EmbeddingModel)
	out["max_body_bytes"] = tripletI64(def.MaxBodyBytes, fileL.MaxBodyBytes, eff.MaxBodyBytes)
	out["retrieve_debug"] = tripletBool(def.RetrieveDebug, fileL.RetrieveDebug, eff.RetrieveDebug)
	out["http_access_log"] = tripletBool(def.HTTPAccessLog, fileL.HTTPAccessLog, eff.HTTPAccessLog)
	return out
}

// Validate enforces invariants for RuntimeTuning.
func (t *RuntimeTuning) Validate() error {
	if t.SemanticThreshold < 0 || t.SemanticThreshold > 1 {
		return fmt.Errorf("semantic_threshold must be in [0,1], got %v", t.SemanticThreshold)
	}
	if t.FusionLexicalWeight < 0 || t.FusionLexicalWeight > 1 {
		return fmt.Errorf("fusion_lexical_weight must be in [0,1], got %v", t.FusionLexicalWeight)
	}
	if t.RetrieveDefaultMaxChunks < 1 {
		return fmt.Errorf("retrieve_default_max_chunks must be >= 1")
	}
	if t.RetrieveKDenseMin < 1 {
		return fmt.Errorf("retrieve_k_dense_min must be >= 1")
	}
	if t.RetrieveKDenseMultiplier < 1 {
		return fmt.Errorf("retrieve_k_dense_multiplier must be >= 1")
	}
	if t.SessionPreferScoreBoost < 1 {
		return fmt.Errorf("session_prefer_score_boost must be >= 1")
	}
	if t.ChunkRuneSize < 1 {
		return fmt.Errorf("chunk_rune_size must be >= 1")
	}
	if t.ChunkOverlap < 0 || t.ChunkOverlap >= t.ChunkRuneSize {
		return fmt.Errorf("chunk_overlap invalid")
	}
	if t.RecentSessionMessagesInfer < 1 {
		return fmt.Errorf("recent_session_messages_for_infer must be >= 1")
	}
	if t.PruneSessionMessagesKeep < 1 {
		return fmt.Errorf("prune_session_messages_keep must be >= 1")
	}
	if strings.TrimSpace(t.InferLLMModel) == "" {
		return fmt.Errorf("infer_llm_model required")
	}
	if strings.TrimSpace(t.EmbeddingModel) == "" {
		return fmt.Errorf("embedding_model required")
	}
	if t.MaxBodyBytes < 1 {
		return fmt.Errorf("max_body_bytes must be >= 1")
	}
	return nil
}
