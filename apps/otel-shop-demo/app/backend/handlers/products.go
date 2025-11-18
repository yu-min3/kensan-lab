package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/yu-min3/otel-shop-demo/backend/database"
	"github.com/yu-min3/otel-shop-demo/backend/models"
	"github.com/yu-min3/otel-shop-demo/backend/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

// GetProducts returns all products
func GetProducts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := telemetry.GetTracer()

	ctx, span := tracer.Start(ctx, "GetProducts")
	defer span.End()

	query := "SELECT id, name, description, price, image_url, stock, created_at FROM products ORDER BY id"
	rows, err := database.QueryWithSpan(ctx, tracer, query)
	if err != nil {
		span.RecordError(err)
		http.Error(w, "Failed to fetch products", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	products := []models.Product{}
	for rows.Next() {
		var p models.Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.ImageURL, &p.Stock, &p.CreatedAt); err != nil {
			span.RecordError(err)
			continue
		}
		products = append(products, p)
	}

	span.SetAttributes(attribute.Int("products.count", len(products)))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(products)
}

// GetProductByID returns a single product by ID
func GetProductByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := telemetry.GetTracer()

	ctx, span := tracer.Start(ctx, "GetProductByID")
	defer span.End()

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		span.RecordError(err)
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.Int("product.id", id))

	query := "SELECT id, name, description, price, image_url, stock, created_at FROM products WHERE id = $1"
	row := database.QueryRowWithSpan(ctx, tracer, query, id)

	var p models.Product
	if err := row.Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.ImageURL, &p.Stock, &p.CreatedAt); err != nil {
		span.RecordError(err)
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

// SearchProducts searches for products by keyword
func SearchProducts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := telemetry.GetTracer()

	ctx, span := tracer.Start(ctx, "SearchProducts")
	defer span.End()

	keyword := r.URL.Query().Get("q")
	if keyword == "" {
		http.Error(w, "Search keyword is required", http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.String("search.keyword", keyword))

	query := `SELECT id, name, description, price, image_url, stock, created_at
	          FROM products
	          WHERE name ILIKE $1 OR description ILIKE $1
	          ORDER BY id`
	searchTerm := "%" + keyword + "%"

	rows, err := database.QueryWithSpan(ctx, tracer, query, searchTerm)
	if err != nil {
		span.RecordError(err)
		http.Error(w, "Failed to search products", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	products := []models.Product{}
	for rows.Next() {
		var p models.Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.ImageURL, &p.Stock, &p.CreatedAt); err != nil {
			span.RecordError(err)
			continue
		}
		products = append(products, p)
	}

	span.SetAttributes(attribute.Int("search.results", len(products)))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(products)
}
