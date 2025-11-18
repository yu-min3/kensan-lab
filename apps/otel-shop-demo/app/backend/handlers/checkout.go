package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/yu-min3/otel-shop-demo/backend/database"
	"github.com/yu-min3/otel-shop-demo/backend/models"
	"github.com/yu-min3/otel-shop-demo/backend/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

// Checkout processes the checkout for a user's cart
func Checkout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := telemetry.GetTracer()

	ctx, span := tracer.Start(ctx, "Checkout")
	defer span.End()

	var req models.CheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.RecordError(err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.String("user.id", req.UserID))

	// Start a transaction
	db := database.GetDB()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		span.RecordError(err)
		http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Get cart items
	cartQuery := `
		SELECT c.product_id, c.quantity, p.price, p.stock
		FROM carts c
		JOIN products p ON c.product_id = p.id
		WHERE c.user_id = $1
	`

	rows, err := tx.QueryContext(ctx, cartQuery, req.UserID)
	if err != nil {
		span.RecordError(err)
		http.Error(w, "Failed to fetch cart", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var totalAmount float64
	type cartData struct {
		productID int
		quantity  int
		price     float64
		stock     int
	}
	cartItems := []cartData{}

	for rows.Next() {
		var item cartData
		if err := rows.Scan(&item.productID, &item.quantity, &item.price, &item.stock); err != nil {
			span.RecordError(err)
			continue
		}

		// Check stock availability
		if item.stock < item.quantity {
			http.Error(w, "Insufficient stock for one or more items", http.StatusBadRequest)
			return
		}

		totalAmount += item.price * float64(item.quantity)
		cartItems = append(cartItems, item)
	}

	if len(cartItems) == 0 {
		http.Error(w, "Cart is empty", http.StatusBadRequest)
		return
	}

	span.SetAttributes(
		attribute.Float64("order.total_amount", totalAmount),
		attribute.Int("order.items_count", len(cartItems)),
	)

	// Create order
	orderQuery := "INSERT INTO orders (user_id, total_amount, status) VALUES ($1, $2, $3) RETURNING id"
	var orderID int
	err = tx.QueryRowContext(ctx, orderQuery, req.UserID, totalAmount, "completed").Scan(&orderID)
	if err != nil {
		span.RecordError(err)
		http.Error(w, "Failed to create order", http.StatusInternalServerError)
		return
	}

	span.SetAttributes(attribute.Int("order.id", orderID))

	// Update product stock
	for _, item := range cartItems {
		updateStockQuery := "UPDATE products SET stock = stock - $1 WHERE id = $2"
		_, err := tx.ExecContext(ctx, updateStockQuery, item.quantity, item.productID)
		if err != nil {
			span.RecordError(err)
			http.Error(w, "Failed to update stock", http.StatusInternalServerError)
			return
		}
	}

	// Clear cart
	clearCartQuery := "DELETE FROM carts WHERE user_id = $1"
	_, err = tx.ExecContext(ctx, clearCartQuery, req.UserID)
	if err != nil {
		span.RecordError(err)
		http.Error(w, "Failed to clear cart", http.StatusInternalServerError)
		return
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		span.RecordError(err)
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"order_id":     orderID,
		"total_amount": totalAmount,
		"status":       "completed",
		"message":      "Checkout successful",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// GetOrders returns all orders for a user
func GetOrders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := telemetry.GetTracer()

	ctx, span := tracer.Start(ctx, "GetOrders")
	defer span.End()

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.String("user.id", userID))

	query := "SELECT id, user_id, total_amount, status, created_at FROM orders WHERE user_id = $1 ORDER BY created_at DESC"
	rows, err := database.QueryWithSpan(ctx, tracer, query, userID)
	if err != nil {
		span.RecordError(err)
		http.Error(w, "Failed to fetch orders", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	orders := []models.Order{}
	for rows.Next() {
		var order models.Order
		if err := rows.Scan(&order.ID, &order.UserID, &order.TotalAmount, &order.Status, &order.CreatedAt); err != nil {
			span.RecordError(err)
			continue
		}
		orders = append(orders, order)
	}

	span.SetAttributes(attribute.Int("orders.count", len(orders)))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
}
