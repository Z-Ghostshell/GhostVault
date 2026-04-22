package mcpbridge

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// newFacadeFixture spins up a stub upstream AS and returns an OAuthHTTP configured against it.
func newFacadeFixture(t *testing.T) (*OAuthHTTP, *httptest.Server, *stubCalls) {
	t.Helper()
	calls := &stubCalls{}
	mux := http.NewServeMux()
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		calls.authorize = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		calls.tokenBody = string(body)
		calls.tokenAuth = r.Header.Get("Authorization")
		calls.tokenCT = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"abc","token_type":"Bearer","expires_in":3600}`))
	})
	mux.HandleFunc("/oidc/register", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		calls.registerBody = string(body)
		calls.registerCT = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"client_id":"cid-xyz","client_secret":"csec"}`))
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	o, err := NewOAuthHTTP(
		ts.URL+"/",
		"https://zf-mac.tail49b3bb.ts.net/mcp-deadbeef/",
		"https://zf-mac.tail49b3bb.ts.net/.well-known/oauth-protected-resource",
		ts.URL+"/oauth/introspect",
		"cid", "csec",
		nil, nil,
		UpstreamOverrides{
			AuthorizeURL: ts.URL + "/authorize",
			TokenURL:     ts.URL + "/oauth/token",
			RegisterURL:  ts.URL + "/oidc/register",
		},
	)
	if err != nil {
		t.Fatalf("NewOAuthHTTP: %v", err)
	}
	return o, ts, calls
}

type stubCalls struct {
	authorize    string
	tokenBody    string
	tokenAuth    string
	tokenCT      string
	registerBody string
	registerCT   string
}

func TestAuthorizationServerMetadataHandler_Shape(t *testing.T) {
	o, _, _ := newFacadeFixture(t)
	rr := httptest.NewRecorder()
	o.AuthorizationServerMetadataHandler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("content-type %q", ct)
	}
	var md authorizationServerMetadata
	if err := json.Unmarshal(rr.Body.Bytes(), &md); err != nil {
		t.Fatalf("decode: %v", err)
	}
	wantIssuer := "https://zf-mac.tail49b3bb.ts.net"
	if md.Issuer != wantIssuer {
		t.Errorf("issuer %q want %q", md.Issuer, wantIssuer)
	}
	if md.AuthorizationEndpoint != wantIssuer+"/authorize" {
		t.Errorf("authorize endpoint %q", md.AuthorizationEndpoint)
	}
	if md.TokenEndpoint != wantIssuer+"/token" {
		t.Errorf("token endpoint %q", md.TokenEndpoint)
	}
	if md.RegistrationEndpoint != wantIssuer+"/register" {
		t.Errorf("registration endpoint %q", md.RegistrationEndpoint)
	}
	hasS256 := false
	for _, m := range md.CodeChallengeMethodsSupported {
		if m == "S256" {
			hasS256 = true
		}
	}
	if !hasS256 {
		t.Errorf("PKCE S256 missing in %v", md.CodeChallengeMethodsSupported)
	}
}

func TestAuthorizeHandler_Redirects_WithAudience(t *testing.T) {
	o, ts, _ := newFacadeFixture(t)
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", "hXvGpjh6")
	q.Set("redirect_uri", "https://claude.ai/api/mcp/auth_callback")
	q.Set("code_challenge", "abc")
	q.Set("code_challenge_method", "S256")
	q.Set("state", "s")
	rr := httptest.NewRecorder()
	o.AuthorizeHandler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/authorize?"+q.Encode(), nil))
	if rr.Code != http.StatusFound {
		t.Fatalf("status %d, body %s", rr.Code, rr.Body.String())
	}
	loc := rr.Header().Get("Location")
	if !strings.HasPrefix(loc, ts.URL+"/authorize?") {
		t.Fatalf("Location %q does not target upstream", loc)
	}
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatal(err)
	}
	got := u.Query()
	if got.Get("client_id") != "hXvGpjh6" {
		t.Errorf("client_id dropped: %q", got.Get("client_id"))
	}
	if got.Get("code_challenge") != "abc" || got.Get("code_challenge_method") != "S256" {
		t.Errorf("PKCE params dropped: %v", got)
	}
	if got.Get("audience") != o.ResourceURL {
		t.Errorf("audience %q want %q", got.Get("audience"), o.ResourceURL)
	}
}

