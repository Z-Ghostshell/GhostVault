// Command gvmcp is the Ghost Vault MCP server: exposes memory_search and memory_save over MCP (stdio or HTTP).
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/z-ghostshell/ghostvault/internal/mcpbridge"
)

func main() {
	log.SetFlags(0)
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	listen := flag.String("listen", "", "if set, serve streamable MCP HTTP on this address (e.g. :3751); otherwise stdio")
	baseURL := flag.String("base-url", os.Getenv("GHOSTVAULT_BASE_URL"), "gvsvd origin (no trailing slash); default from GHOSTVAULT_BASE_URL")
	bearer := flag.String("bearer", os.Getenv("GHOSTVAULT_BEARER_TOKEN"), "Bearer token; default from GHOSTVAULT_BEARER_TOKEN (takes precedence over -token-file / GHOSTVAULT_TOKEN_FILE when set)")
	tokenFile := flag.String("token-file", "", "token file when -bearer and GHOSTVAULT_BEARER_TOKEN are empty; else GHOSTVAULT_TOKEN_FILE, else .ghostvault-bearer if present (e.g. gvctl unlock -write-token-file .ghostvault-bearer)")
	defaultVaultID := flag.String("default-vault-id", strings.TrimSpace(os.Getenv("GHOSTVAULT_DEFAULT_VAULT_ID")), "vault UUID substituted when a tool sends empty or literal \"default\" for vault_id; env GHOSTVAULT_DEFAULT_VAULT_ID (prefer passing this flag in MCP config if env is not applied)")
	defaultUserID := flag.String("default-user-id", strings.TrimSpace(os.Getenv("GHOSTVAULT_DEFAULT_USER_ID")), "user_id when a tool omits it; env GHOSTVAULT_DEFAULT_USER_ID")
	oauthIssuer := flag.String("oauth-auth-server-issuer", os.Getenv("GHOSTVAULT_OAUTH_AUTH_SERVER_ISSUER"), "OAuth AS issuer URL (RFC 8414); env GHOSTVAULT_OAUTH_AUTH_SERVER_ISSUER")
	oauthResource := flag.String("oauth-resource", os.Getenv("GHOSTVAULT_OAUTH_RESOURCE"), "public MCP resource URL for RFC 9728 metadata; env GHOSTVAULT_OAUTH_RESOURCE")
	oauthMetadataURL := flag.String("oauth-metadata-url", os.Getenv("GHOSTVAULT_OAUTH_METADATA_URL"), "full URL of /.well-known/oauth-protected-resource; env GHOSTVAULT_OAUTH_METADATA_URL")
	oauthIntro := flag.String("oauth-introspection-url", os.Getenv("GHOSTVAULT_OAUTH_INTROSPECTION_URL"), "RFC 7662 token introspection URL; env GHOSTVAULT_OAUTH_INTROSPECTION_URL (enables OAuth on MCP HTTP)")
	oauthIntroCID := flag.String("oauth-introspection-client-id", os.Getenv("GHOSTVAULT_OAUTH_INTROSPECTION_CLIENT_ID"), "optional client_id for introspection basic auth; env GHOSTVAULT_OAUTH_INTROSPECTION_CLIENT_ID")
	oauthIntroCS := flag.String("oauth-introspection-client-secret", os.Getenv("GHOSTVAULT_OAUTH_INTROSPECTION_CLIENT_SECRET"), "optional client_secret for introspection; env GHOSTVAULT_OAUTH_INTROSPECTION_CLIENT_SECRET")
	oauthScopes := flag.String("oauth-scopes", os.Getenv("GHOSTVAULT_OAUTH_SCOPES"), "comma-separated required token scopes; env GHOSTVAULT_OAUTH_SCOPES")
	oauthScopesSup := flag.String("oauth-scopes-supported", os.Getenv("GHOSTVAULT_OAUTH_SCOPES_SUPPORTED"), "comma scopes advertised in metadata; env GHOSTVAULT_OAUTH_SCOPES_SUPPORTED")
	oauthUpstreamAuthorize := flag.String("oauth-upstream-authorize-url", os.Getenv("GHOSTVAULT_OAUTH_UPSTREAM_AUTHORIZE_URL"), "upstream AS authorization endpoint; default {issuer}authorize; env GHOSTVAULT_OAUTH_UPSTREAM_AUTHORIZE_URL")
	oauthTokenURL := flag.String("oauth-token-url", os.Getenv("GHOSTVAULT_OAUTH_TOKEN_URL"), "upstream AS token endpoint; default {issuer}oauth/token; env GHOSTVAULT_OAUTH_TOKEN_URL")
	oauthRegURL := flag.String("oauth-registration-url", os.Getenv("GHOSTVAULT_OAUTH_REGISTRATION_URL"), "upstream AS DCR endpoint; default {issuer}oidc/register; env GHOSTVAULT_OAUTH_REGISTRATION_URL")
	oauthAudience := flag.String("oauth-audience", os.Getenv("GHOSTVAULT_OAUTH_AUDIENCE"), "OAuth 2.0 audience parameter for /authorize and /token; default = -oauth-resource; env GHOSTVAULT_OAUTH_AUDIENCE")
	flag.Parse()

	base := strings.TrimSpace(*baseURL)
	defVault := strings.TrimSpace(*defaultVaultID)
	defUser := strings.TrimSpace(*defaultUserID)
	if base == "" {
		return fmt.Errorf("set -base-url or GHOSTVAULT_BASE_URL to your gvsvd URL (e.g. http://127.0.0.1:8989/api with Docker edge)")
	}
	tok, err := mcpbridge.BearerFromEnvAndFile(*bearer, *tokenFile)
	if err != nil {
		return err
	}
	reloadPath := mcpbridge.TokenReloadPathIfFileBased(*bearer, *tokenFile)

	cfg := mcpbridge.Config{
		BaseURL:         base,
		BearerToken:     tok,
		TokenReloadPath: reloadPath,
		DefaultVaultID:  defVault,
		DefaultUserID:   defUser,
	}

	srv, err := mcpbridge.NewMCPServer(cfg)
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if strings.TrimSpace(*listen) != "" {
		oauth, err := mcpbridge.NewOAuthHTTP(
			*oauthIssuer,
			*oauthResource,
			*oauthMetadataURL,
			*oauthIntro,
			*oauthIntroCID,
			*oauthIntroCS,
			splitCommaScopes(*oauthScopes),
			splitCommaScopes(*oauthScopesSup),
			mcpbridge.UpstreamOverrides{
				AuthorizeURL: *oauthUpstreamAuthorize,
				TokenURL:     *oauthTokenURL,
				RegisterURL:  *oauthRegURL,
				Audience:     *oauthAudience,
			},
		)
		if err != nil {
			return err
		}
		return runHTTP(ctx, srv, strings.TrimSpace(*listen), oauth)
	}
	return srv.Run(ctx, &mcp.StdioTransport{})
}

