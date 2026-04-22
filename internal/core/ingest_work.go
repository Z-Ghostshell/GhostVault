package core

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/z-ghostshell/ghostvault/internal/config"
	"github.com/z-ghostshell/ghostvault/internal/store"
)

type ingestWorkItem struct {
	Abstract         string
	Body             string
	Kind             string
	Metadata         map[string]any
	SourceDocumentID *uuid.UUID
	// SourceTextForTSV is the full source document text in memory for this ingest only (FTS excerpt).
	SourceTextForTSV string
}

func contentDedupKey(kind, abstract, body string, sourceID *uuid.UUID) [32]byte {
	var b strings.Builder
	b.WriteString(strings.ToLower(strings.TrimSpace(kind)))
	b.WriteString("\n")
	b.WriteString(strings.ToLower(strings.TrimSpace(abstract)))
	b.WriteString("\n")
	if sourceID != nil {
		b.WriteString(sourceID.String())
	} else {
		b.WriteString(strings.ToLower(strings.TrimSpace(body)))
	}
	return sha256.Sum256([]byte(b.String()))
}

func needsV2Format(w ingestWorkItem) bool {
	if strings.TrimSpace(w.Abstract) != "" {
		return true
	}
	if strings.TrimSpace(w.Kind) != "" {
		return true
	}
	if w.Metadata != nil && len(w.Metadata) > 0 {
		return true
	}
	return false
}

