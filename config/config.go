package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port           string
	DBPath         string
	LLMAPIKey      string
	LLMBaseURL     string
	LLMModel       string
	EmbeddingModel string
	AdminUser      string
	AdminPass      string
}

func Load() *Config {
	// Load .env file if present (ignore errors — env vars take precedence)
	_ = loadDotEnv(".env")

	cfg := &Config{
		Port:           getEnvDefault("PORT", "8080"),
		DBPath:         getEnvDefault("DB_PATH", "data/site.db"),
		LLMAPIKey:      os.Getenv("LLM_API_KEY"),
		LLMBaseURL:     getEnvDefault("LLM_BASE_URL", "https://api.deepseek.com/v1"),
		LLMModel:       getEnvDefault("LLM_MODEL", "deepseek-chat"),
		EmbeddingModel: getEnvDefault("EMBEDDING_MODEL", "text-embedding-ada-002"),
		AdminUser:      getEnvDefault("ADMIN_USER", "admin"),
		AdminPass:      getEnvDefault("ADMIN_PASS", "admin123"),
	}

	if cfg.LLMAPIKey == "" {
		fmt.Fprintln(os.Stderr, "WARNING: LLM_API_KEY is not set. Chat and RAG features will fail.")
	}

	return cfg
}

func getEnvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// loadDotEnv reads a simple KEY=VALUE .env file and sets os.Environ for each line.
// It skips blank lines and comments starting with #. Values are not quoted; quotes
// become part of the value.
func loadDotEnv(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		// Only set if not already in environment
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
	return nil
}
