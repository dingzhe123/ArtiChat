package config

import "os"

type Config struct {
	Port       string
	DBPath     string
	LLMAPIKey  string
	LLMBaseURL string
	LLMModel   string
	AdminUser  string
	AdminPass  string
}

func Load() *Config {
	return &Config{
		Port:       getEnv("PORT", "8080"),
		DBPath:     getEnv("DB_PATH", "data/site.db"),
		LLMAPIKey:  getEnv("LLM_API_KEY", "sk-dbf87afb1133f8deaed84b14347190297b548dc3e43fd1f790c8c1074f53ef43110ff903"),
		LLMBaseURL: getEnv("LLM_BASE_URL", "http://115.190.217.1:38080/v1"),
		LLMModel:   getEnv("LLM_MODEL", "testapi-flash"),
		AdminUser:  getEnv("ADMIN_USER", "admin"),
		AdminPass:  getEnv("ADMIN_PASS", "admin123"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
