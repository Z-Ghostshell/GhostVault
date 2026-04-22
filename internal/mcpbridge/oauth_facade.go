// OAuth 2.1 facade served on the MCP host itself to work around claude.ai ignoring
// RFC 9728 authorization_servers and instead constructing /authorize, /token and /register
// on the MCP host (see anthropics/claude-ai-mcp#82). These handlers 302-redirect (authorize)
// or reverse-proxy (token, register) to the upstream IdP (Auth0 by default), injecting the
// "audience" parameter so Auth0 issues a JWT bound to GHOSTVAULT_OAUTH_RESOURCE.
package mcpbridge

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// facadeMaxBody bounds bodies we copy between Claude and the upstream AS.
const facadeMaxBody = 1 << 20

// hopByHopHeaders are per-RFC 7230 §6.1 never forwarded by an intermediary.
var hopByHopHeaders = map[string]struct{}{
	"Connection":          {},
	"Keep-Alive":          {},
	"Proxy-Authenticate":  {},
	"Proxy-Authorization": {},
	"Te":                  {},
	"Trailers":            {},
	"Transfer-Encoding":   {},
	"Upgrade":             {},
}

// authorizationServerMetadata mirrors the subset of RFC 8414 that claude.ai reads.
type authorizationServerMetadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	RegistrationEndpoint              string   `json:"registration_endpoint,omitempty"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	ResponseModesSupported            []string `json:"response_modes_supported,omitempty"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	ScopesSupported                   []string `json:"scopes_supported,omitempty"`
}

// issuerFromResource returns the origin of the ResourceURL (scheme://host), used as the facade issuer.
func (o *OAuthHTTP) issuerFromResource() string {
	u, err := url.Parse(strings.TrimSpace(o.ResourceURL))
	if err != nil || u == nil || u.Scheme == "" || u.Host == "" {
		return strings.TrimRight(o.ResourceURL, "/")
	}
	return u.Scheme + "://" + u.Host
}

// AuthorizationServerMetadataHandler serves RFC 8414 metadata pointing back at the facade endpoints.
func (o *OAuthHTTP) AuthorizationServerMetadataHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		issuer := o.issuerFromResource()
		scopes := o.ScopesSupported
		if len(scopes) == 0 {
			scopes = o.RequiredScopes
		}
		md := authorizationServerMetadata{
			Issuer:                        issuer,
			AuthorizationEndpoint:         issuer + "/authorize",
			TokenEndpoint:                 issuer + "/token",
			RegistrationEndpoint:          issuer + "/register",
			ResponseTypesSupported:        []string{"code"},
			ResponseModesSupported:        []string{"query"},
			GrantTypesSupported:           []string{"authorization_code", "refresh_token"},
			CodeChallengeMethodsSupported: []string{"S256"},
			TokenEndpointAuthMethodsSupported: []string{
				"client_secret_basic",
				"client_secret_post",
				"none",
			},
			ScopesSupported: scopes,
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(&md)
	})
}

// AuthorizeHandler 302-redirects to the upstream authorization endpoint, forwarding the query
// and setting `audience` to the configured canonical value when GHOSTVAULT_OAUTH_AUDIENCE is set.
// We always override any client-supplied audience: hosts like Auth0 match API identifiers as exact
// strings; Claude sometimes sends an audience derived from the resource URL without a trailing slash
// while the Auth0 API Identifier uses a trailing slash — forwarding the wrong value causes
// "Service not found" even when the API exists.
func (o *OAuthHTTP) AuthorizeHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if strings.TrimSpace(o.UpstreamAuthorizeURL) == "" {
			http.Error(w, "oauth facade: upstream authorize URL not configured", http.StatusInternalServerError)
			return
		}
		u, err := url.Parse(o.UpstreamAuthorizeURL)
		if err != nil {
			http.Error(w, "oauth facade: invalid upstream authorize URL", http.StatusInternalServerError)
			return
		}
		q := u.Query()
		// Do not forward client "audience" — merge then Set can still leave duplicate keys in some
		// edge cases; Claude may send audience without a trailing slash while Auth0's API Identifier
		// uses one. Only the configured canonical GHOSTVAULT_OAUTH_AUDIENCE is sent upstream.
		for k, vs := range r.URL.Query() {
			if strings.EqualFold(k, "audience") {
				continue
			}
			for _, v := range vs {
				q.Add(k, v)
			}
		}
		if aud := strings.TrimSpace(o.AudienceURL); aud != "" {
			q.Set("audience", aud)
		}
		u.RawQuery = q.Encode()
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, u.String(), http.StatusFound)
	})
}

