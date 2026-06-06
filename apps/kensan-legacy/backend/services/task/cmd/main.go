package main

import (
	"log/slog"
	"os"

	"github.com/kensan/backend/services/task/internal/handler"
	"github.com/kensan/backend/services/task/internal/repository"
	"github.com/kensan/backend/services/task/internal/service"
	"github.com/kensan/backend/shared/bootstrap"
)

func main() {
	// Initialize service with common configuration
	svc, err := bootstrap.New("task-service")
	if err != nil {
		slog.Error("Failed to initialize service", "error", err)
		os.Exit(1)
	}
	defer svc.Close()

	// Setup repository, service, and handler
	taskRepo := repository.NewPostgresRepository(svc.Pool)
	taskService := service.NewService(taskRepo)
	taskHandler := handler.NewHandler(taskService)

	// Register routes
	svc.RegisterRoutes(taskHandler.RegisterRoutes)

	// Run server
	if err := svc.Run(); err != nil {
		slog.Error("Server error", "error", err)
		os.Exit(1)
	}
}
