package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/joho/godotenv"
)

type Config struct {
	AppName string
	Version string
}

var (
	cfg  *Config
	once sync.Once
)

func Load() *Config {
	once.Do(func() {
		root, err := projectRoot()
		if err != nil {
			root = "."
		}
		envPath := filepath.Join(root, ".env")
		_ = godotenv.Load(envPath)

		cfg = &Config{
			AppName: envOr("APP_NAME", "Rum"),
			Version: envOr("VERSION", "0.1.1"),
		}
	})
	return cfg
}

func LoadFrom(path string) *Config {
	once.Do(func() {
		_ = godotenv.Load(path)
		cfg = &Config{
			AppName: envOr("APP_NAME", "Rum"),
			Version: envOr("VERSION", "0.1.1"),
		}
	})
	return cfg
}

func envOr(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}

func projectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}
