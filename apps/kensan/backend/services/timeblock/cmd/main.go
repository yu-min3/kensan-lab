package main

import (
	"log/slog"
	"os"

	"github.com/kensan/backend/services/timeblock/internal/handler"
	"github.com/kensan/backend/services/timeblock/internal/repository"
	"github.com/kensan/backend/services/timeblock/internal/service"
	"github.com/kensan/backend/shared/bootstrap"
)

func main() {
	// Initialize service with common configuration
	svc, err := bootstrap.New("timeblock-service")
	if err != nil {
		slog.Error("Failed to initialize service", "error", err)
		os.Exit(1)
	}
	defer svc.Close()

	// Setup repository, service, and handler
	timeblockRepo := repository.NewPostgresRepository(svc.Pool)
	timeblockService := service.NewService(timeblockRepo)
	timeblockHandler := handler.NewHandler(timeblockService)

	// Register routes
	svc.RegisterRoutes(timeblockHandler.RegisterRoutes)

	// Run server
	if err := svc.Run(); err != nil {
		slog.Error("Server error", "error", err)
		os.Exit(1)
	}
}
