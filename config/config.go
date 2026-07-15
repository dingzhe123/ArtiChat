package config

import (
	"fmt"
	"os"
	"strings"
)

// Config 保存所有应用配置项。
type Config struct {
	Port           string // 监听端口
	DBPath         string // SQLite 数据库路径
	LLMAPIKey      string // 大模型 API 密钥
	LLMBaseURL     string // 大模型 API 基础地址
	LLMModel       string // 对话模型名称
	EmbeddingModel string // Embedding 模型名称
	AdminUser      string // 管理后台用户名
	AdminPass      string // 管理后台密码
}

// Load 加载配置：优先环境变量，其次 .env 文件，最后使用默认值（仅限非敏感项）。
func Load() *Config {
	// 加载 .env 文件（如存在），环境变量优先级更高
	_ = loadDotEnv(".env")

	cfg := &Config{
		Port:           getEnvDefault("PORT", "8080"),
		DBPath:         getEnvDefault("DB_PATH", "data/site.db"),
		LLMAPIKey:      os.Getenv("LLM_API_KEY"),                      // 必须由用户配置
		LLMBaseURL:     getEnvDefault("LLM_BASE_URL", "https://api.deepseek.com/v1"),
		LLMModel:       getEnvDefault("LLM_MODEL", "deepseek-chat"),
		EmbeddingModel: getEnvDefault("EMBEDDING_MODEL", "text-embedding-ada-002"),
		AdminUser:      getEnvDefault("ADMIN_USER", "admin"),
		AdminPass:      getEnvDefault("ADMIN_PASS", "admin123"),
	}

	if cfg.LLMAPIKey == "" {
		fmt.Fprintln(os.Stderr, "警告: LLM_API_KEY 未设置，聊天和 RAG 功能将不可用。")
	}

	return cfg
}

// getEnvDefault 读取环境变量，未设置则返回 fallback。
func getEnvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// loadDotEnv 解析简单的 KEY=VALUE 格式 .env 文件，将键值写入环境变量。
// 跳过空行和 # 开头的注释。环境变量已有值时不会被覆盖。
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
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
	return nil
}
