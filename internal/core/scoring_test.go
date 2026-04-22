package core

import (
	"testing"

	"github.com/google/uuid"
	"github.com/z-ghostshell/ghostvault/internal/store"
)

func TestRankChunks_gated(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	dense := []store.ChunkCandidate{
		{ChunkID: id1, SemanticScore: 0.9},
		{ChunkID: id2, SemanticScore: 0.1},
	}
	lex := []store.ChunkCandidate{
		{ChunkID: id1, RawLexRank: 2},
		{ChunkID: id2, RawLexRank: 10},
	}
	out := RankChunks(dense, lex, 0.25, 0.35)
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out))
	}
	if out[0].ID != id1 {
		t.Fatalf("expected id1, got %v", out[0].ID)
	}
}
