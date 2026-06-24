package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	// Transcribe Tonic
	WebhookSecret string

	// LM Studio
	LMStudioURL string
	LMModel     string

	// Obsidian vault
	VaultPath string

	// Server
	Host  string
	Port  int
	Debug bool
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{}
	var missing []string

	cfg.WebhookSecret = os.Getenv("TRANSCRIBE_TONIC_WEBHOOK_SECRET") // optional — skip verification if empty

	if v := os.Getenv("VAULT_PATH"); v != "" {
		cfg.VaultPath = v
	} else {
		missing = append(missing, "VAULT_PATH")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: %v\n→ copy .env.example → .env", missing)
	}

	cfg.LMStudioURL = getEnvOrDefault("LM_STUDIO_URL", "http://localhost:1234")
	cfg.LMModel     = getEnvOrDefault("LM_MODEL", "qwen3-27b-instruct")
	cfg.Host        = getEnvOrDefault("HOST", "0.0.0.0")
	cfg.Port        = getEnvInt("PORT", 5050)
	cfg.Debug       = getEnvBool("DEBUG", false)

	return cfg, nil
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}
