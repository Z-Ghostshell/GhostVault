//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/z-ghostshell/ghostvault/db"
	"github.com/z-ghostshell/ghostvault/internal/api"
	"github.com/z-ghostshell/ghostvault/internal/auth"
	"github.com/z-ghostshell/ghostvault/internal/config"
	"github.com/z-ghostshell/ghostvault/internal/core"
	"github.com/z-ghostshell/ghostvault/internal/crypto"
	"github.com/z-ghostshell/ghostvault/internal/providers"
	"github.com/z-ghostshell/ghostvault/internal/store"
)

func TestHealthAfterMigrate(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	c, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("pgvector/pgvector:pg16"),
		postgres.WithDatabase("gv"),
		postgres.WithUsername("gv"),
		postgres.WithPassword("gv"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp")),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Terminate(ctx) })

	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	pcfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB := stdlib.OpenDB(*pcfg)
	defer sqlDB.Close()
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	goose.SetBaseFS(db.Migrations)
	if err := goose.Up(sqlDB, "migrations"); err != nil {
		t.Fatal(err)
	}

	pool, err := store.NewPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	cfg := &config.Config{
		HTTPAddr:     ":0",
		TuningFile:   "",
		Encryption:   config.EncryptionOff,
		SessionIdle:  time.Hour,
		SessionMax:   2 * time.Hour,
	}
	tState, err := config.NewTuningState("", func(ctx context.Context) ([]byte, error) {
		return pool.GetServerTuning(ctx)
	})
	if err != nil {
		t.Fatal(err)
	}
	sess := auth.NewManager(cfg.SessionIdle, cfg.SessionMax)
	oa := providers.NewOpenAI("https://api.openai.com/v1", "")
	vc := crypto.NewVaultCrypto()
	srv := &api.Server{
		Cfg:    cfg,
		Tuning: tState,
		Store:  pool,
		Sess:   sess,
		Crypto: vc,
		OA:     oa,
		Eng:    &core.Engine{Cfg: cfg, Tuning: tState, Store: pool, Sess: sess, Crypto: vc, OA: oa},
	}
	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}

	// GET /v1/stats without Bearer → 401
	statsNoAuth, err := http.Get(ts.URL + "/v1/stats")
	if err != nil {
		t.Fatal(err)
	}
	defer statsNoAuth.Body.Close()
	if statsNoAuth.StatusCode != http.StatusUnauthorized {
		t.Fatalf("stats without auth: status %d want 401", statsNoAuth.StatusCode)
	}

	initRes, err := http.Post(ts.URL+"/v1/vault/init", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer initRes.Body.Close()
	if initRes.StatusCode != http.StatusCreated {
		t.Fatalf("init: status %d", initRes.StatusCode)
	}

	unlockRes, err := http.Post(ts.URL+"/v1/vault/unlock", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer unlockRes.Body.Close()
	if unlockRes.StatusCode != http.StatusOK {
		t.Fatalf("unlock: status %d", unlockRes.StatusCode)
	}
	var unlockBody struct {
		SessionToken string `json:"session_token"`
	}
	if err := json.NewDecoder(unlockRes.Body).Decode(&unlockBody); err != nil {
		t.Fatal(err)
	}
	if unlockBody.SessionToken == "" {
		t.Fatal("empty session_token")
	}

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/v1/stats", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+unlockBody.SessionToken)
	statsOK, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer statsOK.Body.Close()
	if statsOK.StatusCode != http.StatusOK {
		t.Fatalf("stats with auth: status %d", statsOK.StatusCode)
	}
}
