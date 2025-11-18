package models

import "time"

// Product represents a product in the shop
type Product struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	ImageURL    string    `json:"image_url"`
	Stock       int       `json:"stock"`
	CreatedAt   time.Time `json:"created_at"`
}

// CartItem represents an item in the shopping cart
type CartItem struct {
	ID        int       `json:"id"`
	UserID    string    `json:"user_id"`
	ProductID int       `json:"product_id"`
	Quantity  int       `json:"quantity"`
	CreatedAt time.Time `json:"created_at"`
}

// CartItemWithProduct combines cart item with product details
type CartItemWithProduct struct {
	CartItem
	Product Product `json:"product"`
}

// Order represents a completed order
type Order struct {
	ID          int       `json:"id"`
	UserID      string    `json:"user_id"`
	TotalAmount float64   `json:"total_amount"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

// AddToCartRequest represents the request body for adding items to cart
type AddToCartRequest struct {
	UserID    string `json:"user_id"`
	ProductID int    `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

// CheckoutRequest represents the checkout request
type CheckoutRequest struct {
	UserID string `json:"user_id"`
}
