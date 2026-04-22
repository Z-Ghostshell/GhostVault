package core

import (
	"math"

	"github.com/google/uuid"
	"github.com/z-ghostshell/ghostvault/internal/store"
)

// NormalizeLexical maps raw ts_rank_cd style scores toward [0,1].
func NormalizeLexical(raw float64) float64 {
	if raw <= 0 {
		return 0
	}
	return raw / (raw + 1)
}

type ScoredChunk struct {
	ID    uuid.UUID
	Score float64
}

// FusionRankRow is one dense candidate after the semantic gate with fusion components (pre session-boost).
type FusionRankRow struct {
	ID            uuid.UUID
	SemanticScore float64
	LexicalNorm   float64
	FusedScore    float64
}

// RankChunksFusionDetail applies the same logic as RankChunks and returns per-row fusion inputs and fused score.
func RankChunksFusionDetail(dense []store.ChunkCandidate, lexical []store.ChunkCandidate, semThreshold float64, lexWeight float64) []FusionRankRow {
	lexNorm := map[uuid.UUID]float64{}
	for _, c := range lexical {
		lexNorm[c.ChunkID] = NormalizeLexical(c.RawLexRank)
	}
	lw := lexWeight
	if lw < 0 {
		lw = 0
	}
	if lw > 1 {
		lw = 1
	}
	sw := 1 - lw
	scores := map[uuid.UUID]float64{}
	sem := map[uuid.UUID]float64{}
	lexN := map[uuid.UUID]float64{}
	var order []uuid.UUID
	for _, c := range dense {
		if c.SemanticScore < semThreshold {
			continue
		}
		lx := lexNorm[c.ChunkID]
		sc := sw*c.SemanticScore + lw*lx
		if _, ok := scores[c.ChunkID]; !ok {
			order = append(order, c.ChunkID)
		}
		scores[c.ChunkID] = sc
		sem[c.ChunkID] = c.SemanticScore
		lexN[c.ChunkID] = lx
	}
	out := make([]FusionRankRow, 0, len(order))
	for _, id := range order {
		out = append(out, FusionRankRow{ID: id, SemanticScore: sem[id], LexicalNorm: lexN[id], FusedScore: scores[id]})
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].FusedScore > out[i].FusedScore {
				out[i], out[j] = out[j], out[i]
			} else if out[j].FusedScore == out[i].FusedScore && out[j].ID.String() < out[i].ID.String() {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// RankChunks applies a semantic floor on dense candidates then weighted fusion of semantic + normalized lexical scores.
func RankChunks(dense []store.ChunkCandidate, lexical []store.ChunkCandidate, semThreshold float64, lexWeight float64) []ScoredChunk {
	rows := RankChunksFusionDetail(dense, lexical, semThreshold, lexWeight)
	out := make([]ScoredChunk, len(rows))
	for i, r := range rows {
		out[i] = ScoredChunk{ID: r.ID, Score: r.FusedScore}
	}
	return out
}

// Sigmoid for optional tuning.
func Sigmoid(x float64) float64 {
	return 1 / (1 + math.Exp(-x))
}
