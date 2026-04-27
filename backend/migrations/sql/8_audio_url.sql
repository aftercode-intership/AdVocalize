-- backend/migrations/sql/8_audio_url.sql
-- Sprint 5 follow-up: store the completed audio URL on the generated ad row
-- so the dashboard can show script + audio together without querying Redis.

-- Add audio fields to generated_ads
ALTER TABLE generated_ads
  ADD COLUMN IF NOT EXISTS audio_url         TEXT,
  ADD COLUMN IF NOT EXISTS audio_duration_s  FLOAT,
  ADD COLUMN IF NOT EXISTS audio_job_id      VARCHAR(36),
  ADD COLUMN IF NOT EXISTS audio_voice_id    VARCHAR(100),
  ADD COLUMN IF NOT EXISTS audio_generated_at TIMESTAMP;

CREATE INDEX IF NOT EXISTS idx_gen_ads_audio_job_id
  ON generated_ads(audio_job_id)
  WHERE audio_job_id IS NOT NULL;

-- chat_message_versions table (needed by MessageEditService)
CREATE TABLE IF NOT EXISTS chat_message_versions (
  id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  message_id  UUID NOT NULL,
  content     TEXT NOT NULL,
  version     INT  NOT NULL DEFAULT 1,
  created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_cmv_message_id ON chat_message_versions(message_id);

-- credit_transactions table (needed by CreditService)
CREATE TABLE IF NOT EXISTS credit_transactions (
  id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  amount        INT NOT NULL,             -- positive = add, negative = deduct
  reason        VARCHAR(100) NOT NULL,    -- SCRIPT_GEN, TTS_GEN, MONTHLY_RESET, etc.
  campaign_id   UUID REFERENCES campaigns(id) ON DELETE SET NULL,
  balance_after INT NOT NULL,
  created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_credit_tx_user_id   ON credit_transactions(user_id);
CREATE INDEX IF NOT EXISTS idx_credit_tx_created_at ON credit_transactions(created_at DESC);

-- sessions table (needed by AuthService.Login)
CREATE TABLE IF NOT EXISTS sessions (
  id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash       VARCHAR(64) UNIQUE NOT NULL,
  expires_at       TIMESTAMP NOT NULL,
  created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  last_activity_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  ip_address       INET,
  user_agent       TEXT
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id    ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_token_hash ON sessions(token_hash);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);