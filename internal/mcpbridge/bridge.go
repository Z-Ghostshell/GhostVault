// Package mcpbridge registers Ghost Vault REST operations as MCP tools (memory_search, memory_save, memory_stats).
package mcpbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/z-ghostshell/ghostvault/internal/authdebug"
)

const (
	defaultName    = "ghostvault-mcp"
	defaultVersion = "0.1.0"
)

// Config holds HTTP client settings to reach gvsvd.
type Config struct {
	// BaseURL is the gvsvd origin without a trailing slash, e.g. http://127.0.0.1:8989/api (Docker edge) or http://ghostvault:8080 inside Compose.
	BaseURL string
	// BearerToken is the session token or (plaintext-vault) actions token (initial value from env/file at startup).
	BearerToken string
	// TokenReloadPath is set when BearerToken came from a file, not GHOSTVAULT_BEARER_TOKEN / -bearer.
	// When non-empty, post() re-reads this file before each gvsvd request.
	TokenReloadPath string
	// DefaultVaultID and DefaultUserID fill tool args when the model omits them (from GHOSTVAULT_DEFAULT_VAULT_ID / GHOSTVAULT_DEFAULT_USER_ID).
	DefaultVaultID string
	DefaultUserID  string
	// HTTPClient is used for POST/GET to gvsvd. If nil, a client with a 125s timeout is used.
	HTTPClient *http.Client
}

func (c *Config) client() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 125 * time.Second}
}

func (c *Config) base() string {
	return strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
}

// bearerForGV returns the vault session token for gvsvd, re-reading the file when TokenReloadPath is set.
func (c *Config) bearerForGV() string {
	tok := strings.TrimSpace(c.BearerToken)
	if p := strings.TrimSpace(c.TokenReloadPath); p != "" {
		if b, err := os.ReadFile(p); err == nil {
			if t := strings.TrimSpace(string(b)); t != "" {
				return t
			}
		}
	}
	return tok
}

func applyVaultUserDefaults(cfg Config, vaultID, userID *string) {
	// Models often send vault_id "default" as a placeholder; vault_id must be a UUID from unlock.
	v := strings.TrimSpace(*vaultID)
	if v == "" || strings.EqualFold(v, "default") {
		if def := strings.TrimSpace(cfg.DefaultVaultID); def != "" {
			*vaultID = def
		}
	}
	if strings.TrimSpace(*userID) == "" {
		if def := strings.TrimSpace(cfg.DefaultUserID); def != "" {
			*userID = def
		}
	}
}

// RegisterTools adds memory_search, memory_save, and memory_stats to the MCP server.
func RegisterTools(s *mcp.Server, cfg Config) error {
	base := cfg.base()
	if base == "" || strings.TrimSpace(cfg.BearerToken) == "" {
		return fmt.Errorf("mcpbridge: Config.BaseURL and Config.BearerToken are required")
	}

	const searchDesc = `Search the user's stored memories (hybrid: dense embeddings + full-text + fusion). Use when the user asks about something that may have been saved before, or to ground an answer in prior facts.

Write query with salient keywords and a short paraphrase of the information need (not a raw paste of the whole chat). Cap breadth with max_chunks and max_tokens if results are too large.

Optional session_key + session_mode: "all" (default), "only" (search only memories ingested with that session_key), "prefer" (boost memories from that session after hybrid scoring). session_mode "only" requires session_key. Response includes results and meta (result_count; hint when empty).

Maps to POST /v1/retrieve.`

	mcp.AddTool(s, &mcp.Tool{
		Name:        "memory_search",
		Description: searchDesc,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in memorySearchIn) (*mcp.CallToolResult, any, error) {
		applyVaultUserDefaults(cfg, &in.VaultID, &in.UserID)
		body, err := json.Marshal(in)
		if err != nil {
			return nil, nil, err
		}
		raw, code, err := cfg.post(ctx, base+"/v1/retrieve", body)
		if err != nil {
			return nil, nil, err
		}
		return toolResultFromHTTP(code, raw)
	})

	const saveDesc = `Persist new memories: raw text (chunked) or structured items: abstract, body, kind, optional metadata, or items[] for multiple. infer runs LLM extraction; infer_target abstract|body where to place each extracted string. Chunks are tagged with session_key for optional session_scoped search.

Maps to POST /v1/ingest.`

	mcp.AddTool(s, &mcp.Tool{
		Name:        "memory_save",
		Description: saveDesc,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in memorySaveIn) (*mcp.CallToolResult, any, error) {
		applyVaultUserDefaults(cfg, &in.VaultID, &in.UserID)
		body, err := json.Marshal(in)
		if err != nil {
			return nil, nil, err
		}
		raw, code, err := cfg.post(ctx, base+"/v1/ingest", body)
		if err != nil {
			return nil, nil, err
		}
		return toolResultFromHTTP(code, raw)
	})

	const statsDesc = `Read-only vault summary: total chunks, per-user counts, recent ingest events, session_message counts. The vault is implied by the Bearer token on gvmcp (no vault_id in the body). Use to check whether the vault has data or to debug empty search results. Maps to GET /v1/stats.`

	mcp.AddTool(s, &mcp.Tool{
		Name:        "memory_stats",
		Description: statsDesc,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in memoryStatsIn) (*mcp.CallToolResult, any, error) {
		_ = in
		raw, code, err := cfg.get(ctx, base+"/v1/stats")
		if err != nil {
			return nil, nil, err
		}
		return toolResultFromHTTP(code, raw)
	})
	return nil
}