func splitCommaScopes(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func runHTTP(ctx context.Context, server *mcp.Server, addr string, oauth *mcpbridge.OAuthHTTP) error {
	ver := version()
	mcpHandler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{
		JSONResponse: true,
		Stateless:    true,
	})
	var root http.Handler = mcpHandler
	if oauth != nil {
		mux := http.NewServeMux()
		// RFC 9728 protected-resource metadata (what the MCP resource is) and
		// RFC 8414 authorization-server metadata facade (claude.ai ignores the former's
		// authorization_servers field and reads the latter from the MCP host directly;
		// see anthropics/claude-ai-mcp#82).
		mux.Handle("/.well-known/oauth-protected-resource", auth.ProtectedResourceMetadataHandler(oauth.ProtectedResourceMetadata()))
		mux.Handle("/.well-known/oauth-authorization-server", oauth.AuthorizationServerMetadataHandler())
		mux.Handle("/authorize", oauth.AuthorizeHandler())
		mux.Handle("/token", oauth.TokenHandler())
		mux.Handle("/register", oauth.RegisterHandler())
		guarded := auth.RequireBearerToken(oauth.VerifyToken, &auth.RequireBearerTokenOptions{
			ResourceMetadataURL: oauth.MetadataURL,
			Scopes:              oauth.RequiredScopes,
		})(mcpHandler)
		mux.Handle("/", guarded)
		root = mux
		log.Printf("gvmcp OAuth: protected resource metadata + RFC 7662 introspection + AS facade (metadata URL %s, upstream authorize %s, audience %s)",
			oauth.MetadataURL, oauth.UpstreamAuthorizeURL, oauth.AudienceURL)
	}
	s := &http.Server{
		Addr:    addr,
		Handler: root,
	}
	errCh := make(chan error, 1)
	go func() {
		if oauth != nil {
			log.Printf("gvmcp streamable MCP + OAuth on http://%s (version %s)", addr, ver)
		} else {
			log.Printf("gvmcp streamable MCP listening on http://%s (version %s)", addr, ver)
		}
		errCh <- s.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		_ = s.Shutdown(context.Background())
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func version() string {
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "devel"
}
