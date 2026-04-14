-- Migration: 003_alter_users_table.sql
-- Description: Update users table structure to support Google OAuth and adjust constraints

-- =============================================
-- USERS TABLE CHANGES
-- =============================================

-- Step 1: Drop existing indexes that will be recreated
DROP INDEX IF EXISTS idx_users_subscription_tier_created_at;

-- Step 2: Remove columns that are no longer needed
ALTER TABLE users DROP COLUMN IF EXISTS company;
ALTER TABLE users DROP COLUMN IF EXISTS bio;

-- Step 3: Make name NOT NULL (currently nullable)
ALTER TABLE users ALTER COLUMN name SET NOT NULL;

-- Step 4: Make password_hash nullable (for Google OAuth users who don't have password)
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;

-- Step 5: Add new columns for Google OAuth
ALTER TABLE users ADD COLUMN IF NOT EXISTS google_id VARCHAR(255) UNIQUE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS google_profile_data JSONB;

-- Step 6: Add new indexes
CREATE INDEX IF NOT EXISTS idx_users_google_id ON users(google_id);

-- Step 7: Recreate the composite index
CREATE INDEX IF NOT EXISTS idx_users_subscription_tier_created_at ON users(subscription_tier, created_at);

-- Step 8: Add comment for documentation
COMMENT ON COLUMN users.google_id IS 'Google OAuth user ID for SSO authentication';
COMMENT ON COLUMN users.google_profile_data IS 'Cached Google profile data for OAuth users';

-- =============================================
-- SESSIONS TABLE (already exists, no changes needed)
-- =============================================

-- The sessions table already matches the desired schema:
-- CREATE TABLE IF NOT EXISTS sessions (
--   id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
--   user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
--   token_hash VARCHAR(255) UNIQUE NOT NULL,
--   expires_at TIMESTAMP NOT NULL,
--   created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
--   last_activity_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
--   ip_address VARCHAR(45),
--   user_agent TEXT
-- );

-- =============================================
-- EMAIL VERIFICATION TOKENS TABLE
-- =============================================

-- Create email_verification_tokens table if it doesn't exist
CREATE TABLE IF NOT EXISTS email_verification_tokens (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token VARCHAR(255) UNIQUE NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  used_at TIMESTAMP
);

-- Create indexes for email verification tokens
CREATE INDEX IF NOT EXISTS idx_email_verification_tokens_user_id ON email_verification_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_email_verification_tokens_token ON email_verification_tokens(token);
