ALTER TABLE orders ADD COLUMN subscription_canceled_at DATETIME NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_orders_subscription_canceled_at
ON orders(subscription_canceled_at);
