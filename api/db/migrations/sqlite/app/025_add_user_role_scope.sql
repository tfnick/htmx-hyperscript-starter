ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'user';
ALTER TABLE users ADD COLUMN organization_id TEXT NOT NULL DEFAULT '';

UPDATE users
SET role = 'platform_admin'
WHERE is_admin = 1 AND (role = '' OR role = 'user');

UPDATE users
SET role = 'user'
WHERE is_admin != 1 AND role = '';

CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
CREATE INDEX IF NOT EXISTS idx_users_organization ON users(organization_id);
