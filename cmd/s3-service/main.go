package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"s3-service/internal/auth"
	"s3-service/internal/config"
	"s3-service/internal/database"
	httpmiddleware "s3-service/internal/httpapi/middleware"
	"s3-service/internal/httpapi/router"
	"s3-service/internal/service"
)

func main() {
	_ = godotenv.Load()
	cfg := config.LoadFromEnv()
	if err := cfg.Validate(); err != nil {
		os.Exit(1)
	}

	logger := newLogger(cfg.LogLevel)

	db, err := database.Open(context.Background(), cfg.DatabaseUrl)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}

	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("failed to close database connection", "error", err)
		}
	}()

	if cfg.DatabaseMigrateOnStartup {
		logger.Info("migrating database on startup")
		if err := database.MigrateUp(context.Background(), db, cfg.DatabaseMigrationsDir); err != nil {
			logger.Error("database migration failed", "error", err)
			os.Exit(1)
		}
	}

	if cfg.DatabaseSchemaCheckOnStartup {
		logger.Info("checking database schema on startup")
		if err := database.CheckSchema(context.Background(), db); err != nil {
			logger.Error("database schema check failed", "error", err)
			os.Exit(1)
		}
	}

	verifier, err := auth.NewJWTVerifier(auth.Config{
		Issuer:   cfg.JWTIssuer,
		Audience: cfg.JWTAudience,
		JWKSURL:  cfg.JWTJWKSURL,
		Enabled:  cfg.JWTEnabled,
	})
	if err != nil {
		logger.Error("failed to initialize JWT verifier", "error", err)
		os.Exit(1)
	}
	defer verifier.Close()

	ownershipRepo := database.NewOwnershipRepository(db)
	auditRepo := database.NewAuditRepository(db)
	bucketService := service.NewBucketConnectionsService(ownershipRepo)
	authorizationService := service.NewAuthorizationService(ownershipRepo)
	auditService := service.NewAuditService(auditRepo)

	handler := router.NewRouter(
		logger,
		httpmiddleware.JWTAuthMiddleware(logger, verifier),
		bucketService,
		authorizationService,
		auditService,
	)
	server := &http.Server{
		Addr:         cfg.Addr(),
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("starting s3-service", "addr", cfg.Addr())
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger.Info("shutting down s3-service")
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
}

func newLogger(level string) *slog.Logger {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
}
