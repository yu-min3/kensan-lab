package main

import (
	"log/slog"
	"os"

	"github.com/kensan/backend/services/memo/internal/handler"
	"github.com/kensan/backend/services/memo/internal/repository"
	"github.com/kensan/backend/services/memo/internal/service"
	"github.com/kensan/backend/shared/bootstrap"
)

func main() {
	// Initialize service with common configuration
	svc, err := bootstrap.New("memo-service")
	if err != nil {
		slog.Error("Failed to initialize service", "error", err)
		os.Exit(1)
	}
	defer svc.Close()

	// Setup repository, service, and handler
	memoRepo := repository.NewPostgresRepository(svc.Pool)
	memoService := service.NewService(memoRepo)
	memoHandler := handler.NewHandler(memoService)

	// Register routes
	svc.RegisterRoutes(memoHandler.RegisterRoutes)

	// Run server
	if err := svc.Run(); err != nil {
		slog.Error("Server error", "error", err)
		os.Exit(1)
	}
}
