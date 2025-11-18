import './Cart.css';

function Cart({ items, total, onRemove, onCheckout, onClose }) {
  return (
    <div className="cart-container">
      <div className="cart-header">
        <h2>Shopping Cart</h2>
        <button className="close-button" onClick={onClose}>✕</button>
      </div>

      {items.length === 0 ? (
        <div className="empty-cart">Your cart is empty</div>
      ) : (
        <>
          <div className="cart-items">
            {items.map((item) => (
              <div key={item.id} className="cart-item">
                <img
                  src={item.product.image_url}
                  alt={item.product.name}
                  className="cart-item-image"
                />
                <div className="cart-item-info">
                  <h4>{item.product.name}</h4>
                  <p>Quantity: {item.quantity}</p>
                  <p className="cart-item-price">
                    ${item.product.price.toFixed(2)} × {item.quantity} = ${(item.product.price * item.quantity).toFixed(2)}
                  </p>
                </div>
                <button
                  className="remove-button"
                  onClick={() => onRemove(item.product_id)}
                >
                  Remove
                </button>
              </div>
            ))}
          </div>

          <div className="cart-footer">
            <div className="cart-total">
              <strong>Total: ${total.toFixed(2)}</strong>
            </div>
            <button className="checkout-button" onClick={onCheckout}>
              Proceed to Checkout
            </button>
          </div>
        </>
      )}
    </div>
  );
}

export default Cart;