// collectExplicitStructuredWork returns work items from items[] or top-level abstract/kind/metadata (single row).
func collectExplicitStructuredWork(in *IngestInput) []ingestWorkItem {
	if len(in.Items) > 0 {
		var out []ingestWorkItem
		for _, it := range in.Items {
			b := strings.TrimSpace(it.Body)
			if b == "" {
				b = strings.TrimSpace(it.Text)
			}
			w := ingestWorkItem{
				Abstract: strings.TrimSpace(it.Abstract),
				Body:     b,
				Kind:     strings.TrimSpace(it.Kind),
				Metadata: it.Metadata,
			}
			if w.Abstract == "" && w.Body == "" {
				continue
			}
			out = append(out, w)
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}
	abs := strings.TrimSpace(in.Abstract)
	kind := strings.TrimSpace(in.Kind)
	t := strings.TrimSpace(in.Text)
	hasMeta := in.Metadata != nil && len(in.Metadata) > 0
	if abs == "" && kind == "" && !hasMeta {
		return nil
	}
	return []ingestWorkItem{{
		Abstract: abs,
		Body:     t,
		Kind:     kind,
		Metadata: in.Metadata,
	}}
}

// attachSharedSource sets SourceDocumentID and SourceTextForTSV on work items with empty body when
// in.SourceDocument is present with non-empty text. Inserts or reuses source_documents by content hash.
// Does not create a source_documents row if every work item has an inline body.
func (e *Engine) attachSharedSource(ctx context.Context, in *IngestInput, work *[]ingestWorkItem) error {
	if in.SourceDocument == nil {
		return nil
	}
	full := strings.TrimSpace(in.SourceDocument.Text)
	if full == "" {
		return nil
	}
	need := false
	for i := range *work {
		if strings.TrimSpace((*work)[i].Body) == "" {
			need = true
			break
		}
	}
	if !need {
		return nil
	}
	tn := e.tuning()
	id, err := e.upsertSourceDocument(ctx, in, full, strings.TrimSpace(in.SourceDocument.Title), tn)
	if err != nil {
		return err
	}
	for i := range *work {
		w := &(*work)[i]
		if strings.TrimSpace(w.Body) != "" {
			continue
		}
		uid := id
		w.SourceDocumentID = &uid
		w.SourceTextForTSV = full
	}
	return nil
}

func (e *Engine) upsertSourceDocument(ctx context.Context, in *IngestInput, text, title string, tn config.RuntimeTuning) (uuid.UUID, error) {
	if tn.MaxBodyBytes > 0 && int64(len(text)) > tn.MaxBodyBytes {
		return uuid.Nil, fmt.Errorf("source document exceeds max_body_bytes")
	}
	h := sha256.Sum256([]byte(text))
	if id, err := e.Store.GetSourceDocumentIDByHash(ctx, in.VaultID, in.UserID, h[:]); err == nil {
		return id, nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return uuid.Nil, err
	}
	id := uuid.New()
	d := &store.SourceDocumentInsert{
		ID:            id,
		VaultID:       in.VaultID,
		UserID:        in.UserID,
		ContentSHA256: h[:],
	}
	if title != "" {
		d.Title = &title
	}
	if e.Cfg.Encryption == config.EncryptionOn {
		dek := in.Session.DEK
		nonce, ct, err := e.Crypto.SealChunk(dek, []byte(text))
		if err != nil {
			return uuid.Nil, err
		}
		k := "v1"
		d.Ciphertext = ct
		d.Nonce = nonce
		d.KeyID = &k
	} else {
		t := text
		d.BodyText = &t
	}
	if err := e.Store.InsertSourceDocument(ctx, d); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func (e *Engine) ingestStructuredWork(ctx context.Context, in *IngestInput, work []ingestWorkItem, wantDebug bool) ([]uuid.UUID, *IngestDebugInfo, error) {
	tn := e.tuning()
	var inserted []uuid.UUID
	var dbg *IngestDebugInfo
	if wantDebug {
		dbg = &IngestDebugInfo{}
	}
	seen := map[[32]byte]struct{}{}
	for _, w := range work {
		if strings.TrimSpace(w.Abstract) == "" && strings.TrimSpace(w.Body) == "" {
			continue
		}
		h := contentDedupKey(w.Kind, w.Abstract, w.Body, w.SourceDocumentID)
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		exists, err := e.Store.ChunkExistsHash(ctx, in.VaultID, in.UserID, h[:])
		if err != nil {
			return inserted, dbg, err
		}
		if exists {
			continue
		}
		id, summary, err := e.persistStructuredItem(ctx, in, w, h[:], tn)
		if err != nil {
			return inserted, dbg, err
		}
		if wantDebug && summary != "" && dbg != nil {
			dbg.StructuredSummary = append(dbg.StructuredSummary, summary)
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

func (e *Engine) persistStructuredItem(ctx context.Context, in *IngestInput, w ingestWorkItem, h []byte, tn config.RuntimeTuning) (uuid.UUID, string, error) {
	bodyForJSON := w.Body
	if w.SourceDocumentID != nil {
		bodyForJSON = ""
	}
	var tsv string
	if w.SourceDocumentID != nil && w.SourceTextForTSV != "" {
		a := strings.TrimSpace(w.Abstract)
		ex := ExcerptRunes(w.SourceTextForTSV, 2000)
		if a == "" {
			tsv = ex
		} else {
			tsv = a + "\n\n" + ex
		}
	} else {
		cb0 := ChunkBody{Abstract: w.Abstract, Body: w.Body, Kind: w.Kind, Metadata: w.Metadata, SchemaVer: 2}
		tsv = cb0.LexicalSource()
	}
	embedSrc := strings.TrimSpace(w.Abstract)
	if embedSrc == "" {
		embedSrc = w.Body
		if w.SourceDocumentID != nil {
			embedSrc = ExcerptRunes(w.SourceTextForTSV, 2000)
		}
	}
	var plain string
	var schemaVer int
	if needsV2Format(w) {
		j, err := FormatChunkV2JSON(w.Abstract, bodyForJSON, w.Kind, w.Metadata)
		if err != nil {
			return uuid.Nil, "", err
		}
		plain = j
		schemaVer = 2
	} else {
		plain = bodyForJSON
		schemaVer = 1
	}
	if tn.MaxBodyBytes > 0 && int64(len(plain)) > tn.MaxBodyBytes {
		return uuid.Nil, "", fmt.Errorf("memory payload exceeds max_body_bytes")
	}
	vecs, err := e.OA.Embed(ctx, tn.EmbeddingModel, []string{embedSrc})
	if err != nil {
		return uuid.Nil, "", err
	}
	id := uuid.New()
	sk := strings.TrimSpace(in.SessionKey)
	if sk == "" {
		sk = "default"
	}
	ci := &store.ChunkInsert{
		ID:                 id,
		VaultID:            in.VaultID,
		UserID:             in.UserID,
		ContentSHA256:      h,
		EmbeddingModelID:   tn.EmbeddingModel,
		ChunkSchemaVersion: schemaVer,
		PlainForTSV:        tsv,
		Embedding:          vecs[0],
		IngestSessionKey:   &sk,
		ItemKind:           w.Kind,
		ItemMetadata:       w.Metadata,
		SourceDocumentID:   w.SourceDocumentID,
	}
	if e.Cfg.Encryption == config.EncryptionOn {
		dek := in.Session.DEK
		nonce, ct, err := e.Crypto.SealChunk(dek, []byte(plain))
		if err != nil {
			return uuid.Nil, "", err
		}
		k := "v1"
		ci.Ciphertext = ct
		ci.Nonce = nonce
		ci.KeyID = &k
	} else {
		bt := plain
		ci.BodyText = &bt
	}
	if err := e.Store.InsertChunk(ctx, ci); err != nil {
		return uuid.Nil, "", err
	}
	summary := fmt.Sprintf("kind=%q embed=%d tsv=%d", w.Kind, len(embedSrc), len(tsv))
	return id, summary, nil
}

func (e *Engine) persistV1TextChunk(ctx context.Context, in *IngestInput, f string, tn config.RuntimeTuning) (uuid.UUID, error) {
	f = strings.TrimSpace(f)
	if f == "" {
		return uuid.Nil, nil
	}
	if tn.MaxBodyBytes > 0 && int64(len(f)) > tn.MaxBodyBytes {
		return uuid.Nil, fmt.Errorf("memory payload exceeds max_body_bytes")
	}
	h := sha256.Sum256([]byte(strings.ToLower(f)))
	exists, err := e.Store.ChunkExistsHash(ctx, in.VaultID, in.UserID, h[:])
	if err != nil {
		return uuid.Nil, err
	}
	if exists {
		return uuid.Nil, nil
	}
	vecs, err := e.OA.Embed(ctx, tn.EmbeddingModel, []string{f})
	if err != nil {
		return uuid.Nil, err
	}
	id := uuid.New()
	sk := strings.TrimSpace(in.SessionKey)
	if sk == "" {
		sk = "default"
	}
	ci := &store.ChunkInsert{
		ID:                 id,
		VaultID:            in.VaultID,
		UserID:             in.UserID,
		ContentSHA256:      h[:],
		EmbeddingModelID:   tn.EmbeddingModel,
		ChunkSchemaVersion: 1,
		PlainForTSV:        f,
		Embedding:          vecs[0],
		IngestSessionKey:   &sk,
	}
	if e.Cfg.Encryption == config.EncryptionOn {
		dek := in.Session.DEK
		nonce, ct, err := e.Crypto.SealChunk(dek, []byte(f))
		if err != nil {
			return uuid.Nil, err
		}
		k := "v1"
		ci.Ciphertext = ct
		ci.Nonce = nonce
		ci.KeyID = &k
	} else {
		bt := f
		ci.BodyText = &bt
	}
	if err := e.Store.InsertChunk(ctx, ci); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}
