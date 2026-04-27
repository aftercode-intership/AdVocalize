-- Consolidated all migrations into one idempotent script
-- Run: docker-compose exec postgres psql -U vocalize -d vocalize_db -f backend/migrations/sql/0-consolidated-migrations.sql (from host? Use docker cp)

-- 1. Extensions
CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\";
CREATE EXTENSION IF NOT EXISTS \"citext\";

-- 2. Users table (core)
CREATE TABLE IF NOT EXISTS users (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  email CITEXT UNIQUE NOT NULL,
  email_verified BOOLEAN DEFAULT FALSE,
  email_verified_at TIMESTAMP,
  password_hash VARCHAR(255),
  name VARCHAR(255) NOT NULL,
  avatar_url VARCHAR(255),
  language VARCHAR(10) DEFAULT 'en',
  subscription_tier VARCHAR(50) DEFAULT 'FREE',
  credits_remaining INTEGER DEFAULT 50,
  account_status VARCHAR(50) DEFAULT 'active',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMP
);

-- Add google columns if missing
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'users' AND column_name = 'google_id') THEN
    ALTER TABLE users ADD COLUMN google_id VARCHAR(255) UNIQUE;
    ALTER TABLE users ADD COLUMN google_profile_data JSONB;
  END IF;
END $$;

-- 3. Sessions
CREATE TABLE IF NOT EXISTS sessions (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash VARCHAR(255) UNIQUE NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  last_activity_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  ip_address VARCHAR(45),
  user_agent TEXT
);

