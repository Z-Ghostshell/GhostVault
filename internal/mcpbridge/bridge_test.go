package mcpbridge

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestPostRetrieve_authAndBody(t *testing.T) {
	t.Parallel()
	var gotAuth string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/retrieve" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		gotAuth = r.Header.Get("Authorization")
		var err error
		gotBody, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
	}))
	t.Cleanup(srv.Close)

	cfg := Config{
		BaseURL:     srv.URL,
		BearerToken: "test-token-xyz",
		HTTPClient:  srv.Client(),
	}
	raw, code, err := cfg.PostRetrieve(context.Background(), "550e8400-e29b-41d4-a716-446655440000", "u1", "hello", 3, 0)
	if err != nil {
		t.Fatal(err)
	}
	if code != http.StatusOK {
		t.Fatalf("code=%d body=%s", code, raw)
	}
	if gotAuth != "Bearer test-token-xyz" {
		t.Fatalf("Authorization: got %q", gotAuth)
	}
	var payload map[string]any
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["vault_id"] != "550e8400-e29b-41d4-a716-446655440000" || payload["user_id"] != "u1" || payload["query"] != "hello" {
		t.Fatalf("unexpected body: %s", gotBody)
	}
	if int(payload["max_chunks"].(float64)) != 3 {
		t.Fatalf("max_chunks: %v", payload["max_chunks"])
	}
}

func TestPostIngest_authAndBody(t *testing.T) {
	t.Parallel()
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/ingest" {
			http.NotFound(w, r)
			return
		}
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(b), `"vault_id"`) {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"chunk_ids": []string{}})
	}))
	t.Cleanup(srv.Close)

	cfg := Config{
		BaseURL:     srv.URL,
		BearerToken: "tok",
		HTTPClient:  srv.Client(),
	}
	raw, code, err := cfg.PostIngest(context.Background(), "550e8400-e29b-41d4-a716-446655440000", "u1", "memo text")
	if err != nil {
		t.Fatal(err)
	}
	if code != http.StatusOK {
		t.Fatalf("code=%d", code)
	}
	if gotAuth != "Bearer tok" {
		t.Fatalf("Authorization: got %q", gotAuth)
	}
	if !strings.Contains(string(raw), "chunk_ids") {
		t.Fatalf("response: %s", raw)
	}
}

func TestGetStats_auth(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotAuth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		if r.Method != http.MethodGet || r.URL.Path != "/v1/stats" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"vault_id": "test-vault", "chunks_total": 0})
	}))
	t.Cleanup(srv.Close)

	cfg := Config{
		BaseURL:     srv.URL,
		BearerToken: "test-token-xyz",
		HTTPClient:  srv.Client(),
	}
	raw, code, err := cfg.get(context.Background(), cfg.base()+"/v1/stats")
	if err != nil {
		t.Fatal(err)
	}
	if code != http.StatusOK {
		t.Fatalf("code=%d body=%s", code, raw)
	}
	if gotMethod != http.MethodGet || gotPath != "/v1/stats" {
		t.Fatalf("request: %s %s", gotMethod, gotPath)
	}
	if gotAuth != "Bearer test-token-xyz" {
		t.Fatalf("Authorization: got %q", gotAuth)
	}
	if !strings.Contains(string(raw), "chunks_total") {
		t.Fatalf("response: %s", raw)
	}
}

func TestApplyVaultUserDefaults_replacesPlaceholderDefault(t *testing.T) {
	t.Parallel()
	cfg := Config{
		DefaultVaultID: "f2be5dcc-5a79-4d9a-8e5a-69990076772f",
		DefaultUserID:  "svc",
	}
	v := "default"
	u := ""
	applyVaultUserDefaults(cfg, &v, &u)
	if v != cfg.DefaultVaultID {
		t.Fatalf("vault_id: got %q want %q", v, cfg.DefaultVaultID)
	}
	if u != "svc" {
		t.Fatalf("user_id: got %q want svc", u)
	}
}

func TestRegisterTools_requiresConfig(t *testing.T) {
	t.Parallel()
	s := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)
	err := RegisterTools(s, Config{})
	if err == nil {
		t.Fatal("expected error for empty config")
	}
}

func TestToolResultFromHTTP_error(t *testing.T) {
	t.Parallel()
	_, _, err := toolResultFromHTTP(401, []byte(`{"detail":"nope"}`))
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected 401 error, got %v", err)
	}
}
