package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct{
	DbPath string
}

func Load()*Config{
	godotenv.Load()

	return &Config{
		DbPath: getEnv("DB_PATH", "../"),
	}
	
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}