// Package bootstrap provides common service initialization utilities.
package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kensan/backend/shared/auth"
	"github.com/kensan/backend/shared/config"
	"github.com/kensan/backend/shared/database"
	"github.com/kensan/backend/shared/logging"
	"github.com/kensan/backend/shared/middleware"
	"github.com/kensan/backend/shared/telemetry"
)

// Service represents a microservice with its configuration and dependencies.
type Service struct {
	Name         string
	Config       *config.Config
	Pool         *pgxpool.Pool
	JWTManager   *auth.JWTManager
	Router       chi.Router
	apiRouter    chi.Router // sub-router for /api/v1 routes
	writeTimeout time.Duration
	otelProvider *telemetry.Provider
}

// RouteRegistrar is a function that registers routes on a chi.Router.
// It receives the router with authentication middleware already applied.
type RouteRegistrar func(r chi.Router)

// Option is a function that configures a Service.
type Option func(*Service)

// WithWriteTimeout sets a custom write timeout for the HTTP server.
// Useful for services that handle long-running requests (e.g., AI service).
func WithWriteTimeout(d time.Duration) Option {
	return func(s *Service) {
		s.writeTimeout = d
	}
}

// New creates a new Service with all common dependencies initialized.
// It sets up logging, database connection, JWT manager, and router with middleware.
func New(name string, opts ...Option) (*Service, error) {
	// Setup logging (stdout/stderr only, OTel not yet initialized)
	cfg := config.Load()
	logging.Setup(cfg.Server.Env)

	// Setup OpenTelemetry
	ctx := context.Background()
	otelProvider, err := telemetry.Initialize(ctx, telemetry.Config{
		ServiceName:  name,
		Environment:  cfg.Server.Env,
		CollectorURL: cfg.Telemetry.CollectorURL,
		Enabled:      cfg.Telemetry.Enabled,
	})
	if err != nil {
		slog.Warn("Failed to initialize OpenTelemetry, continuing without it", "error", err)
		otelProvider = &telemetry.Provider{}
	}

	// Re-configure logging with OTel bridge if available
	if lp := otelProvider.LoggerProvider(); lp != nil {
		logging.SetupWithOTel(cfg.Server.Env, lp)
	}

	if cfg.Telemetry.Enabled {
		slog.Info("OpenTelemetry initialized", "collector", cfg.Telemetry.CollectorURL)
	}

	// Setup database connection
	pool, err := database.NewPostgresPool(ctx, cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	slog.Info("Connected to database")

	// Setup JWT manager
	jwtManager := auth.NewJWTManager(cfg.JWT)

	// Setup router with common middleware
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.OTelTrace(name))
	r.Use(middleware.Metrics)
	r.Use(middleware.Logger)
	r.Use(corsMiddleware())

	// Health check endpoint (no auth required)
	r.Get("/health", healthHandler(name))

	// Create API router for /api/v1 prefix
	apiRouter := chi.NewRouter()
	r.Mount("/api/v1", apiRouter)

	svc := &Service{
		Name:         name,
		Config:       cfg,
		Pool:         pool,
		JWTManager:   jwtManager,
		Router:       r,
		apiRouter:    apiRouter,
		writeTimeout: 15 * time.Second, // default
		otelProvider: otelProvider,
	}

	// Apply options
	for _, opt := range opts {
		opt(svc)
	}

	return svc, nil
}

// RegisterRoutes registers API routes with authentication middleware.
// Routes are registered under /api/v1 prefix.
func (s *Service) RegisterRoutes(registrar RouteRegistrar) {
	s.apiRouter.Group(func(r chi.Router) {
		r.Use(middleware.Auth(s.JWTManager))
		registrar(r)
	})
}

// RegisterPublicRoutes registers API routes without authentication.
// Routes are registered under /api/v1 prefix.
func (s *Service) RegisterPublicRoutes(registrar RouteRegistrar) {
	s.apiRouter.Group(func(r chi.Router) {
		registrar(r)
	})
}

// Run starts the HTTP server and blocks until shutdown signal is received.
func (s *Service) Run() error {
	addr := fmt.Sprintf("%s:%d", s.Config.Server.Host, s.Config.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      s.Router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: s.writeTimeout,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		slog.Info("Starting service", "addr", addr, "service", s.Name)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for interrupt signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case <-quit:
		slog.Info("Shutting down server...", "service", s.Name)
	}

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	slog.Info("Server exited properly", "service", s.Name)
	return nil
}

// Close releases all resources held by the service.
func (s *Service) Close() {
	if s.otelProvider != nil {
		if err := s.otelProvider.Shutdown(context.Background()); err != nil {
			slog.Warn("Failed to shutdown OpenTelemetry", "error", err)
		}
	}
	if s.Pool != nil {
		s.Pool.Close()
	}
}

// corsMiddleware returns the CORS middleware with common configuration.
func corsMiddleware() func(http.Handler) http.Handler {
	origins := []string{"http://localhost:*", "https://*.kensan.dev"}
	if extra := os.Getenv("CORS_ALLOWED_ORIGINS"); extra != "" {
		origins = append(origins, strings.Split(extra, ",")...)
	}
	return cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID", "traceparent", "tracestate"},
		ExposedHeaders:   []string{"X-Request-ID", "traceparent", "tracestate"},
		AllowCredentials: true,
		MaxAge:           300,
	})
}

// healthHandler returns a handler for health check endpoint.
func healthHandler(serviceName string) http.HandlerFunc {
	response := fmt.Sprintf(`{"status":"healthy","service":"%s"}`, serviceName)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}
}
