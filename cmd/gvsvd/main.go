package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/z-ghostshell/ghostvault/db"
	"github.com/z-ghostshell/ghostvault/internal/api"
	"github.com/z-ghostshell/ghostvault/internal/auth"
	"github.com/z-ghostshell/ghostvault/internal/config"
	"github.com/z-ghostshell/ghostvault/internal/core"
	"github.com/z-ghostshell/ghostvault/internal/crypto"
	"github.com/z-ghostshell/ghostvault/internal/providers"
	"github.com/z-ghostshell/ghostvault/internal/store"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	pool, err := store.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("db pool: %w", err)
	}
	defer pool.Close()

	if err := migrate(ctx, cfg.DatabaseURL); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	if err := assertEncryptionMode(ctx, pool, cfg); err != nil {
		return err
	}

	vc := crypto.NewVaultCrypto()
	vaultState := &auth.VaultState{}

	if err := initAuthMode(ctx, pool, vc, vaultState); err != nil {
		return err
	}

	sessMgr := auth.NewManagerWithStore(cfg.SessionIdle, cfg.SessionMax, pool, vaultState)
	oa := providers.NewOpenAI(cfg.OpenAIBaseURL, cfg.OpenAIAPIKey)
	oa.EmbeddingsURL = cfg.OpenAIEmbeddingsURL
	tState, err := config.NewTuningState(cfg.TuningFile, func(ctx context.Context) ([]byte, error) {
		return pool.GetServerTuning(ctx)
	})
	if err != nil {
		return fmt.Errorf("tuning: %w", err)
	}
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	go func() {
		for range hup {
			if err := tState.Reload(context.Background()); err != nil {
				log.Printf("gvsvd: tuning reload failed: %v", err)
			} else {
				log.Printf("gvsvd: tuning reloaded from %s", cfg.TuningFile)
			}
		}
	}()

	eng := &core.Engine{Cfg: cfg, Tuning: tState, Store: pool, Sess: sessMgr, Crypto: vc, OA: oa}
	srv := &api.Server{
		Cfg:    cfg,
		Tuning: tState,
		Store:  pool,
		Sess:   sessMgr,
		Crypto: vc,
		OA:     oa,
		Eng:    eng,
	}

	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("http listen %s: %w", cfg.HTTPAddr, err)
	}
	log.Printf("gvsvd listening on %s encryption=%s", ln.Addr().String(), cfg.Encryption)
	if cfg.DebugAuthLog {
		log.Printf("gvsvd: GV_DEBUG_AUTH=true — logging auth rejections with request path/remote/token fingerprint (set GV_DEBUG_AUTH=false to silence)")
	}
	if cfg.DebugAuthFull {
		log.Printf("gvsvd: WARNING GV_DEBUG_AUTH_FULL=true — full bearer tokens may appear in logs; use only in isolated troubleshooting")
	}

	httpSrv := &http.Server{
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      125 * time.Second, // > chi request timeout (120s) for slow retrieve/ingest
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("server: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	sessMgr.InvalidateAll()
	shCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpSrv.Shutdown(shCtx)
}

func migrate(ctx context.Context, dsn string) error {
	pcfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return err
	}
	sqlDB := stdlib.OpenDB(*pcfg)
	defer sqlDB.Close()
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	goose.SetBaseFS(db.Migrations)
	if err := goose.UpContext(ctx, sqlDB, "migrations"); err != nil {
		return err
	}
	return nil
}

func assertEncryptionMode(ctx context.Context, pool *store.Pool, cfg *config.Config) error {
	v, err := pool.GetAnyVault(ctx)
	if err != nil {
		return err
	}
	if v == nil {
		return nil
	}
	want := cfg.Encryption == config.EncryptionOn
	if v.EncryptionEnabled != want {
		return fmt.Errorf("GV_ENCRYPTION does not match existing vault (db=%v env=%v): refusing to start",
			v.EncryptionEnabled, want)
	}
	return nil
}

