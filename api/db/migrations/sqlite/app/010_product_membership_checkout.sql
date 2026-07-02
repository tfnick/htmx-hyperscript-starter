ALTER TABLE users ADD COLUMN membership_level TEXT NOT NULL DEFAULT 'basic';
ALTER TABLE users ADD COLUMN membership_expires_at DATETIME NOT NULL DEFAULT '2099-12-31 23:59:59';

ALTER TABLE products ADD COLUMN currency TEXT NOT NULL DEFAULT '';
ALTER TABLE products ADD COLUMN enabled INTEGER NOT NULL DEFAULT 1;
ALTER TABLE products ADD COLUMN creem_product_id TEXT NOT NULL DEFAULT '';
ALTER TABLE products ADD COLUMN billing_type TEXT NOT NULL DEFAULT 'one_time';
ALTER TABLE products ADD COLUMN membership_level TEXT NOT NULL DEFAULT 'basic';
ALTER TABLE products ADD COLUMN subscription_interval TEXT NOT NULL DEFAULT '';
ALTER TABLE products ADD COLUMN updated_at DATETIME;

UPDATE products
SET updated_at = COALESCE(created_at, CURRENT_TIMESTAMP)
WHERE updated_at IS NULL;

ALTER TABLE orders ADD COLUMN product_id TEXT NOT NULL DEFAULT '';
ALTER TABLE orders ADD COLUMN provider_checkout_id TEXT NOT NULL DEFAULT '';
ALTER TABLE orders ADD COLUMN provider_order_id TEXT NOT NULL DEFAULT '';
ALTER TABLE orders ADD COLUMN provider_customer_id TEXT NOT NULL DEFAULT '';
ALTER TABLE orders ADD COLUMN provider_subscription_id TEXT NOT NULL DEFAULT '';
ALTER TABLE orders ADD COLUMN provider_product_id TEXT NOT NULL DEFAULT '';
ALTER TABLE orders ADD COLUMN subscription_status TEXT NOT NULL DEFAULT '';
ALTER TABLE orders ADD COLUMN membership_applied_at DATETIME NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_orders_product_id ON orders(product_id);
CREATE INDEX IF NOT EXISTS idx_orders_provider_subscription_id ON orders(provider_subscription_id);
CREATE INDEX IF NOT EXISTS idx_products_creem_product_id ON products(creem_product_id);
