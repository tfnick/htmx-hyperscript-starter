-- 002_seed.sql: 种子数据（可选，用于测试）

-- 插入测试用户
INSERT OR IGNORE INTO users (id, name, email) VALUES
    ('u001', '张三', 'zhangsan@example.com'),
    ('u002', '李四', 'lisi@example.com'),
    ('u003', '王五', 'wangwu@example.com');

-- 插入测试产品
INSERT OR IGNORE INTO products (id, name, description, price, stock) VALUES
    ('p001', 'iPhone 15', '苹果手机', 699900, 100),
    ('p002', 'MacBook Pro', '苹果笔记本电脑', 1299900, 50),
    ('p003', 'AirPods Pro', '苹果无线耳机', 189900, 200),
    ('p004', 'iPad Air', '苹果平板电脑', 479900, 80);