// NewMCPServer builds an MCP server with Ghost Vault tools registered.
func NewMCPServer(cfg Config) (*mcp.Server, error) {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    defaultName,
		Version: defaultVersion,
	}, &mcp.ServerOptions{
		Instructions: `Ghost Vault: device-local memory for the LLM host via REST.

vault_id: UUID from POST /v1/vault/unlock. If the model sends "default" or omits it, gvmcp fills -default-vault-id / GHOSTVAULT_DEFAULT_VAULT_ID. user_id: logical scope (profiles, threads); use -default-user-id / GHOSTVAULT_DEFAULT_USER_ID when omitted.

Call memory_search when answers should use stored facts. Call memory_save when the user asked to remember or you must persist. Call memory_stats to see chunk counts and recent activity, especially if memory_search returns no useful hits (see meta.hint in retrieve responses). Prefer a few targeted searches per turn; avoid thrashing.

Bearer for gvmcp → gvsvd invalidates when gvsvd restarts: unlock and refresh GHOSTVAULT_BEARER_TOKEN or GHOSTVAULT_TOKEN_FILE (gvctl unlock -write-token-file). This is separate from any OAuth the MCP client uses to reach gvmcp.

Full agent guidance: docs/integration/skills/ghostvault/SKILL.md in the Ghost Vault repo.`,
	})
	if err := RegisterTools(s, cfg); err != nil {
		return nil, err
	}
	return s, nil
}

