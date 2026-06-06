package main

import (
	"log/slog"
	"os"

	"github.com/kensan/backend/services/analytics/internal/handler"
	"github.com/kensan/backend/services/analytics/internal/repository"
	"github.com/kensan/backend/services/analytics/internal/service"
	"github.com/kensan/backend/shared/bootstrap"
)

func main() {
	// Initialize service with common configuration
	svc, err := bootstrap.New("analytics-service")
	if err != nil {
		slog.Error("Failed to initialize service", "error", err)
		os.Exit(1)
	}
	defer svc.Close()

	// Setup repository, service, and handler
	analyticsRepo := repository.NewPostgresRepository(svc.Pool)
	analyticsService := service.NewService(analyticsRepo)
	analyticsHandler := handler.NewHandler(analyticsService)

	// Register routes
	svc.RegisterRoutes(analyticsHandler.RegisterRoutes)

	// Run server
	if err := svc.Run(); err != nil {
		slog.Error("Server error", "error", err)
		os.Exit(1)
	}
}