// initAuthMode loads the existing vault row (if any) and applies mode-specific boot
// behavior to VaultState and the persisted sessions table.
//
//   - session      → purge all sessions rows; VaultState.DEK stays empty until /v1/vault/unlock.
//   - auto_unlock  → read server-side password, unwrap DEK into VaultState, keep sessions.
//     Fail-closed: wrong password or missing material returns an error so
//     supervisor-managed gvsvd refuses to serve.
//   - api_keys     → reserved for Part 3; currently rejected at init so we shouldn't see it.
func initAuthMode(ctx context.Context, pool *store.Pool, vc *crypto.VaultCrypto, state *auth.VaultState) error {
	v, err := pool.GetAnyVault(ctx)
	if err != nil {
		return err
	}
	if v == nil {
		log.Printf("auth_mode=<uninitialized> (no vault row yet)")
		return nil
	}
	state.ID = v.ID
	mode := auth.AuthMode(v.AuthMode)

	switch mode {
	case auth.AuthModeSession, "":
		if err := pool.PurgeSessionsForVault(ctx, v.ID); err != nil {
			return fmt.Errorf("purge sessions on boot: %w", err)
		}
		log.Printf("auth_mode=session vault=%s (sessions purged on boot)", v.ID)
		return nil
	case auth.AuthModeAutoUnlock:
		if !v.EncryptionEnabled {
			log.Printf("auth_mode=auto_unlock vault=%s encryption=off (no DEK to unwrap)", v.ID)
			return nil
		}
		pw, err := readAutoUnlockPassword()
		if err != nil {
			return fmt.Errorf("auto_unlock: %w", err)
		}
		if v.ArgonTime == nil || v.ArgonMemoryKB == nil || v.ArgonParallelism == nil {
			return fmt.Errorf("auto_unlock: vault row missing KDF parameters")
		}
		wd := &crypto.WrappedDEK{
			Salt:        v.ArgonSalt,
			TimeCost:    uint32(*v.ArgonTime),
			MemoryKB:    uint32(*v.ArgonMemoryKB),
			Parallelism: uint8(*v.ArgonParallelism),
			Blob:        v.WrappedDEK,
		}
		dek, err := vc.UnwrapDEK(pw, wd)
		if err != nil {
			return fmt.Errorf("auto_unlock: invalid password (DEK unwrap failed)")
		}
		state.DEK = dek
		log.Printf("auth_mode=auto_unlock vault=%s (DEK unwrapped, sessions persist across restart)", v.ID)
		return nil
	case auth.AuthModeAPIKeys:
		return fmt.Errorf("auth_mode=api_keys is not yet implemented on this build; re-init the vault with a supported mode")
	default:
		return fmt.Errorf("unknown auth_mode %q on vault %s", v.AuthMode, v.ID)
	}
}

// readAutoUnlockPassword prefers GHOSTVAULT_PASSWORD_FILE (file mount, restrictive perms
// recommended) and falls back to GHOSTVAULT_PASSWORD for dev convenience.
func readAutoUnlockPassword() (string, error) {
	if p := strings.TrimSpace(os.Getenv("GHOSTVAULT_PASSWORD_FILE")); p != "" {
		b, err := os.ReadFile(p)
		if err != nil {
			return "", fmt.Errorf("read GHOSTVAULT_PASSWORD_FILE=%s: %w", p, err)
		}
		pw := strings.TrimRight(string(b), "\r\n")
		if pw == "" {
			return "", fmt.Errorf("GHOSTVAULT_PASSWORD_FILE=%s is empty", p)
		}
		return pw, nil
	}
	if pw := os.Getenv("GHOSTVAULT_PASSWORD"); pw != "" {
		log.Printf("auto_unlock: using GHOSTVAULT_PASSWORD env var (prefer GHOSTVAULT_PASSWORD_FILE for production)")
		return pw, nil
	}
	return "", fmt.Errorf("auto_unlock requires GHOSTVAULT_PASSWORD_FILE (preferred) or GHOSTVAULT_PASSWORD")
}
