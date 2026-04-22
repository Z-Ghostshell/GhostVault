package mcpbridge

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewOAuthHTTP_NilWhenNoIntrospection(t *testing.T) {
	o, err := NewOAuthHTTP("https://issuer", "https://ex/mcp/1/", "https://ex/.well-known/oauth-protected-resource", "", "", "", nil, nil, UpstreamOverrides{})
	if err != nil {
		t.Fatal(err)
	}
	if o != nil {
		t.Fatalf("expected nil when introspection empty")
	}
}

func TestOAuthHTTP_VerifyToken_Introspection(t *testing.T) {
	exp := time.Now().Add(time.Hour).Unix()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method %s", r.Method)
		}
		_ = r.ParseForm()
		if r.Form.Get("token") != "tok" {
			t.Errorf("token %q", r.Form.Get("token"))
		}
		u, p, ok := r.BasicAuth()
		if !ok || u != "cid" || p != "csec" {
			t.Errorf("basic auth ok=%v u=%q p=%q", ok, u, p)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"active": true,
			"scope":  "a b",
			"exp":    exp,
			"sub":    "user-1",
		})
	}))
	defer ts.Close()

	o, err := NewOAuthHTTP("https://issuer", "https://ex/mcp/", ts.URL+"/introspect", ts.URL+"/introspect", "cid", "csec", []string{"a"}, nil, UpstreamOverrides{})
	if err != nil {
		t.Fatal(err)
	}
	info, err := o.VerifyToken(context.Background(), "tok", nil)
	if err != nil {
		t.Fatal(err)
	}
	if info.UserID != "user-1" {
		t.Errorf("UserID %q", info.UserID)
	}
	if len(info.Scopes) != 2 || info.Scopes[0] != "a" || info.Scopes[1] != "b" {
		t.Errorf("scopes %v", info.Scopes)
	}
}

func TestOAuthHTTP_VerifyToken_IntrospectionActiveNoExp_UsesJWTPayload(t *testing.T) {
	// Minimal JWT: header.payload.sig — payload {"exp": 2000000000} (year 2033)
	payload := `{"exp":2000000000}`
	b64 := base64.RawURLEncoding.EncodeToString([]byte(payload))
	tok := "eyJhbGciOiJIUzI1NiJ9." + b64 + ".sig"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"active": true,
			"scope":  "openid profile",
			"sub":    "auth0|123",
			// no exp — Auth0 sometimes omits for JWT access tokens
		})
	}))
	defer ts.Close()

	o, err := NewOAuthHTTP("https://issuer", "https://ex/mcp/", "https://ex/.well-known/oauth-protected-resource", ts.URL, "cid", "csec", nil, nil, UpstreamOverrides{})
	if err != nil {
		t.Fatal(err)
	}
	info, err := o.VerifyToken(context.Background(), tok, nil)
	if err != nil {
		t.Fatal(err)
	}
	if info.UserID != "auth0|123" {
		t.Errorf("UserID %q", info.UserID)
	}
	if info.Expiration.Unix() != 2000000000 {
		t.Errorf("exp %v", info.Expiration)
	}
}

func TestOAuthHTTP_VerifyToken_IntrospectionFloatExp(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate JSON number as float (common default)
		_, _ = w.Write([]byte(`{"active":true,"exp":2000000000.0,"sub":"u1"}`))
	}))
	defer ts.Close()

	o, err := NewOAuthHTTP("https://issuer", "https://ex/mcp/", "https://ex/.well-known/oauth-protected-resource", ts.URL, "", "", nil, nil, UpstreamOverrides{})
	if err != nil {
		t.Fatal(err)
	}
	info, err := o.VerifyToken(context.Background(), "opaque-token", nil)
	if err != nil {
		t.Fatal(err)
	}
	if info.Expiration.Unix() != 2000000000 {
		t.Errorf("exp unix %v", info.Expiration.Unix())
	}
}

func TestOAuthHTTP_VerifyToken_Inactive(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"active": false})
	}))
	defer ts.Close()

	o, err := NewOAuthHTTP("https://issuer", "https://ex/mcp/", "https://ex/.well-known/oauth-protected-resource", ts.URL, "", "", nil, nil, UpstreamOverrides{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = o.VerifyToken(context.Background(), "x", nil)
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("expected invalid token, got %v", err)
	}
}