-- 4. Credits, audit, campaigns
CREATE TABLE IF NOT EXISTS credit_transactions (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  amount INTEGER NOT NULL,
  reason VARCHAR(100) NOT NULL,
  campaign_id UUID,
  balance_after INTEGER NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS audit_logs (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  action VARCHAR(100) NOT NULL,
  entity_type VARCHAR(100),
  entity_id UUID,
  old_value JSONB,
  new_value JSONB,
  ip_address VARCHAR(45),
  user_agent TEXT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS campaigns (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name VARCHAR(255) NOT NULL,
  brand VARCHAR(255) NOT NULL,
  objective VARCHAR(50),
  description TEXT,
  target_markets TEXT[],
  channels TEXT[],
  budget DECIMAL(10,2),
  status VARCHAR(50) DEFAULT 'planning',
  start_date DATE,
  end_date DATE,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 5. Generated ads (full version)
CREATE TABLE IF NOT EXISTS generated_ads (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  campaign_id UUID REFERENCES campaigns(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  product_name VARCHAR(255) NOT NULL,
  product_description TEXT,
  target_audience VARCHAR(255),
  tone VARCHAR(50),
  duration INTEGER,
  language VARCHAR(10),
  marketing_channel VARCHAR(50),
  script_text TEXT,
  script_text_arabic_alternative TEXT,
  word_count INTEGER,
  estimated_duration_seconds INTEGER,
  audio_url VARCHAR(255),
  audio_duration_seconds DECIMAL(10,1),
  audio_format VARCHAR(10),
  video_url VARCHAR(255),
  video_format VARCHAR(10),
  vast_metadata_url VARCHAR(255),
  status VARCHAR(50) DEFAULT 'draft',
  watermark_id VARCHAR(255),
  watermark_present BOOLEAN DEFAULT FALSE,
  disclaimer_present BOOLEAN DEFAULT FALSE,
  provenance_certificate JSONB,
  performance_metrics JSONB,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 6. Voice clones, subscriptions
CREATE TABLE IF NOT EXISTS voice_clones (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  voice_name VARCHAR(255) NOT NULL,
  reference_audio_url VARCHAR(255) NOT NULL,
  voice_consent_affirmed BOOLEAN NOT NULL DEFAULT FALSE,
  voice_consent_timestamp TIMESTAMP,
  voice_consent_ip_address VARCHAR(45),
  voice_quality_verified BOOLEAN DEFAULT FALSE,
  snr_score DECIMAL(5,1),
  voice_embedding_vector BYTEA,
  voice_similarity_score DECIMAL(3,2),
  usage_count INTEGER DEFAULT 0,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS subscriptions (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  stripe_subscription_id VARCHAR(255) UNIQUE,
  stripe_customer_id VARCHAR(255),
  tier VARCHAR(50) NOT NULL,
  status VARCHAR(50) DEFAULT 'active',
  current_period_start DATE NOT NULL,
  current_period_end DATE NOT NULL,
  next_billing_date DATE,
  auto_renew BOOLEAN DEFAULT TRUE,
  monthly_price DECIMAL(10,2),
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  cancelled_at TIMESTAMP
);

-- 7. Chat tables
CREATE TABLE IF NOT EXISTS chat_sessions (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  campaign_id UUID REFERENCES campaigns(id) ON DELETE SET NULL,
  ad_id UUID REFERENCES generated_ads(id) ON DELETE SET NULL,
  topic VARCHAR(50) NOT NULL DEFAULT 'general',
  status VARCHAR(50) DEFAULT 'active',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS chat_messages (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  session_id UUID NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
  user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  content TEXT NOT NULL,
  role VARCHAR(20) NOT NULL,
  is_edited BOOLEAN DEFAULT FALSE,
  version INTEGER DEFAULT 1,
  metadata JSONB,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS chat_message_versions (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  message_id UUID NOT NULL REFERENCES chat_messages(id) ON DELETE CASCADE,
  content TEXT NOT NULL,
  version INTEGER NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 8. Support, tokens, invoices, campaign_products
CREATE TABLE IF NOT EXISTS support_escalations (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  session_id UUID REFERENCES chat_sessions(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  reason TEXT NOT NULL,
  status VARCHAR(50) DEFAULT 'pending',
  assigned_to UUID REFERENCES users(id) ON DELETE SET NULL,
  resolution TEXT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  resolved_at TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS email_verification_tokens (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token VARCHAR(255) UNIQUE NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  used_at TIMESTAMP,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS password_reset_tokens (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash VARCHAR(64) UNIQUE NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  used_at TIMESTAMP,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS invoices (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  stripe_invoice_id VARCHAR(255) UNIQUE,
  invoice_number VARCHAR(50) UNIQUE,
  amount DECIMAL(10,2) NOT NULL,
  currency VARCHAR(3) DEFAULT 'EUR',
  status VARCHAR(50) DEFAULT 'draft',
  pdf_url VARCHAR(255),
  payment_date TIMESTAMP,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  due_date TIMESTAMP
);

CREATE TABLE IF NOT EXISTS campaign_products (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
  product_name VARCHAR(255) NOT NULL,
  product_description TEXT,
  target_audience VARCHAR(255),
  tone VARCHAR(50),
  language VARCHAR(10) DEFAULT 'en',
  marketing_channel VARCHAR(50),
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 9. Update function and triggers
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = CURRENT_TIMESTAMP;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Triggers for main tables (skip if exists or create always)
DROP TRIGGER IF EXISTS users_updated_at ON users;
CREATE TRIGGER users_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Similar for other tables (campaigns, generated_ads, subscriptions, chat_sessions, chat_messages, support_escalations)
DROP TRIGGER IF EXISTS campaigns_updated_at ON campaigns;
CREATE TRIGGER campaigns_updated_at BEFORE UPDATE ON campaigns FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS generated_ads_updated_at ON generated_ads;
CREATE TRIGGER generated_ads_updated_at BEFORE UPDATE ON generated_ads FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS subscriptions_updated_at ON subscriptions;
CREATE TRIGGER subscriptions_updated_at BEFORE UPDATE ON subscriptions FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS chat_sessions_updated_at ON chat_sessions;
CREATE TRIGGER chat_sessions_updated_at BEFORE UPDATE ON chat_sessions FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS chat_messages_updated_at ON chat_messages;
CREATE TRIGGER chat_messages_updated_at BEFORE UPDATE ON chat_messages FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS support_escalations_updated_at ON support_escalations;
CREATE TRIGGER support_escalations_updated_at BEFORE UPDATE ON support_escalations FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 10. All indexes (CONCURRENTLY where possible, IF NOT EXISTS)
-- Users indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_created_at ON users(created_at);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_subscription_tier ON users(subscription_tier);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_google_id ON users(google_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_subscription_tier_created_at ON users(subscription_tier, created_at);

-- Sessions
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sessions_token_hash ON sessions(token_hash);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

-- Continue with all other indexes from the files...
-- Credit transactions
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_credit_transactions_user_id ON credit_transactions(user_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_credit_transactions_created_at ON credit_transactions(created_at);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_credit_transactions_campaign_id ON credit_transactions(campaign_id);

-- Audit logs
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);

-- Campaigns
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_campaigns_user_id ON campaigns(user_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_campaigns_status ON campaigns(status);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_campaigns_created_at ON campaigns(created_at);

-- Generated ads (many)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_generated_ads_campaign_id ON generated_ads(campaign_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_generated_ads_user_id ON generated_ads(user_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_generated_ads_language ON generated_ads(language);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_generated_ads_status ON generated_ads(status);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_generated_ads_created_at ON generated_ads(created_at);

-- Voice clones
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_voice_clones_user_id ON voice_clones(user_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_voice_clones_voice_consent_affirmed ON voice_clones(voice_consent_affirmed);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_voice_clones_created_at ON voice_clones(created_at);

-- Subscriptions
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_subscriptions_user_id ON subscriptions(user_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_subscriptions_stripe_subscription_id ON subscriptions(stripe_subscription_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_subscriptions_status ON subscriptions(status);

-- Chat
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_chat_sessions_user_id ON chat_sessions(user_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_chat_sessions_status ON chat_sessions(status);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_chat_sessions_updated_at ON chat_sessions(updated_at DESC);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_chat_messages_session_id ON chat_messages(session_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_chat_messages_user_id ON chat_messages(user_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_chat_messages_role ON chat_messages(role);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_chat_messages_created_at ON chat_messages(created_at);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_chat_message_versions_message_id ON chat_message_versions(message_id);

-- Support
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_support_escalations_user_id ON support_escalations(user_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_support_escalations_status ON support_escalations(status);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_support_escalations_created_at ON support_escalations(created_at);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_support_escalations_assigned_to ON support_escalations(assigned_to);

-- Tokens
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_email_verification_tokens_user_id ON email_verification_tokens(user_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_email_verification_tokens_token ON email_verification_tokens(token);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_email_verification_tokens_expires_at ON email_verification_tokens(expires_at);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_prt_token_hash ON password_reset_tokens(token_hash);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_prt_user_id ON password_reset_tokens(user_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_prt_expires_at ON password_reset_tokens(expires_at);

-- Invoices, campaign_products
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_invoices_user_id ON invoices(user_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_invoices_status ON invoices(status);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_invoices_created_at ON invoices(created_at);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_campaign_products_campaign_id ON campaign_products(campaign_id);

-- Done! Run \dt to verify.

