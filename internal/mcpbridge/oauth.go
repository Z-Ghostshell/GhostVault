// OAuth helpers for streamable HTTP MCP: RFC 9728 protected-resource metadata and
// RFC 7662 token introspection (see github.com/modelcontextprotocol/go-sdk/examples/auth/server).
package mcpbridge

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

// OAuthHTTP bundles OAuth 2.0 resource-server settings for gvmcp streamable HTTP mode.
type OAuthHTTP struct {
	AuthServerIssuer string
	ResourceURL      string
	MetadataURL      string
	// IntrospectionURL is the RFC 7662 token introspection endpoint (POST, form body token=…).
	IntrospectionURL          string
	IntrospectionClientID     string
	IntrospectionClientSecret string
	// RequiredScopes, if non-empty, must all appear on the token (introspection "scope" split on spaces).
	RequiredScopes []string
	// ScopesSupported is advertised in protected-resource metadata (RFC 9728). If nil, defaults to RequiredScopes.
	ScopesSupported []string
	// Upstream Auth0 (or any RFC 8414 AS) endpoints the facade on the MCP host delegates to.
	// Claude.ai currently ignores authorization_servers from RFC 9728 metadata and instead
	// builds /authorize, /token and /register on the MCP host itself (anthropics/claude-ai-mcp#82),
	// so gvmcp exposes a thin facade that 302s / proxies to these upstream URLs.
	UpstreamAuthorizeURL string
	UpstreamTokenURL     string
	UpstreamRegisterURL  string
	// AudienceURL is injected into /authorize and /token as the OAuth 2.0 "audience" parameter
	// (Auth0-specific extension) when the caller did not supply one, so the issued access token
	// is bound to the MCP resource identifier. Defaults to ResourceURL.
	AudienceURL string
	HTTPClient  *http.Client
}

// ProtectedResourceMetadata returns RFC 9728 metadata for /.well-known/oauth-protected-resource.
func (o *OAuthHTTP) ProtectedResourceMetadata() *oauthex.ProtectedResourceMetadata {
	sup := o.ScopesSupported
	if len(sup) == 0 && len(o.RequiredScopes) > 0 {
		sup = append([]string(nil), o.RequiredScopes...)
	}
	return &oauthex.ProtectedResourceMetadata{
		Resource:               o.ResourceURL,
		AuthorizationServers:   []string{o.AuthServerIssuer},
		ScopesSupported:        sup,
		BearerMethodsSupported: []string{"header"},
		ResourceName:           "Ghost Vault MCP",
	}
}

func (o *OAuthHTTP) httpClient() *http.Client {
	if o.HTTPClient != nil {
		return o.HTTPClient
	}
	return &http.Client{Timeout: 15 * time.Second}
}

// VerifyToken implements [auth.TokenVerifier] via RFC 7662 introspection.
func (o *OAuthHTTP) VerifyToken(ctx context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
	if strings.TrimSpace(o.IntrospectionURL) == "" {
		return nil, fmt.Errorf("%w", auth.ErrInvalidToken)
	}
	form := url.Values{}
	form.Set("token", token)
	form.Set("token_type_hint", "access_token")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.IntrospectionURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if o.IntrospectionClientID != "" || o.IntrospectionClientSecret != "" {
		req.SetBasicAuth(o.IntrospectionClientID, o.IntrospectionClientSecret)
	}

	resp, err := o.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: introspection status %d", auth.ErrInvalidToken, resp.StatusCode)
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var tr map[string]interface{}
	if err := dec.Decode(&tr); err != nil {
		return nil, err
	}
	active, _ := tr["active"].(bool)
	if !active {
		return nil, fmt.Errorf("%w", auth.ErrInvalidToken)
	}

	exp, ok := oauthExpFromIntrospectionMap(tr)
	if !ok {
		// Auth0 often issues JWT access tokens for custom APIs; introspection may omit `exp` or
		// return it in a shape that does not map cleanly. When missing, read `exp` from the JWT payload.
		exp, ok = expFromJWTUnverified(token)
		if !ok {
			return nil, fmt.Errorf("%w: introspection missing exp and JWT payload has no exp", auth.ErrInvalidToken)
		}
	}

	scopeStr, _ := tr["scope"].(string)
	var scopes []string
	if strings.TrimSpace(scopeStr) != "" {
		scopes = strings.Fields(scopeStr)
	}
	sub, _ := tr["sub"].(string)
	return &auth.TokenInfo{
		Scopes:     scopes,
		Expiration: exp,
		UserID:     sub,
	}, nil
}

// oauthExpFromIntrospectionMap parses `exp` from RFC 7662 JSON (Auth0 may use int or float).
func oauthExpFromIntrospectionMap(tr map[string]interface{}) (time.Time, bool) {
	v, ok := tr["exp"]
	if !ok || v == nil {
		return time.Time{}, false
	}
	switch x := v.(type) {
	case json.Number:
		// Int64() fails for JSON floats like 2000000000.0 — use Float64().
		f, err := x.Float64()
		if err != nil {
			return time.Time{}, false
		}
		return time.Unix(int64(f), 0), true
	case float64:
		return time.Unix(int64(x), 0), true
	case int64:
		return time.Unix(x, 0), true
	}
	return time.Time{}, false
}

