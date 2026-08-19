package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	// TranscripTonic
	WebhookSecret string

	// LLM (LM Studio, MiniMax, or any OpenAI-compatible endpoint)
	LMStudioURL    string
	LMModel        string
	LMStudioAPIKey string

	// Model lifecycle: load LMModel before a meeting, unload it after. Local
	// LM Studio only — leave off for hosted backends.
	LMManageModel bool
	LMSBin        string
	LMContextLen  int
	LMTTL         int

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

	cfg.WebhookSecret = getEnvOrDefault("TRANSCRIPTONIC_WEBHOOK_SECRET", os.Getenv("TRANSCRIBE_TONIC_WEBHOOK_SECRET")) // optional — skip verification if empty

	if v := os.Getenv("VAULT_PATH"); v != "" {
		cfg.VaultPath = v
	} else {
		missing = append(missing, "VAULT_PATH")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: %v\n→ copy .env.example → .env", missing)
	}

	cfg.LMStudioURL = getEnvOrDefault("LM_STUDIO_URL", "http://localhost:1234")
	cfg.LMModel = getEnvOrDefault("LM_MODEL", "qwen3-27b-instruct")
	cfg.LMStudioAPIKey = os.Getenv("LM_STUDIO_API_KEY") // optional — empty for keyless backends like LM Studio
	cfg.LMManageModel = getEnvBool("LM_MANAGE_MODEL", false)
	cfg.LMSBin = getEnvOrDefault("LMS_BIN", defaultLMSBin())
	cfg.LMContextLen = getEnvInt("LM_CONTEXT_LENGTH", 0) // 0 = LM Studio's per-model default
	cfg.LMTTL = getEnvInt("LM_TTL", 900)
	cfg.Host = getEnvOrDefault("HOST", "0.0.0.0")
	cfg.Port = getEnvInt("PORT", 5050)
	cfg.Debug = getEnvBool("DEBUG", false)

	return cfg, nil
}

// defaultLMSBin is where LM Studio installs its CLI. Falls back to a bare "lms"
// so a PATH install still works.
func defaultLMSBin() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "lms"
	}
	path := filepath.Join(home, ".lmstudio", "bin", "lms")
	if _, err := os.Stat(path); err != nil {
		return "lms"
	}
	return path
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
