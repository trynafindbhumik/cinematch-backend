-- Add login lockout fields to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS failed_attempts int DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS lockout_until timestamptz;

-- Add user_id to email_verifications
ALTER TABLE email_verifications 
ADD COLUMN IF NOT EXISTS user_id bigint REFERENCES users(id) ON DELETE CASCADE;

-- Create index
CREATE INDEX IF NOT EXISTS idx_email_verifications_user_id 
ON email_verifications(user_id) 
WHERE user_id IS NOT NULL;

COMMENT ON COLUMN users.failed_attempts IS 'Number of consecutive failed login attempts';
COMMENT ON COLUMN users.lockout_until IS 'Timestamp until which the account is locked out';
COMMENT ON COLUMN email_verifications.user_id IS 'Reference to user (nullable for signup verification)';