package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/yu-min3/otel-shop-demo/backend/database"
	"github.com/yu-min3/otel-shop-demo/backend/models"
	"github.com/yu-min3/otel-shop-demo/backend/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

// AddToCart adds an item to the cart
func AddToCart(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := telemetry.GetTracer()

	ctx, span := tracer.Start(ctx, "AddToCart")
	defer span.End()

	var req models.AddToCartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.RecordError(err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == "" || req.ProductID == 0 || req.Quantity <= 0 {
		http.Error(w, "Invalid request: user_id, product_id, and quantity are required", http.StatusBadRequest)
		return
	}

	span.SetAttributes(
		attribute.String("user.id", req.UserID),
		attribute.Int("product.id", req.ProductID),
		attribute.Int("cart.quantity", req.Quantity),
	)

	// Check if product exists and has enough stock
	query := "SELECT stock FROM products WHERE id = $1"
	row := database.QueryRowWithSpan(ctx, tracer, query, req.ProductID)

	var stock int
	if err := row.Scan(&stock); err != nil {
		span.RecordError(err)
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}

	if stock < req.Quantity {
		http.Error(w, "Insufficient stock", http.StatusBadRequest)
		return
	}

	// Check if item already exists in cart
	checkQuery := "SELECT id, quantity FROM carts WHERE user_id = $1 AND product_id = $2"
	checkRow := database.QueryRowWithSpan(ctx, tracer, checkQuery, req.UserID, req.ProductID)

	var existingID int
	var existingQty int
	err := checkRow.Scan(&existingID, &existingQty)

	if err == nil {
		// Update existing cart item
		updateQuery := "UPDATE carts SET quantity = quantity + $1 WHERE id = $2"
		_, err := database.ExecWithSpan(ctx, tracer, updateQuery, req.Quantity, existingID)
		if err != nil {
			span.RecordError(err)
			http.Error(w, "Failed to update cart", http.StatusInternalServerError)
			return
		}
	} else {
		// Insert new cart item
		insertQuery := "INSERT INTO carts (user_id, product_id, quantity) VALUES ($1, $2, $3)"
		_, err := database.ExecWithSpan(ctx, tracer, insertQuery, req.UserID, req.ProductID, req.Quantity)
		if err != nil {
			span.RecordError(err)
			http.Error(w, "Failed to add to cart", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Item added to cart"})
}

// GetCart returns the cart for a user
func GetCart(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := telemetry.GetTracer()

	ctx, span := tracer.Start(ctx, "GetCart")
	defer span.End()

	userID := chi.URLParam(r, "userId")
	if userID == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.String("user.id", userID))

	query := `
		SELECT c.id, c.user_id, c.product_id, c.quantity, c.created_at,
		       p.id, p.name, p.description, p.price, p.image_url, p.stock, p.created_at
		FROM carts c
		JOIN products p ON c.product_id = p.id
		WHERE c.user_id = $1
		ORDER BY c.created_at DESC
	`

	rows, err := database.QueryWithSpan(ctx, tracer, query, userID)
	if err != nil {
		span.RecordError(err)
		http.Error(w, "Failed to fetch cart", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	items := []models.CartItemWithProduct{}
	for rows.Next() {
		var item models.CartItemWithProduct
		if err := rows.Scan(
			&item.ID, &item.UserID, &item.ProductID, &item.Quantity, &item.CreatedAt,
			&item.Product.ID, &item.Product.Name, &item.Product.Description,
			&item.Product.Price, &item.Product.ImageURL, &item.Product.Stock, &item.Product.CreatedAt,
		); err != nil {
			span.RecordError(err)
			continue
		}
		items = append(items, item)
	}

	span.SetAttributes(attribute.Int("cart.items", len(items)))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

// RemoveFromCart removes an item from the cart
func RemoveFromCart(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := telemetry.GetTracer()

	ctx, span := tracer.Start(ctx, "RemoveFromCart")
	defer span.End()

	userID := chi.URLParam(r, "userId")
	productID := chi.URLParam(r, "productId")

	if userID == "" || productID == "" {
		http.Error(w, "User ID and Product ID are required", http.StatusBadRequest)
		return
	}

	span.SetAttributes(
		attribute.String("user.id", userID),
		attribute.String("product.id", productID),
	)

	query := "DELETE FROM carts WHERE user_id = $1 AND product_id = $2"
	result, err := database.ExecWithSpan(ctx, tracer, query, userID, productID)
	if err != nil {
		span.RecordError(err)
		http.Error(w, "Failed to remove from cart", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "Item not found in cart", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Item removed from cart"})
}
