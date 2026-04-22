package mcpbridge

import (
	"fmt"
	"os"
	"strings"
)

// BearerFromEnvAndFile resolves the session token. Precedence: non-empty bearerStr (from -bearer /
// GHOSTVAULT_BEARER_TOKEN) always wins; the token file is used only when that value is empty. Then:
// tokenFileFlag, else GHOSTVAULT_TOKEN_FILE, else .ghostvault-bearer if that path exists.
func BearerFromEnvAndFile(bearerStr, tokenFileFlag string) (string, error) {
	tok := strings.TrimSpace(bearerStr)
	if tok != "" {
		return tok, nil
	}
	path := strings.TrimSpace(tokenFileFlag)
	if path == "" {
		path = strings.TrimSpace(os.Getenv("GHOSTVAULT_TOKEN_FILE"))
	}
	if path == "" {
		const def = ".ghostvault-bearer"
		if st, err := os.Stat(def); err == nil && !st.IsDir() {
			path = def
		}
	}
	if path == "" {
		return "", fmt.Errorf("set GHOSTVAULT_BEARER_TOKEN or -bearer, or run gvctl unlock -write-token-file .ghostvault-bearer (or set GHOSTVAULT_TOKEN_FILE)")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read bearer token file %q: %w", path, err)
	}
	tok = strings.TrimSpace(string(b))
	if tok == "" {
		return "", fmt.Errorf("bearer token file %q is empty", path)
	}
	return tok, nil
}

// TokenReloadPathIfFileBased returns the token file path when the bearer is loaded from disk
// (no non-empty bearerStr from env/flag). When non-empty, callers may re-read this path before
// each gvsvd request so rotate-token / unlock updates apply without restarting gvmcp.
func TokenReloadPathIfFileBased(bearerStr, tokenFileFlag string) string {
	if strings.TrimSpace(bearerStr) != "" {
		return ""
	}
	path := strings.TrimSpace(tokenFileFlag)
	if path == "" {
		path = strings.TrimSpace(os.Getenv("GHOSTVAULT_TOKEN_FILE"))
	}
	if path == "" {
		const def = ".ghostvault-bearer"
		if st, err := os.Stat(def); err == nil && !st.IsDir() {
			path = def
		}
	}
	return path
}
