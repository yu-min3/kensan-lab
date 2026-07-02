package main

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/kensan/backend/services/user/internal/demo"
	"github.com/kensan/backend/services/user/internal/handler"
	"github.com/kensan/backend/services/user/internal/repository"
	"github.com/kensan/backend/services/user/internal/service"
	"github.com/kensan/backend/shared/bootstrap"
	"github.com/kensan/backend/shared/vault"
)

func main() {
	// Initialize service with common configuration
	svc, err := bootstrap.New("user-service")
	if err != nil {
		slog.Error("Failed to initialize service", "error", err)
		os.Exit(1)
	}
	defer svc.Close()

	// Vault Transit encryptor for users.name (Stage 6).
	// Falls back to NoOpEncryptor when VAULT_ADDR is unset (local dev).
	encryptor := buildEncryptor(context.Background())
	defer func() {
		if c, ok := encryptor.(*vault.Client); ok {
			c.Close()
		}
	}()

	// Setup repository, service, and handler
	// Note: user service needs JWTManager for token generation
	userRepo := repository.NewPostgresRepository(svc.Pool, encryptor)
	userService := service.NewService(userRepo, svc.JWTManager)
	userHandler := handler.NewHandler(userService)

	// Register public routes (no auth required)
	svc.RegisterPublicRoutes(userHandler.RegisterPublicRoutes)

	// Register demo login routes (public, no auth required)
	demoHandler := demo.NewHandler(svc.Pool, svc.JWTManager)
	svc.RegisterPublicRoutes(demoHandler.RegisterRoutes)

	// Register protected routes (auth required)
	svc.RegisterRoutes(userHandler.RegisterRoutes)

	// Run server
	if err := svc.Run(); err != nil {
		slog.Error("Server error", "error", err)
		os.Exit(1)
	}
}

// buildEncryptor returns a Vault Transit client when VAULT_ADDR is configured,
// otherwise NoOpEncryptor for local dev / docker-compose. It logs the choice so
// production misconfigurations are visible in logs.
func buildEncryptor(ctx context.Context) vault.Encryptor {
	cfg := vault.LoadConfigFromEnv()
	if cfg.KeyName == "" {
		cfg.KeyName = "users-name"
	}
	cli, err := vault.NewClient(ctx, cfg)
	if err == nil {
		slog.Info("Vault Transit encryptor initialized",
			"address", cfg.Address,
			"role", cfg.Role,
			"mount", cfg.MountPath,
			"key", cfg.KeyName)
		return cli
	}
	if errors.Is(err, vault.ErrNotConfigured) {
		slog.Warn("VAULT_ADDR unset — using NoOpEncryptor (DO NOT use in production)")
		return vault.NoOpEncryptor{}
	}
	slog.Error("Vault Transit init failed", "error", err)
	os.Exit(1)
	return nil // unreachable
}
