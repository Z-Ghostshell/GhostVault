package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type EncryptionMode string

const (
	EncryptionOff EncryptionMode = "off"
	EncryptionOn  EncryptionMode = "on"
)

// Config is bootstrap: env, paths, and secrets. Algorithm tuning is in [RuntimeTuning] and [TuningFile].
type Config struct {
	HTTPAddr string
	// TuningFile is the path to YAML (defaults + file merge). Env: GV_Tuning_FILE, default configs/gvsvd.yaml
	TuningFile string
	// DebugAuthLog logs bearer/session diagnostics on 401/403 from vault routes when true (GV_DEBUG_AUTH).
	DebugAuthLog bool
	// DebugAuthFull includes the full bearer token in those logs when true (GV_DEBUG_AUTH_FULL). Dangerous in production.
	DebugAuthFull bool
	DatabaseURL   string
	Encryption    EncryptionMode
	OpenAIAPIKey  string
	OpenAIBaseURL string
	// OpenAIEmbeddingsURL optional full POST URL for embeddings. If empty, uses OpenAIBaseURL + "/embeddings" (same as OPENAI_BASE_URL).
	OpenAIEmbeddingsURL string
	SessionIdle         time.Duration
	SessionMax          time.Duration
	EmbeddingDimension  int
}

// Load reads bootstrap [Config] from the environment. Tuning values are loaded at runtime from [TuningFile] and DB; see [TuningState].
func Load() (*Config, error) {
	encRaw := strings.ToLower(strings.TrimSpace(getEnv("GV_ENCRYPTION", "on")))
	var enc EncryptionMode
	switch encRaw {
	case "off", "false", "0":
		enc = EncryptionOff
	case "on", "true", "1":
		enc = EncryptionOn
	default:
		return nil, fmt.Errorf("GV_ENCRYPTION must be off or on, got %q", encRaw)
	}

	idleMin, _ := strconv.Atoi(getEnv("GV_SESSION_IDLE_MINUTES", "60"))
	if idleMin <= 0 {
		idleMin = 60
	}
	maxH, _ := strconv.Atoi(getEnv("GV_SESSION_MAX_HOURS", "12"))
	if maxH <= 0 {
		maxH = 12
	}

	return &Config{
		HTTPAddr:            normalizeListenAddr(getEnv("GV_HTTP_ADDR", ":8080")),
		TuningFile:          getEnv("GV_Tuning_FILE", "configs/gvsvd.yaml"),
		DebugAuthLog:        parseBoolEnv("GV_DEBUG_AUTH", false),
		DebugAuthFull:       parseBoolEnv("GV_DEBUG_AUTH_FULL", false),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		Encryption:          enc,
		OpenAIAPIKey:        os.Getenv("OPENAI_API_KEY"),
		OpenAIBaseURL:       getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		OpenAIEmbeddingsURL: strings.TrimSpace(os.Getenv("OPENAI_EMBEDDINGS_URL")),
		SessionIdle:         time.Duration(idleMin) * time.Minute,
		SessionMax:          time.Duration(maxH) * time.Hour,
		EmbeddingDimension:  1536,
	}, nil
}

func (c *Config) Validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.Encryption == EncryptionOn && c.OpenAIAPIKey == "" {
		_ = true
	}
	return nil
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// parseBoolEnv treats empty as def; accepts 1/t/true/yes/on and 0/f/false/no/off.
func parseBoolEnv(key string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return def
	}
	switch v {
	case "1", "t", "true", "yes", "y", "on":
		return true
	case "0", "f", "false", "no", "n", "off":
		return false
	default:
		return def
	}
}

// normalizeListenAddr makes net.Listen("tcp", addr) accept common env mistakes:
// bare port "8080" -> ":8080"; trims spaces.
func normalizeListenAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ":8080"
	}
	if strings.Contains(addr, ":") {
		return addr
	}
	if _, err := strconv.Atoi(addr); err == nil {
		return ":" + addr
	}
	return addr
}
