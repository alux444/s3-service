package config

import "os"

type Config struct {
	Port     string
	LogLevel string
}

func LoadFromEnv() Config {
	return Config{
		Port:     envOrDefault("PORT", "8080"),
		LogLevel: envOrDefault("LOG_LEVEL", "info"),
	}
}

func (c Config) Addr() string {
	return ":" + c.Port
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