// TokenHandler proxies POST /token to the upstream token endpoint. Request body must be
// application/x-www-form-urlencoded (RFC 6749 §4.1.3). Sets `audience` to the configured value
// when set (same override rationale as AuthorizeHandler).
func (o *OAuthHTTP) TokenHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if strings.TrimSpace(o.UpstreamTokenURL) == "" {
			http.Error(w, "oauth facade: upstream token URL not configured", http.StatusInternalServerError)
			return
		}
		ct := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]))
		if ct != "application/x-www-form-urlencoded" {
			http.Error(w, "oauth facade: token endpoint requires application/x-www-form-urlencoded", http.StatusUnsupportedMediaType)
			return
		}
		raw, err := io.ReadAll(io.LimitReader(r.Body, facadeMaxBody))
		if err != nil {
			http.Error(w, "oauth facade: read body: "+err.Error(), http.StatusBadRequest)
			return
		}
		form, err := url.ParseQuery(string(raw))
		if err != nil {
			http.Error(w, "oauth facade: invalid form body", http.StatusBadRequest)
			return
		}
		if aud := strings.TrimSpace(o.AudienceURL); aud != "" {
			form.Del("audience")
			form.Set("audience", aud)
		}
		body := form.Encode()
		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, o.UpstreamTokenURL, strings.NewReader(body))
		if err != nil {
			http.Error(w, "oauth facade: build upstream request: "+err.Error(), http.StatusInternalServerError)
			return
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		if v := r.Header.Get("Authorization"); v != "" {
			req.Header.Set("Authorization", v)
		}
		o.relayUpstream(w, req)
	})
}

// RegisterHandler proxies POST /register to the upstream DCR endpoint (Auth0 /oidc/register).
// Request body must be application/json (RFC 7591 §3).
func (o *OAuthHTTP) RegisterHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if strings.TrimSpace(o.UpstreamRegisterURL) == "" {
			http.Error(w, "oauth facade: upstream registration URL not configured", http.StatusInternalServerError)
			return
		}
		ct := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]))
		if ct != "application/json" {
			http.Error(w, "oauth facade: registration endpoint requires application/json", http.StatusUnsupportedMediaType)
			return
		}
		raw, err := io.ReadAll(io.LimitReader(r.Body, facadeMaxBody))
		if err != nil {
			http.Error(w, "oauth facade: read body: "+err.Error(), http.StatusBadRequest)
			return
		}
		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, o.UpstreamRegisterURL, strings.NewReader(string(raw)))
		if err != nil {
			http.Error(w, "oauth facade: build upstream request: "+err.Error(), http.StatusInternalServerError)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		if v := r.Header.Get("Authorization"); v != "" {
			req.Header.Set("Authorization", v)
		}
		o.relayUpstream(w, req)
	})
}

// relayUpstream executes req against the upstream AS and relays status + headers + body back to the caller.
func (o *OAuthHTTP) relayUpstream(w http.ResponseWriter, req *http.Request) {
	resp, err := o.httpClient().Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("oauth facade: upstream call failed: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	for k, vs := range resp.Header {
		if _, hop := hopByHopHeaders[http.CanonicalHeaderKey(k)]; hop {
			continue
		}
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(resp.Body, facadeMaxBody))
}
