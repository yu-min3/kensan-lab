import { useState, useEffect } from 'react';
import ProductList from './components/ProductList';
import Cart from './components/Cart';
import { api } from './services/api';
import './App.css';

// Mock user ID (in real app, this would come from authentication)
const USER_ID = 'demo-user';

function App() {
  const [products, setProducts] = useState([]);
  const [cart, setCart] = useState([]);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [showCart, setShowCart] = useState(false);

  // Load products on mount
  useEffect(() => {
    loadProducts();
    loadCart();
  }, []);

  const loadProducts = async () => {
    try {
      setLoading(true);
      setError(null);
      const data = await api.getProducts();
      setProducts(data);
    } catch (err) {
      setError('Failed to load products: ' + err.message);
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const loadCart = async () => {
    try {
      const data = await api.getCart(USER_ID);
      setCart(data || []);
    } catch (err) {
      console.error('Failed to load cart:', err);
    }
  };

  const handleSearch = async (e) => {
    e.preventDefault();
    if (!searchKeyword.trim()) {
      loadProducts();
      return;
    }

    try {
      setLoading(true);
      setError(null);
      const data = await api.searchProducts(searchKeyword);
      setProducts(data);
    } catch (err) {
      setError('Search failed: ' + err.message);
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const handleAddToCart = async (productId) => {
    try {
      await api.addToCart(USER_ID, productId, 1);
      await loadCart();
      alert('Added to cart!');
    } catch (err) {
      alert('Failed to add to cart: ' + err.message);
      console.error(err);
    }
  };

  const handleRemoveFromCart = async (productId) => {
    try {
      await api.removeFromCart(USER_ID, productId);
      await loadCart();
    } catch (err) {
      alert('Failed to remove from cart: ' + err.message);
      console.error(err);
    }
  };

  const handleCheckout = async () => {
    if (cart.length === 0) {
      alert('Cart is empty');
      return;
    }

    try {
      const result = await api.checkout(USER_ID);
      alert(`Order completed! Order ID: ${result.order_id}, Total: $${result.total_amount.toFixed(2)}`);
      await loadCart();
      setShowCart(false);
    } catch (err) {
      alert('Checkout failed: ' + err.message);
      console.error(err);
    }
  };

  const cartItemCount = cart.reduce((sum, item) => sum + item.quantity, 0);
  const cartTotal = cart.reduce((sum, item) => sum + (item.product.price * item.quantity), 0);

  return (
    <div className="app">
      <header className="header">
        <h1>🛒 OTel Shop Demo</h1>
        <div className="header-actions">
          <button
            className="cart-button"
            onClick={() => setShowCart(!showCart)}
          >
            🛒 Cart ({cartItemCount})
          </button>
        </div>
      </header>

      <div className="container">
        {showCart ? (
          <Cart
            items={cart}
            total={cartTotal}
            onRemove={handleRemoveFromCart}
            onCheckout={handleCheckout}
            onClose={() => setShowCart(false)}
          />
        ) : (
          <>
            <div className="search-bar">
              <form onSubmit={handleSearch}>
                <input
                  type="text"
                  placeholder="Search products..."
                  value={searchKeyword}
                  onChange={(e) => setSearchKeyword(e.target.value)}
                  className="search-input"
                />
                <button type="submit" className="search-button">Search</button>
                {searchKeyword && (
                  <button
                    type="button"
                    onClick={() => { setSearchKeyword(''); loadProducts(); }}
                    className="clear-button"
                  >
                    Clear
                  </button>
                )}
              </form>
            </div>

            {error && (
              <div className="error-message">{error}</div>
            )}

            {loading ? (
              <div className="loading">Loading...</div>
            ) : (
              <ProductList
                products={products}
                onAddToCart={handleAddToCart}
              />
            )}
          </>
        )}
      </div>
    </div>
  );
}

export default App;
