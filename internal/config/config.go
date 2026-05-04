package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	// DB
	DbPath string

	// LLM
	LLMBaseURL string
	LLMModel   string

	// Pipeline
	Workers          int
	BatchSize        int
	ExampleBatchSize int
	Limit            int
	MaxChars         int
}

func Load() *Config {
	// .env — опционально, флаги перебивают
	_ = godotenv.Load()

	return &Config{
		DbPath:     getEnv("DB_PATH", "./dictionary.db"),
		LLMBaseURL: getEnv("LLM_BASE_URL", "http://127.0.0.1:1234/v1"),
		LLMModel:   getEnv("LLM_MODEL", "google/gemma-4-e4b"),
		Workers:    getEnvInt("WORKERS", 1),
		BatchSize:        getEnvInt("BATCH_SIZE", 20),
		ExampleBatchSize: getEnvInt("EXAMPLE_BATCH_SIZE", 3),
		Limit:      getEnvInt("LIMIT", 0),
		MaxChars:   getEnvInt("MAX_CHARS", 2),
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if n, err := strconv.Atoi(value); err == nil {
			return n
		}
	}
	return defaultValue
}
