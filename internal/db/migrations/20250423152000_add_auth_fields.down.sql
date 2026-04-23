ALTER TABLE users DROP COLUMN IF EXISTS failed_attempts;
ALTER TABLE users DROP COLUMN IF EXISTS lockout_until;

ALTER TABLE email_verifications DROP COLUMN IF EXISTS user_id;

DROP INDEX IF EXISTS idx_email_verifications_user_id;