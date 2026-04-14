-- backend/migrations/sql/3_missing_tables.sql
-- Run this to add tables that were missing from the initial migration.
-- Safe to run multiple times (uses CREATE TABLE IF NOT EXISTS).

-- ─────────────────────────────────────────────────────────────────────────────
-- chat_sessions
-- ─────────────────────────────────────────────────────────────────────────────
-- The chat service's CreateSession, ListSessions, ArchiveSession all need this.
CREATE TABLE IF NOT EXISTS chat_sessions (
  id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  campaign_id UUID REFERENCES campaigns(id) ON DELETE SET NULL,
  ad_id       UUID REFERENCES generated_ads(id) ON DELETE SET NULL,
  topic       VARCHAR(50) NOT NULL DEFAULT 'general',
  status      VARCHAR(50) DEFAULT 'active',
  created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_chat_sessions_user_id   ON chat_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_chat_sessions_status     ON chat_sessions(status);
CREATE INDEX IF NOT EXISTS idx_chat_sessions_updated_at ON chat_sessions(updated_at DESC);

-- ─────────────────────────────────────────────────────────────────────────────
-- email_verification_tokens
-- ─────────────────────────────────────────────────────────────────────────────
-- Used by VerifyEmail handler. Without this the verify-email page errors.
CREATE TABLE IF NOT EXISTS email_verification_tokens (
  id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token      VARCHAR(255) UNIQUE NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  used_at    TIMESTAMP,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_evt_user_id    ON email_verification_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_evt_token      ON email_verification_tokens(token);
CREATE INDEX IF NOT EXISTS idx_evt_expires_at ON email_verification_tokens(expires_at);

-- ─────────────────────────────────────────────────────────────────────────────
-- password_reset_tokens
-- ─────────────────────────────────────────────────────────────────────────────
-- Used by ForgotPassword / ResetPassword handlers.
CREATE TABLE IF NOT EXISTS password_reset_tokens (
  id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash VARCHAR(64) UNIQUE NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  used_at    TIMESTAMP,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_prt_token_hash ON password_reset_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_prt_user_id    ON password_reset_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_prt_expires_at ON password_reset_tokens(expires_at);

-- ─────────────────────────────────────────────────────────────────────────────
-- support_escalations
-- ─────────────────────────────────────────────────────────────────────────────
-- Used by the SupportBotService (CreateEscalation etc.)
CREATE TABLE IF NOT EXISTS support_escalations (
  id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  session_id  UUID REFERENCES chat_sessions(id) ON DELETE CASCADE,
  user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  reason      TEXT NOT NULL,
  status      VARCHAR(50) DEFAULT 'pending',
  assigned_to UUID REFERENCES users(id) ON DELETE SET NULL,
  resolution  TEXT,
  created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  resolved_at TIMESTAMP,
  updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_support_esc_user_id ON support_escalations(user_id);
CREATE INDEX IF NOT EXISTS idx_support_esc_status  ON support_escalations(status);

-- ─────────────────────────────────────────────────────────────────────────────
-- google_id column on users (needed for Google OAuth)
-- ─────────────────────────────────────────────────────────────────────────────
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'users' AND column_name = 'google_id'
  ) THEN
    ALTER TABLE users ADD COLUMN google_id VARCHAR(255) UNIQUE;
    ALTER TABLE users ADD COLUMN google_profile_data JSONB;
    CREATE INDEX idx_users_google_id ON users(google_id);
  END IF;
END $$;
