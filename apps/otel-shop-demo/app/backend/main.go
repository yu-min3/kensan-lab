package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/yu-min3/otel-shop-demo/backend/database"
	"github.com/yu-min3/otel-shop-demo/backend/handlers"
	"github.com/yu-min3/otel-shop-demo/backend/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	// Initialize OpenTelemetry tracer
	shutdown := telemetry.InitTracer()
	defer shutdown()

	// Initialize database
	if err := database.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.CloseDB()

	// Create router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS configuration
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "traceparent", "tracestate"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check endpoint
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// API routes with OpenTelemetry instrumentation
	r.Route("/api", func(r chi.Router) {
		// Products
		r.Get("/products", handlers.GetProducts)
		r.Get("/products/search", handlers.SearchProducts)
		r.Get("/products/{id}", handlers.GetProductByID)

		// Cart
		r.Post("/cart", handlers.AddToCart)
		r.Get("/cart/{userId}", handlers.GetCart)
		r.Delete("/cart/{userId}/{productId}", handlers.RemoveFromCart)

		// Checkout
		r.Post("/checkout", handlers.Checkout)
		r.Get("/orders", handlers.GetOrders)
	})

	// Wrap the router with OpenTelemetry HTTP instrumentation
	handler := otelhttp.NewHandler(r, "otel-shop-backend")

	// Get port from environment variable
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Start server
	log.Printf("Starting server on port %s", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
