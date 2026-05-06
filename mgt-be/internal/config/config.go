package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Addr     string
	GRPCAddr string
	BaseURL string
	UIBaseURL string

	DatabaseURL string

	OpenSearchURL string

	GitHubClientID     string
	GitHubClientSecret string
	GitHubWebhookSecret string

	MasterKey     []byte
	SessionSecret []byte

	LLMAPIKey  string
	LLMBaseURL string
	LLMModel   string

	WorkerEnabled bool
	WebhookPollFallbackSeconds int
}

func Load() (*Config, error) {
	c := &Config{
		Addr:        getenv("MGT_ADDR", ":8080"),
		GRPCAddr:    getenv("MGT_GRPC_ADDR", ":9090"),
		BaseURL:     strings.TrimRight(getenv("MGT_BASE_URL", "http://localhost:8080"), "/"),
		UIBaseURL:   strings.TrimRight(getenv("MGT_UI_BASE_URL", "http://localhost:3000"), "/"),
		DatabaseURL: getenv("DATABASE_URL", ""),
		OpenSearchURL: getenv("OPENSEARCH_URL", ""),
		GitHubClientID:     getenv("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret: getenv("GITHUB_CLIENT_SECRET", ""),
		GitHubWebhookSecret: getenv("GITHUB_WEBHOOK_SECRET", ""),
		LLMAPIKey:  getenv("LLM_API_KEY", ""),
		LLMBaseURL: strings.TrimRight(getenv("LLM_BASE_URL", "https://api.openai.com/v1"), "/"),
		LLMModel:   getenv("LLM_MODEL", "gpt-4o-mini"),
		WorkerEnabled: getenvBool("MGT_WORKER_ENABLED", true),
		WebhookPollFallbackSeconds: getenvInt("MGT_QUEUE_POLL_SECONDS", 30),
	}

	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	mkRaw := os.Getenv("MGT_MASTER_KEY")
	if mkRaw == "" {
		return nil, fmt.Errorf("MGT_MASTER_KEY is required (32+ chars)")
	}
	if len(mkRaw) < 32 {
		return nil, fmt.Errorf("MGT_MASTER_KEY must be at least 32 chars")
	}
	// Use the first 32 bytes as the AES-256 key.
	c.MasterKey = []byte(mkRaw)[:32]

	ssRaw := os.Getenv("SESSION_SECRET")
	if ssRaw == "" {
		return nil, fmt.Errorf("SESSION_SECRET is required (32+ chars)")
	}
	if len(ssRaw) < 32 {
		return nil, fmt.Errorf("SESSION_SECRET must be at least 32 chars")
	}
	c.SessionSecret = []byte(ssRaw)

	return c, nil
}

func (c *Config) LLMConfigured() bool {
	return c.LLMAPIKey != ""
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func getenvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
