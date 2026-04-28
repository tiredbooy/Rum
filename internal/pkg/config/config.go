package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	AppName string
	Version string
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = ".env"
	}

	_ = godotenv.Load(path)

	cfg := &Config{
		AppName: envOr("APP_NAME", "Rum"),
		Version: envOr("VERSION", "0.1.1"),
	}

	return cfg, nil
}

func envOr(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		log.Println("VALUE: ", val)
		return val
	}
	return fallback
}