func TestAuthorizeHandler_OverridesWrongCallerAudience(t *testing.T) {
	o, _, _ := newFacadeFixture(t)
	q := url.Values{}
	q.Set("response_type", "code")
	// Claude sometimes sends a resource URL without trailing slash; Auth0 API identifiers are exact.
	q.Set("audience", "https://zf-mac.tail49b3bb.ts.net/mcp-deadbeef") // wrong — missing trailing /
	rr := httptest.NewRecorder()
	o.AuthorizeHandler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/authorize?"+q.Encode(), nil))
	loc := rr.Header().Get("Location")
	u, _ := url.Parse(loc)
	if got := u.Query().Get("audience"); got != o.ResourceURL {
		t.Errorf("audience %q want canonical %q", got, o.ResourceURL)
	}
}

func TestTokenHandler_Proxies_AndInjectsAudience(t *testing.T) {
	o, _, calls := newFacadeFixture(t)
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", "abc")
	form.Set("code_verifier", "verifier")
	form.Set("redirect_uri", "https://claude.ai/api/mcp/auth_callback")
	form.Set("client_id", "cid")
	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic Y2lkOmNzZWM=")
	rr := httptest.NewRecorder()
	o.TokenHandler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("relayed content-type %q", ct)
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["access_token"] != "abc" {
		t.Errorf("access_token %v", resp["access_token"])
	}
	if calls.tokenCT != "application/x-www-form-urlencoded" {
		t.Errorf("upstream content-type %q", calls.tokenCT)
	}
	if calls.tokenAuth != "Basic Y2lkOmNzZWM=" {
		t.Errorf("upstream authorization %q", calls.tokenAuth)
	}
	parsed, err := url.ParseQuery(calls.tokenBody)
	if err != nil {
		t.Fatalf("parse upstream body: %v", err)
	}
	if parsed.Get("audience") != o.ResourceURL {
		t.Errorf("audience not injected: %v", parsed)
	}
	if parsed.Get("code_verifier") != "verifier" {
		t.Errorf("PKCE verifier dropped: %v", parsed)
	}
}

func TestTokenHandler_OverridesWrongCallerAudience(t *testing.T) {
	o, _, calls := newFacadeFixture(t)
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("audience", "https://wrong/no-slash")
	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	o.TokenHandler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	parsed, _ := url.ParseQuery(calls.tokenBody)
	if parsed.Get("audience") != o.ResourceURL {
		t.Errorf("audience %q want canonical %q", parsed.Get("audience"), o.ResourceURL)
	}
}

func TestTokenHandler_RejectsWrongContentType(t *testing.T) {
	o, _, _ := newFacadeFixture(t)
	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(`{"grant_type":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	o.TokenHandler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnsupportedMediaType {
		t.Errorf("status %d", rr.Code)
	}
}

func TestRegisterHandler_ProxiesJSON(t *testing.T) {
	o, _, calls := newFacadeFixture(t)
	body := `{"client_name":"Claude","redirect_uris":["https://claude.ai/api/mcp/auth_callback"]}`
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	o.RegisterHandler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["client_id"] != "cid-xyz" {
		t.Errorf("client_id %v", resp["client_id"])
	}
	if calls.registerCT != "application/json" {
		t.Errorf("upstream content-type %q", calls.registerCT)
	}
	if !strings.Contains(calls.registerBody, "Claude") {
		t.Errorf("body not forwarded: %q", calls.registerBody)
	}
}

func TestRegisterHandler_RejectsWrongContentType(t *testing.T) {
	o, _, _ := newFacadeFixture(t)
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader("client_name=Claude"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	o.RegisterHandler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnsupportedMediaType {
		t.Errorf("status %d", rr.Code)
	}
}

func TestAuthorizeHandler_RejectsPOST(t *testing.T) {
	o, _, _ := newFacadeFixture(t)
	req := httptest.NewRequest(http.MethodPost, "/authorize", nil)
	rr := httptest.NewRecorder()
	o.AuthorizeHandler().ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status %d", rr.Code)
	}
}

func TestDefaults_DerivedFromIssuer(t *testing.T) {
	o, err := NewOAuthHTTP(
		"https://dev.example.com/",
		"https://mcp.example.com/mcp-tok/",
		"https://mcp.example.com/.well-known/oauth-protected-resource",
		"https://dev.example.com/oauth/introspect",
		"", "", nil, nil,
		UpstreamOverrides{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if o.UpstreamAuthorizeURL != "https://dev.example.com/authorize" {
		t.Errorf("authorize default %q", o.UpstreamAuthorizeURL)
	}
	if o.UpstreamTokenURL != "https://dev.example.com/oauth/token" {
		t.Errorf("token default %q", o.UpstreamTokenURL)
	}
	if o.UpstreamRegisterURL != "https://dev.example.com/oidc/register" {
		t.Errorf("register default %q", o.UpstreamRegisterURL)
	}
	if o.AudienceURL != "https://mcp.example.com/mcp-tok/" {
		t.Errorf("audience default %q", o.AudienceURL)
	}
}
