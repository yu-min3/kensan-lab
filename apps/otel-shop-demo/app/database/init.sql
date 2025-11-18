-- Database initialization script for otel-shop-demo

-- Create products table
CREATE TABLE IF NOT EXISTS products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price DECIMAL(10, 2) NOT NULL,
    image_url VARCHAR(512),
    stock INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Create carts table
CREATE TABLE IF NOT EXISTS carts (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(100) NOT NULL,
    product_id INT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    quantity INT DEFAULT 1,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, product_id)
);

-- Create orders table
CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(100) NOT NULL,
    total_amount DECIMAL(10, 2) NOT NULL,
    status VARCHAR(50) DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT NOW()
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_carts_user_id ON carts(user_id);
CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id);
CREATE INDEX IF NOT EXISTS idx_products_name ON products(name);

-- Insert sample products
INSERT INTO products (name, description, price, image_url, stock) VALUES
('Laptop Pro 15"', 'High-performance laptop with 16GB RAM and 512GB SSD', 1299.99, 'https://images.unsplash.com/photo-1496181133206-80ce9b88a853?w=400', 10),
('Wireless Mouse', 'Ergonomic wireless mouse with precision tracking', 29.99, 'https://images.unsplash.com/photo-1527814050087-3793815479db?w=400', 50),
('Mechanical Keyboard', 'RGB mechanical keyboard with blue switches', 89.99, 'https://images.unsplash.com/photo-1595225476474-87563907a212?w=400', 30),
('USB-C Hub', '7-in-1 USB-C hub with HDMI, USB 3.0, and SD card reader', 49.99, 'https://images.unsplash.com/photo-1625948515291-69613efd103f?w=400', 25),
('4K Monitor 27"', 'Ultra HD 4K monitor with IPS panel and HDR support', 399.99, 'https://images.unsplash.com/photo-1527443224154-c4a3942d3acf?w=400', 15),
('Wireless Headphones', 'Noise-canceling over-ear headphones with 30h battery', 199.99, 'https://images.unsplash.com/photo-1505740420928-5e560c06d30e?w=400', 40),
('Webcam HD', '1080p webcam with built-in microphone and auto-focus', 79.99, 'https://images.unsplash.com/photo-1614624532983-4ce03382d63d?w=400', 35),
('External SSD 1TB', 'Portable SSD with USB 3.2 Gen 2 for fast data transfer', 129.99, 'https://images.unsplash.com/photo-1531492746076-161ca9bcad58?w=400', 20),
('Standing Desk', 'Electric height-adjustable standing desk with memory presets', 599.99, 'https://images.unsplash.com/photo-1595515106969-1ce29566ff1c?w=400', 8),
('Office Chair', 'Ergonomic mesh office chair with lumbar support', 299.99, 'https://images.unsplash.com/photo-1580480055273-228ff5388ef8?w=400', 12),
('Desk Lamp LED', 'Adjustable LED desk lamp with touch control and USB port', 39.99, 'https://images.unsplash.com/photo-1507473885765-e6ed057f782c?w=400', 45),
('Cable Management', 'Cable management kit with clips, sleeves, and ties', 19.99, 'https://images.unsplash.com/photo-1558618666-fcd25c85cd64?w=400', 60)
ON CONFLICT DO NOTHING;

-- Log initialization
DO $$
BEGIN
    RAISE NOTICE 'Database initialized successfully with % products', (SELECT COUNT(*) FROM products);
END $$;
