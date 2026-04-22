package core

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

const chunkFormatV2 = 2

// ExcerptRunes returns at most max runes of s (for search_tsv without duplicating a huge shared doc).
func ExcerptRunes(s string, max int) string {
	if max < 1 || s == "" {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

// ChunkBody is the logical content of a memory row after decrypt (v1 or v2).
type ChunkBody struct {
	Abstract   string
	Body       string
	Kind       string
	Metadata   map[string]any
	SchemaVer  int // 1 = legacy single body string; 2 = structured
	rawLegacy  string
}

// ParseChunkPlaintext decodes a stored chunk body. Legacy v1 is the entire string as Body.
func ParseChunkPlaintext(plain string) ChunkBody {
	s := strings.TrimSpace(plain)
	if s == "" {
		return ChunkBody{SchemaVer: 1}
	}
	if s[0] != '{' {
		return ChunkBody{Body: plain, SchemaVer: 1, rawLegacy: plain}
	}
	var w struct {
		V int    `json:"v"`
		A string `json:"a"`
		B string `json:"b"`
		K string `json:"k"`
		M map[string]any
	}
	if err := json.Unmarshal([]byte(plain), &w); err != nil {
		return ChunkBody{Body: plain, SchemaVer: 1, rawLegacy: plain}
	}
	if w.V != chunkFormatV2 {
		return ChunkBody{Body: plain, SchemaVer: 1, rawLegacy: plain}
	}
	meta := w.M
	if meta == nil {
		meta = nil
	}
	return ChunkBody{
		Abstract:  w.A,
		Body:      w.B,
		Kind:      w.K,
		Metadata:  meta,
		SchemaVer: 2,
	}
}

// FormatChunkV2JSON returns the JSON string stored in body_text or ciphertext (plaintext before seal).
func FormatChunkV2JSON(abstract, body, kind string, metadata map[string]any) (string, error) {
	if metadata == nil {
		metadata = nil
	}
	b, err := json.Marshal(struct {
		V int              `json:"v"`
		A string           `json:"a"`
		B string           `json:"b"`
		K string           `json:"k"`
		M map[string]any `json:"m,omitempty"`
	}{V: chunkFormatV2, A: abstract, B: body, K: kind, M: metadata})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// EmbedSource returns the text used for the dense embedding (abstract if non-empty, else body).
func (c ChunkBody) EmbedSource() string {
	if strings.TrimSpace(c.Abstract) != "" {
		return c.Abstract
	}
	return c.Body
}

// LexicalSource concatenates abstract and body for full-text index.
func (c ChunkBody) LexicalSource() string {
	a := strings.TrimSpace(c.Abstract)
	b := strings.TrimSpace(c.Body)
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + "\n\n" + b
}

// SnippetForContentMode returns text used for the primary `text` field, side text, and token estimate input.
// mode is "abstract" | "full" | "both" (for token packing, "both" counts abstract+full).
func (c ChunkBody) SnippetForContentMode(mode string) (primary, secondary string, forTokens string) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "abstract"
	}
	switch mode {
	case "full":
		return c.Body, "", c.Body
	case "both":
		return c.Abstract, c.Body, c.LexicalSource() // count both parts for token budget when returning both
	case "abstract", "default":
		fallthrough
	default:
		if strings.TrimSpace(c.Abstract) != "" {
			return c.Abstract, "", c.Abstract
		}
		return c.Body, "", c.Body
	}
}

// FullBodyAvailable is true when a separate long-form body exists (user can GET /chunks/:id for full v2 or same as text for v1).
func (c ChunkBody) FullBodyAvailable() bool {
	return strings.TrimSpace(c.Body) != ""
}

// SameAbstractAndBody is true when abstract is empty or equals body (short-circuit for flags).
func (c ChunkBody) SameAbstractAndBody() bool {
	a := strings.TrimSpace(c.Abstract)
	b := strings.TrimSpace(c.Body)
	if a == "" {
		return true
	}
	return a == b
}

// EstimatedReturnTokens approximates token cost for a retrieve response for this body under a resolved content mode
// (abstract, full, or both — not "auto").
func (c ChunkBody) EstimatedReturnTokens(resolvedMode string) int {
	m := strings.ToLower(strings.TrimSpace(resolvedMode))
	switch m {
	case "both":
		return estTok(c.Abstract) + estTok(c.Body)
	case "full":
		return estTok(c.Body)
	default: // abstract
		if strings.TrimSpace(c.Abstract) != "" {
			return estTok(c.Abstract)
		}
		return estTok(c.Body)
	}
}

func estTok(s string) int {
	if s == "" {
		return 0
	}
	r := utf8.RuneCountInString(s)
	n := r / 4
	if n < 1 {
		return 1
	}
	return n
}
