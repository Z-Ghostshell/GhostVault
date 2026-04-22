package mcpbridge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBearerFromEnvAndFile_prefersEnv(t *testing.T) {
	t.Parallel()
	got, err := BearerFromEnvAndFile("from-env", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "from-env" {
		t.Fatalf("got %q want from-env", got)
	}
}

func TestBearerFromEnvAndFile_bearerOverridesFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	tokFile := filepath.Join(dir, ".ghostvault-bearer")
	if err := os.WriteFile(tokFile, []byte("from-file-only"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := BearerFromEnvAndFile("from-bearer-wins", tokFile)
	if err != nil {
		t.Fatal(err)
	}
	if got != "from-bearer-wins" {
		t.Fatalf("got %q want bearer to override file", got)
	}
}

func TestBearerFromEnvAndFile_readsDefaultFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	tokFile := filepath.Join(dir, ".ghostvault-bearer")
	want := "abc123token"
	if err := os.WriteFile(tokFile, []byte(want+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := BearerFromEnvAndFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestBearerFromEnvAndFile_explicitPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "custom.tok")
	want := "explicit"
	if err := os.WriteFile(p, []byte(want), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := BearerFromEnvAndFile("", p)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestBearerFromEnvAndFile_missing(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	_, err := BearerFromEnvAndFile("", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTokenReloadPathIfFileBased(t *testing.T) {
	if p := TokenReloadPathIfFileBased("env-wins", "/tmp/x"); p != "" {
		t.Fatalf("expected empty when bearer set, got %q", p)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".ghostvault-bearer"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	got := TokenReloadPathIfFileBased("", "")
	if got == "" {
		t.Fatal("expected default .ghostvault-bearer path")
	}
	if filepath.Base(got) != ".ghostvault-bearer" {
		t.Fatalf("got %q", got)
	}
}

func TestBearerForGV_reloadsFromFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bearer.tok")
	if err := os.WriteFile(p, []byte("first\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := Config{BearerToken: "first", TokenReloadPath: p}
	if got := cfg.bearerForGV(); got != "first" {
		t.Fatalf("got %q", got)
	}
	if err := os.WriteFile(p, []byte("second\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := cfg.bearerForGV(); got != "second" {
		t.Fatalf("want reload from file, got %q", got)
	}
}
