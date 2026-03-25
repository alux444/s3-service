package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port                         string
	LogLevel                     string
	DatabaseUrl                  string
	DatabaseMigrationsDir        string
	DatabaseMigrateOnStartup     bool
	DatabaseSchemaCheckOnStartup bool
	JWTIssuer                    string
	JWTAudience                  string
	JWTJWKSURL                   string
	JWTEnabled                   bool
}

func LoadFromEnv() Config {
	return Config{
		Port:                         envOrDefault("PORT", "8080"),
		LogLevel:                     envOrDefault("LOG_LEVEL", "info"),
		DatabaseUrl:                  envOrDefault("DATABASE_URL", ""),
		DatabaseMigrationsDir:        envOrDefault("DB_MIGRATIONS_DIR", "./migrations"),
		DatabaseMigrateOnStartup:     envOrDefault("DB_MIGRATE_ON_STARTUP", "true") == "true",
		DatabaseSchemaCheckOnStartup: envOrDefault("DB_SCHEMA_CHECK_ON_STARTUP", "true") == "true",
		JWTIssuer:                    envOrDefault("JWT_ISSUER", ""),
		JWTAudience:                  envOrDefault("JWT_AUDIENCE", ""),
		JWTJWKSURL:                   envOrDefault("JWT_JWKS_URL", ""),
		JWTEnabled:                   envOrDefault("JWT_ENABLED", "false") == "true",
	}
}

func (c Config) Validate() error {
	var missing []string
	if c.DatabaseUrl == "" {
		missing = append(missing, "DATABASE_URL")
	}

	if c.JWTEnabled {
		if c.JWTIssuer == "" {
			missing = append(missing, "JWT_ISSUER")
		}
		if c.JWTAudience == "" {
			missing = append(missing, "JWT_AUDIENCE")
		}
		if c.JWTJWKSURL == "" {
			missing = append(missing, "JWT_JWKS_URL")
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variable(s): %v", missing)
	}

	return nil
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
