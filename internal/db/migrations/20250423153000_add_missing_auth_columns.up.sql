-- Add is_verified column to users table (MISSING!)
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_verified bool DEFAULT false NOT NULL;

-- Add name column to users table (MISSING!)
-- Note: We create users without name currently
ALTER TABLE users ADD COLUMN IF NOT EXISTS name text DEFAULT '' NOT NULL;

COMMENT ON COLUMN users.is_verified IS 'Whether the user has verified their email';
COMMENT ON COLUMN users.name IS 'User display name';