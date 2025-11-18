// API client for backend communication
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '/api';

export const api = {
  // Products
  async getProducts() {
    const response = await fetch(`${API_BASE_URL}/products`);
    if (!response.ok) throw new Error('Failed to fetch products');
    return response.json();
  },

  async getProduct(id) {
    const response = await fetch(`${API_BASE_URL}/products/${id}`);
    if (!response.ok) throw new Error('Failed to fetch product');
    return response.json();
  },

  async searchProducts(keyword) {
    const response = await fetch(`${API_BASE_URL}/products/search?q=${encodeURIComponent(keyword)}`);
    if (!response.ok) throw new Error('Failed to search products');
    return response.json();
  },

  // Cart
  async addToCart(userId, productId, quantity = 1) {
    const response = await fetch(`${API_BASE_URL}/cart`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ user_id: userId, product_id: productId, quantity }),
    });
    if (!response.ok) throw new Error('Failed to add to cart');
    return response.json();
  },

  async getCart(userId) {
    const response = await fetch(`${API_BASE_URL}/cart/${userId}`);
    if (!response.ok) throw new Error('Failed to fetch cart');
    return response.json();
  },

  async removeFromCart(userId, productId) {
    const response = await fetch(`${API_BASE_URL}/cart/${userId}/${productId}`, {
      method: 'DELETE',
    });
    if (!response.ok) throw new Error('Failed to remove from cart');
    return response.json();
  },

  // Checkout
  async checkout(userId) {
    const response = await fetch(`${API_BASE_URL}/checkout`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ user_id: userId }),
    });
    if (!response.ok) throw new Error('Failed to checkout');
    return response.json();
  },

  async getOrders(userId) {
    const response = await fetch(`${API_BASE_URL}/orders?user_id=${userId}`);
    if (!response.ok) throw new Error('Failed to fetch orders');
    return response.json();
  },
};