func (c *Config) post(ctx context.Context, url string, body []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	tok := c.bearerForGV()
	req.Header.Set("Authorization", "Bearer "+tok)
	if envTruthy("GHOSTVAULT_DEBUG_AUTH") {
		full := envTruthy("GHOSTVAULT_DEBUG_AUTH_FULL")
		log.Printf("gvmcp http debug POST %s bearer_fp=%s bearer=%s", url, authdebug.Fingerprint(tok), authdebug.ForLog(tok, full))
		if full && tok != "" {
			log.Printf("gvmcp http debug WARNING GHOSTVAULT_DEBUG_AUTH_FULL=true: full bearer logged above")
		}
	}
	resp, err := c.client().Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized && envTruthy("GHOSTVAULT_DEBUG_AUTH") {
		log.Printf("gvmcp: gvsvd returned 401 for POST %s (Ghost Vault session bearer on gvmcp rejected by gvsvd — check GHOSTVAULT_TOKEN_FILE / .ghostvault-bearer and make rotate-token after gvsvd restart; Claude OAuth is separate from this header)", url)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return raw, resp.StatusCode, nil
}

func (c *Config) get(ctx context.Context, url string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	tok := c.bearerForGV()
	req.Header.Set("Authorization", "Bearer "+tok)
	if envTruthy("GHOSTVAULT_DEBUG_AUTH") {
		full := envTruthy("GHOSTVAULT_DEBUG_AUTH_FULL")
		log.Printf("gvmcp http debug GET %s bearer_fp=%s bearer=%s", url, authdebug.Fingerprint(tok), authdebug.ForLog(tok, full))
		if full && tok != "" {
			log.Printf("gvmcp http debug WARNING GHOSTVAULT_DEBUG_AUTH_FULL=true: full bearer logged above")
		}
	}
	resp, err := c.client().Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized && envTruthy("GHOSTVAULT_DEBUG_AUTH") {
		log.Printf("gvmcp: gvsvd returned 401 for GET %s (check GHOSTVAULT_TOKEN_FILE / rotate-token after gvsvd restart)", url)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return raw, resp.StatusCode, nil
}

func toolResultFromHTTP(code int, raw []byte) (*mcp.CallToolResult, any, error) {
	if code >= 200 && code < 300 {
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, nil, fmt.Errorf("ghostvault: invalid JSON response: %w", err)
		}
		return nil, v, nil
	}
	msg := strings.TrimSpace(string(raw))
	if len(msg) > 4000 {
		msg = msg[:4000] + "…"
	}
	return nil, nil, fmt.Errorf("ghostvault HTTP %d: %s", code, msg)
}

type memorySearchIn struct {
	VaultID     string `json:"vault_id,omitempty" jsonschema:"optional when gvmcp has -default-vault-id / GHOSTVAULT_DEFAULT_VAULT_ID; vault UUID from unlock — not the word default"`
	UserID      string `json:"user_id,omitempty" jsonschema:"optional when gvmcp has -default-user-id / GHOSTVAULT_DEFAULT_USER_ID; logical scope inside the vault"`
	Query       string `json:"query" jsonschema:"search query: keywords and short paraphrase"`
	MaxChunks   int    `json:"max_chunks,omitempty" jsonschema:"max result chunks"`
	MaxTokens   int    `json:"max_tokens,omitempty" jsonschema:"approximate token budget for snippets"`
	SessionKey  string `json:"session_key,omitempty" jsonschema:"filter or boost: must match memory_save session_key for only/prefer"`
	SessionMode string `json:"session_mode,omitempty" jsonschema:"all (default), only, or prefer — only requires session_key"`
	ContentMode string `json:"content_mode,omitempty" jsonschema:"auto (default), abstract, full, or both — which fields in each result"`
}

// memoryStatsIn is a placeholder for the stats tool; GET /v1/stats uses the Bearer (vault) only.
type memoryStatsIn struct{}

type ingestItemIn struct {
	Abstract string         `json:"abstract,omitempty"`
	Text     string         `json:"text,omitempty"`
	Body     string         `json:"body,omitempty"`
	Kind     string         `json:"kind,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type sourceDocumentIn struct {
	Text  string `json:"text,omitempty" jsonschema:"full document stored once; items with no body link here"`
	Title string `json:"title,omitempty" jsonschema:"optional label"`
}

type memorySaveIn struct {
	VaultID         string            `json:"vault_id,omitempty" jsonschema:"optional when gvmcp has -default-vault-id / GHOSTVAULT_DEFAULT_VAULT_ID; vault UUID from unlock — not the word default"`
	UserID          string            `json:"user_id,omitempty" jsonschema:"optional when gvmcp has -default-user-id / GHOSTVAULT_DEFAULT_USER_ID; logical scope inside the vault"`
	SessionKey      string            `json:"session_key,omitempty" jsonschema:"conversation session key"`
	Text            string            `json:"text,omitempty" jsonschema:"raw text to chunk and store (legacy) or full body with abstract"`
	Abstract        string            `json:"abstract,omitempty" jsonschema:"short abstract for hybrid index; use with text/body for long content"`
	Kind            string            `json:"kind,omitempty" jsonschema:"e.g. note, preference, article_segment"`
	Metadata        map[string]any    `json:"metadata,omitempty" jsonschema:"optional host-defined JSON object"`
	Items           []ingestItemIn    `json:"items,omitempty" jsonschema:"multiple structured memories in one call"`
	SourceDocument  *sourceDocumentIn `json:"source_document,omitempty" jsonschema:"one shared full document for items with empty body"`
	Infer           bool              `json:"infer,omitempty" jsonschema:"use LLM to extract facts"`
	InferTarget     string            `json:"infer_target,omitempty" jsonschema:"abstract (default) or body — where extracted strings are stored"`
	Messages        []ingestMessage   `json:"messages,omitempty" jsonschema:"chat turns to append before ingest"`
	IdempotencyKey  string            `json:"idempotency_key,omitempty" jsonschema:"dedupe key for this ingest"`
}

type ingestMessage struct {
	Role    string `json:"role" jsonschema:"e.g. user, assistant"`
	Content string `json:"content" jsonschema:"turn text"`
}

// PostRetrieve calls POST /v1/retrieve with the same JSON body as memory_search (for tests).
func (c *Config) PostRetrieve(ctx context.Context, vaultID, userID, query string, maxChunks, maxTokens int) ([]byte, int, error) {
	base := c.base()
	if base == "" {
		return nil, 0, fmt.Errorf("mcpbridge: empty BaseURL")
	}
	in := memorySearchIn{VaultID: vaultID, UserID: userID, Query: query, MaxChunks: maxChunks, MaxTokens: maxTokens}
	body, err := json.Marshal(in)
	if err != nil {
		return nil, 0, err
	}
	return c.post(ctx, base+"/v1/retrieve", body)
}

// PostIngest calls POST /v1/ingest with the same JSON body shape as memory_save (for tests).
func (c *Config) PostIngest(ctx context.Context, vaultID, userID, text string) ([]byte, int, error) {
	base := c.base()
	if base == "" {
		return nil, 0, fmt.Errorf("mcpbridge: empty BaseURL")
	}
	in := memorySaveIn{VaultID: vaultID, UserID: userID, Text: text}
	body, err := json.Marshal(in)
	if err != nil {
		return nil, 0, err
	}
	return c.post(ctx, base+"/v1/ingest", body)
}

func envTruthy(k string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(k)))
	switch v {
	case "1", "t", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}
