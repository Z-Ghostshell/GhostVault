package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteProblem_redacts5xxDetail(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx := context.WithValue(context.Background(), requestIDKey, "rid-1")
	req = req.WithContext(ctx)

	WriteProblem(rec, req, http.StatusInternalServerError, "Ingest failed", "pq: secret database error", "ingest")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("code %d", rec.Code)
	}
	var p Problem
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(p.Detail, "pq:") || strings.Contains(p.Detail, "database") {
		t.Fatalf("client body leaked internal detail: %q", p.Detail)
	}
	if p.Detail != redactedServerDetail {
		t.Fatalf("detail %q want redacted constant", p.Detail)
	}
}

func TestWriteProblem_preserves4xxDetail(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx := context.WithValue(context.Background(), requestIDKey, "rid-2")
	req = req.WithContext(ctx)

	WriteProblem(rec, req, http.StatusBadRequest, "Bad Request", "vault_id required", "validate")

	var p Problem
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatal(err)
	}
	if p.Detail != "vault_id required" {
		t.Fatalf("detail %q", p.Detail)
	}
}