// expFromJWTUnverified reads the registered `exp` claim from an access token JWT without
// verifying the signature (Auth0 already accepted the token at introspection when active=true).
func expFromJWTUnverified(token string) (time.Time, bool) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.URLEncoding.DecodeString(parts[1])
	}
	if err != nil {
		return time.Time{}, false
	}
	var claims struct {
		Exp float64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return time.Time{}, false
	}
	if claims.Exp == 0 {
		return time.Time{}, false
	}
	return time.Unix(int64(claims.Exp), 0), true
}

// UpstreamOverrides carries optional explicit upstream endpoint URLs for [NewOAuthHTTP].
// Any field left empty is derived from issuer ({issuer}authorize, {issuer}oauth/token, {issuer}oidc/register).
// Audience defaults to resourceURL when empty.
type UpstreamOverrides struct {
	AuthorizeURL string
	TokenURL     string
	RegisterURL  string
	Audience     string
}

// NewOAuthHTTP builds OAuth settings for gvmcp. If introspectionURL is empty, returns nil, nil.
func NewOAuthHTTP(issuer, resourceURL, metadataURL, introspectionURL, introClientID, introSecret string, requiredScopes, scopesSupported []string, up UpstreamOverrides) (*OAuthHTTP, error) {
	introspectionURL = strings.TrimSpace(introspectionURL)
	if introspectionURL == "" {
		return nil, nil
	}
	issuer = strings.TrimSpace(issuer)
	resourceURL = strings.TrimSpace(resourceURL)
	metadataURL = strings.TrimSpace(metadataURL)
	if issuer == "" || resourceURL == "" || metadataURL == "" {
		return nil, fmt.Errorf("OAuth enabled (introspection URL set): require issuer, resource URL, and metadata URL")
	}
	authorize := strings.TrimSpace(up.AuthorizeURL)
	token := strings.TrimSpace(up.TokenURL)
	register := strings.TrimSpace(up.RegisterURL)
	audience := strings.TrimSpace(up.Audience)
	issuerSlash := strings.TrimRight(issuer, "/") + "/"
	if authorize == "" {
		authorize = issuerSlash + "authorize"
	}
	if token == "" {
		token = issuerSlash + "oauth/token"
	}
	if register == "" {
		register = issuerSlash + "oidc/register"
	}
	if audience == "" {
		audience = resourceURL
	}
	return &OAuthHTTP{
		AuthServerIssuer:          issuer,
		ResourceURL:               resourceURL,
		MetadataURL:               metadataURL,
		IntrospectionURL:          introspectionURL,
		IntrospectionClientID:     strings.TrimSpace(introClientID),
		IntrospectionClientSecret: strings.TrimSpace(introSecret),
		RequiredScopes:            requiredScopes,
		ScopesSupported:           scopesSupported,
		UpstreamAuthorizeURL:      authorize,
		UpstreamTokenURL:          token,
		UpstreamRegisterURL:       register,
		AudienceURL:               audience,
	}, nil
}

// ParseOAuthHTTPFromEnv builds [OAuthHTTP] when GHOSTVAULT_OAUTH_INTROSPECTION_URL is set; otherwise nil.
func ParseOAuthHTTPFromEnv() (*OAuthHTTP, error) {
	get := func(k string) string { return strings.TrimSpace(os.Getenv(k)) }
	var req, sup []string
	if s := get("GHOSTVAULT_OAUTH_SCOPES"); s != "" {
		for _, p := range strings.Split(s, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				req = append(req, p)
			}
		}
	}
	if s := get("GHOSTVAULT_OAUTH_SCOPES_SUPPORTED"); s != "" {
		for _, p := range strings.Split(s, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				sup = append(sup, p)
			}
		}
	}
	return NewOAuthHTTP(
		get("GHOSTVAULT_OAUTH_AUTH_SERVER_ISSUER"),
		get("GHOSTVAULT_OAUTH_RESOURCE"),
		get("GHOSTVAULT_OAUTH_METADATA_URL"),
		get("GHOSTVAULT_OAUTH_INTROSPECTION_URL"),
		get("GHOSTVAULT_OAUTH_INTROSPECTION_CLIENT_ID"),
		get("GHOSTVAULT_OAUTH_INTROSPECTION_CLIENT_SECRET"),
		req,
		sup,
		UpstreamOverrides{
			AuthorizeURL: get("GHOSTVAULT_OAUTH_UPSTREAM_AUTHORIZE_URL"),
			TokenURL:     get("GHOSTVAULT_OAUTH_TOKEN_URL"),
			RegisterURL:  get("GHOSTVAULT_OAUTH_REGISTRATION_URL"),
			Audience:     get("GHOSTVAULT_OAUTH_AUDIENCE"),
		},
	)
}
