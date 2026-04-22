// Command gvctl performs common Ghost Vault HTTP calls (health, init, unlock, lock, tools) without hand-written curl.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	cmd := os.Args[1]
	args := os.Args[2:]
	switch cmd {
	case "help", "-h", "--help":
		usage()
	case "health":
		fs := flag.NewFlagSet("health", flag.ExitOnError)
		base := fs.String("base-url", defaultBase(), "gvsvd origin (no trailing slash)")
		_ = fs.Parse(args)
		exitIfErr(runGET(*base, "/healthz", ""))
	case "ready":
		fs := flag.NewFlagSet("ready", flag.ExitOnError)
		base := fs.String("base-url", defaultBase(), "gvsvd origin (no trailing slash)")
		_ = fs.Parse(args)
		exitIfErr(runGET(*base, "/readyz", ""))
	case "init":
		fs := flag.NewFlagSet("init", flag.ExitOnError)
		base := fs.String("base-url", defaultBase(), "gvsvd origin (no trailing slash)")
		pass := fs.String("password", os.Getenv("GHOSTVAULT_PASSWORD"), "required when GV_ENCRYPTION=on; or set GHOSTVAULT_PASSWORD")
		authMode := fs.String("auth-mode", "session", "vault auth mode: session | auto_unlock | api_keys (api_keys reserved for future)")
		_ = fs.Parse(args)
		body := map[string]any{}
		if strings.TrimSpace(*pass) != "" {
			body["password"] = *pass
		}
		if strings.TrimSpace(*authMode) != "" {
			body["auth_mode"] = strings.TrimSpace(*authMode)
		}
		exitIfErr(runJSON(http.MethodPost, join(*base, "/v1/vault/init"), body, ""))
	case "unlock":
		fs := flag.NewFlagSet("unlock", flag.ExitOnError)
		base := fs.String("base-url", defaultBase(), "gvsvd origin (no trailing slash)")
		pass := fs.String("password", os.Getenv("GHOSTVAULT_PASSWORD"), "vault passphrase when encryption is on")
		tokenOnly := fs.Bool("token-only", false, "print only session_token (no JSON)")
		vaultIDOnly := fs.Bool("vault-id-only", false, "print only vault_id (no JSON)")
		writeTokenFile := fs.String("write-token-file", "", "write session_token to this file (0600); use with direnv + .ghostvault-bearer")
		_ = fs.Parse(args)
		raw, err := postJSON(join(*base, "/v1/vault/unlock"), map[string]string{"password": *pass}, "")
		if err != nil {
			exitIfErr(err)
			return
		}
		if *tokenOnly && *vaultIDOnly {
			log.Fatal("unlock: use only one of -token-only or -vault-id-only")
		}
		var unlockOut struct {
			SessionToken string `json:"session_token"`
			VaultID      string `json:"vault_id"`
		}
		if err := json.Unmarshal(raw, &unlockOut); err != nil {
			log.Fatal(err)
		}
		if p := strings.TrimSpace(*writeTokenFile); p != "" {
			if unlockOut.SessionToken == "" {
				log.Fatalf("no session_token in response: %s", raw)
			}
			abs, err := filepath.Abs(p)
			if err != nil {
				log.Fatal(err)
			}
			if err := os.WriteFile(abs, []byte(unlockOut.SessionToken), 0o600); err != nil {
				log.Fatal(err)
			}
			log.Printf("unlock: wrote session_token to %s", abs)
		}
		if *vaultIDOnly {
			if unlockOut.VaultID == "" {
				log.Fatalf("no vault_id in response: %s", raw)
			}
			fmt.Println(unlockOut.VaultID)
			return
		}
		if *tokenOnly {
			if unlockOut.SessionToken == "" {
				log.Fatalf("no session_token in response: %s", raw)
			}
			if strings.TrimSpace(*writeTokenFile) == "" {
				log.Printf("unlock: for direnv/MCP file, also pass -write-token-file .ghostvault-bearer (or run ./scripts/refresh-ghostvault-token.sh)")
			}
			fmt.Println(unlockOut.SessionToken)
			return
		}
		prettyPrintJSON(raw)
	case "lock":
		fs := flag.NewFlagSet("lock", flag.ExitOnError)
		base := fs.String("base-url", defaultBase(), "gvsvd origin (no trailing slash)")
		tok := fs.String("token", os.Getenv("GHOSTVAULT_BEARER_TOKEN"), "session Bearer token; or GHOSTVAULT_BEARER_TOKEN")
		_ = fs.Parse(args)
		if strings.TrimSpace(*tok) == "" {
			log.Fatal("lock: -token or GHOSTVAULT_BEARER_TOKEN required")
		}
		exitIfErr(runJSON(http.MethodPost, join(*base, "/v1/vault/lock"), nil, *tok))
	case "actions-token":
		fs := flag.NewFlagSet("actions-token", flag.ExitOnError)
		base := fs.String("base-url", defaultBase(), "gvsvd origin (no trailing slash)")
		tok := fs.String("token", os.Getenv("GHOSTVAULT_BEARER_TOKEN"), "session Bearer after unlock; or GHOSTVAULT_BEARER_TOKEN")
		ttl := fs.Int("ttl-seconds", 3600, "token TTL (plaintext vaults only)")
		_ = fs.Parse(args)
		if strings.TrimSpace(*tok) == "" {
			log.Fatal("actions-token: -token or GHOSTVAULT_BEARER_TOKEN required")
		}
		body := map[string]any{
			"scopes":      []string{"retrieve", "ingest"},
			"ttl_seconds": *ttl,
		}
		exitIfErr(runJSON(http.MethodPost, join(*base, "/v1/tokens/actions"), body, *tok))
	case "retrieve":
		fs := flag.NewFlagSet("retrieve", flag.ExitOnError)
		base := fs.String("base-url", defaultBase(), "gvsvd origin (no trailing slash)")
		tok := fs.String("token", os.Getenv("GHOSTVAULT_BEARER_TOKEN"), "Bearer session or actions token")
		vid := fs.String("vault-id", "", "vault UUID (required)")
		uid := fs.String("user-id", "", "logical user id (required)")
		query := fs.String("query", "", "search query (required)")
		maxC := fs.Int("max-chunks", 0, "max chunks (0 = server default)")
		maxT := fs.Int("max-tokens", 0, "max tokens (0 = server default)")
		_ = fs.Parse(args)
		if *vid == "" || *uid == "" || strings.TrimSpace(*query) == "" {
			log.Fatal("retrieve: -vault-id, -user-id, and -query are required")
		}
		if strings.TrimSpace(*tok) == "" {
			log.Fatal("retrieve: -token or GHOSTVAULT_BEARER_TOKEN required")
		}
		body := map[string]any{
			"vault_id": *vid,
			"user_id":  *uid,
			"query":    *query,
		}
		if *maxC > 0 {
			body["max_chunks"] = *maxC
		}
		if *maxT > 0 {
			body["max_tokens"] = *maxT
		}
		exitIfErr(runJSON(http.MethodPost, join(*base, "/v1/retrieve"), body, *tok))
	case "ingest":
		fs := flag.NewFlagSet("ingest", flag.ExitOnError)
		base := fs.String("base-url", defaultBase(), "gvsvd origin (no trailing slash)")
		tok := fs.String("token", os.Getenv("GHOSTVAULT_BEARER_TOKEN"), "Bearer session or actions token")
		vid := fs.String("vault-id", "", "vault UUID (required)")
		uid := fs.String("user-id", "", "logical user id (required)")
		text := fs.String("text", "", "text to store")
		infer := fs.Bool("infer", false, "LLM-assisted extraction")
		sessionKey := fs.String("session-key", "default", "session key for conversation scoping")
		_ = fs.Parse(args)
		if *vid == "" || *uid == "" {
			log.Fatal("ingest: -vault-id and -user-id are required")
		}
		if strings.TrimSpace(*tok) == "" {
			log.Fatal("ingest: -token or GHOSTVAULT_BEARER_TOKEN required")
		}
		body := map[string]any{
			"vault_id":    *vid,
			"user_id":     *uid,
			"session_key": *sessionKey,
			"text":        *text,
			"infer":       *infer,
		}
		exitIfErr(runJSON(http.MethodPost, join(*base, "/v1/ingest"), body, *tok))
	default:
		log.Fatalf("unknown command %q (try: gvctl help)", cmd)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `gvctl — Ghost Vault HTTP helper (replaces manual curl for common calls).

Usage:
  gvctl <command> [flags]

Environment:
  GHOSTVAULT_BASE_URL       Default base URL (default http://127.0.0.1:8989/api via edge → gvsvd; works without dashboard)
  GHOSTVAULT_PASSWORD       Passphrase for init/unlock when encryption is on
  GHOSTVAULT_BEARER_TOKEN   Session or actions token for authenticated routes

Commands:
  health              GET  /healthz
  ready               GET  /readyz
  init                POST /v1/vault/init  (-password or GHOSTVAULT_PASSWORD; omit for plaintext vault init; -auth-mode session|auto_unlock)
  unlock              POST /v1/vault/unlock  (-password; -token-only / -vault-id-only / -write-token-file)
  lock                POST /v1/vault/lock  (-token)
  actions-token       POST /v1/tokens/actions  (plaintext vaults; -token session)
  retrieve            POST /v1/retrieve  (-token -vault-id -user-id -query)
  ingest              POST /v1/ingest  (-token -vault-id -user-id [-text] [-infer])

Global flags (each command):
  -base-url string   Override GHOSTVAULT_BASE_URL for this invocation

Examples:
  gvctl health
  gvctl init -password 'your-passphrase'
  gvctl unlock
  gvctl unlock -token-only
  gvctl unlock -vault-id-only
  gvctl unlock -write-token-file .ghostvault-bearer
  export GHOSTVAULT_BEARER_TOKEN=$(gvctl unlock -token-only)
  gvctl retrieve -vault-id "$VID" -user-id u1 -query "notes about cats"
`)
}

func defaultBase() string {
	if s := strings.TrimSpace(os.Getenv("GHOSTVAULT_BASE_URL")); s != "" {
		return strings.TrimRight(s, "/")
	}
	return "http://127.0.0.1:8989/api"
}

func join(base, path string) string {
	return strings.TrimRight(base, "/") + path
}

var httpClient = &http.Client{Timeout: 125 * time.Second}

func runGET(base, path, bearer string) error {
	req, err := http.NewRequest(http.MethodGet, join(base, path), nil)
	if err != nil {
		return err
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(b)))
	}
	fmt.Println(strings.TrimSpace(string(b)))
	return nil
}

func runJSON(method, url string, body any, bearer string) error {
	raw, err := postJSON(url, body, bearer)
	if err != nil {
		return err
	}
	prettyPrintJSON(raw)
	return nil
}

func postJSON(url string, body any, bearer string) ([]byte, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(http.MethodPost, url, rdr)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(raw)))
	}
	return raw, nil
}

func prettyPrintJSON(raw []byte) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		fmt.Println(string(raw))
		return
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Println(string(raw))
		return
	}
	fmt.Println(string(out))
}

func truncate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 2000 {
		return s[:2000] + "…"
	}
	return s
}

func exitIfErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
