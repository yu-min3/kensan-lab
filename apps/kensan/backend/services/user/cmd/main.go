package main

import (
	"log/slog"
	"os"

	"github.com/kensan/backend/services/user/internal/demo"
	"github.com/kensan/backend/services/user/internal/handler"
	"github.com/kensan/backend/services/user/internal/repository"
	"github.com/kensan/backend/services/user/internal/service"
	"github.com/kensan/backend/shared/bootstrap"
)

func main() {
	// Initialize service with common configuration
	svc, err := bootstrap.New("user-service")
	if err != nil {
		slog.Error("Failed to initialize service", "error", err)
		os.Exit(1)
	}
	defer svc.Close()

	// Setup repository, service, and handler
	// Note: user service needs JWTManager for token generation
	userRepo := repository.NewPostgresRepository(svc.Pool)
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
