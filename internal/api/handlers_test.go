package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/z-ghostshell/ghostvault/internal/auth"
	"github.com/z-ghostshell/ghostvault/internal/config"
)

func TestVaultStats_requiresAuth(t *testing.T) {
	cfg := &config.Config{
		HTTPAddr:    ":0",
		TuningFile:  "",
		Encryption:  config.EncryptionOff,
		SessionIdle: time.Hour,
		SessionMax:  2 * time.Hour,
	}
	tsT, err := config.NewTuningState("", func(ctx context.Context) ([]byte, error) { return []byte("{}"), nil })
	if err != nil {
		t.Fatal(err)
	}
	srv := &Server{
		Cfg:    cfg,
		Tuning: tsT,
		Sess:   auth.NewManager(cfg.SessionIdle, cfg.SessionMax),
		Store:  nil,
	}
	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/v1/stats")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)

	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d body=%s", res.StatusCode, body)
	}
}
