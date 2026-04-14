-- migrations/001_initial_schema.sql

-- Enable extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "citext";  -- Case-insensitive text

-- Users table
CREATE TABLE IF NOT EXISTS users (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  email CITEXT UNIQUE NOT NULL,
  email_verified BOOLEAN DEFAULT FALSE,
  email_verified_at TIMESTAMP,
  password_hash VARCHAR(255) NOT NULL,
  name VARCHAR(255),
  company VARCHAR(255),
  bio TEXT,
  avatar_url VARCHAR(255),
  language VARCHAR(10) DEFAULT 'en',  -- en, fr, ar
  subscription_tier VARCHAR(50) DEFAULT 'FREE',  -- FREE, PRO
  credits_remaining INTEGER DEFAULT 50,  -- -1 = unlimited
  account_status VARCHAR(50) DEFAULT 'active',  -- active, suspended, deleted
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMP
);

-- Create indexes on users
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_created_at ON users(created_at);
CREATE INDEX idx_users_subscription_tier ON users(subscription_tier);

-- Sessions table
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

-- Create indexes on sessions
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_token_hash ON sessions(token_hash);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

-- Credit transactions table
CREATE TABLE IF NOT EXISTS credit_transactions (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  amount INTEGER NOT NULL,  -- Can be negative for deductions
  reason VARCHAR(100) NOT NULL,  -- SCRIPT_GEN, AUDIO_SYNTHESIS, VIDEO_GEN, VOICE_CLONE, etc.
  campaign_id UUID,  -- Can be null for standalone generations
  balance_after INTEGER NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes on credit transactions
CREATE INDEX idx_credit_transactions_user_id ON credit_transactions(user_id);
CREATE INDEX idx_credit_transactions_created_at ON credit_transactions(created_at);
CREATE INDEX idx_credit_transactions_campaign_id ON credit_transactions(campaign_id);

-- Audit log table
CREATE TABLE IF NOT EXISTS audit_logs (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  action VARCHAR(100) NOT NULL,
  entity_type VARCHAR(100),  -- user, campaign, ad, voice_clone, etc.
  entity_id UUID,
  old_value JSONB,
  new_value JSONB,
  ip_address VARCHAR(45),
  user_agent TEXT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes on audit logs
CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);

-- Campaigns table
CREATE TABLE IF NOT EXISTS campaigns (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name VARCHAR(255) NOT NULL,
  brand VARCHAR(255) NOT NULL,
  objective VARCHAR(50),  -- AWARENESS, CONSIDERATION, CONVERSION, RETARGETING
  description TEXT,
  target_markets TEXT[],  -- Array of country codes
  channels TEXT[],  -- Array of channels: YouTube, Instagram, TikTok, Spotify, Programmatic
  budget DECIMAL(10,2),
  status VARCHAR(50) DEFAULT 'planning',  -- planning, in_progress, paused, completed
  start_date DATE,
  end_date DATE,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes on campaigns
CREATE INDEX idx_campaigns_user_id ON campaigns(user_id);
CREATE INDEX idx_campaigns_status ON campaigns(status);
CREATE INDEX idx_campaigns_created_at ON campaigns(created_at);

-- Generated ads table
CREATE TABLE IF NOT EXISTS generated_ads (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  campaign_id UUID REFERENCES campaigns(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  product_name VARCHAR(255) NOT NULL,
  product_description TEXT,
  target_audience VARCHAR(255),
  tone VARCHAR(50),  -- FORMAL, CASUAL, PODCAST
  duration INTEGER,  -- seconds
  language VARCHAR(10),  -- en, fr, ar
  marketing_channel VARCHAR(50),  -- YouTube, Instagram, Spotify, etc.
  script_text TEXT,
  script_text_arabic_alternative TEXT,  -- For fallback
  word_count INTEGER,
  estimated_duration_seconds INTEGER,
  audio_url VARCHAR(255),
  audio_duration_seconds DECIMAL(10,1),
  audio_format VARCHAR(10),  -- mp3, wav
  video_url VARCHAR(255),
  video_format VARCHAR(10),
  vast_metadata_url VARCHAR(255),
  status VARCHAR(50) DEFAULT 'draft',  -- draft, generating, completed, failed
  watermark_id VARCHAR(255),
  watermark_present BOOLEAN DEFAULT FALSE,
  disclaimer_present BOOLEAN DEFAULT FALSE,
  provenance_certificate JSONB,
  performance_metrics JSONB,  -- impressions, clicks, cpc after publishing
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes on generated ads
CREATE INDEX idx_generated_ads_campaign_id ON generated_ads(campaign_id);
CREATE INDEX idx_generated_ads_user_id ON generated_ads(user_id);
CREATE INDEX idx_generated_ads_language ON generated_ads(language);
CREATE INDEX idx_generated_ads_status ON generated_ads(status);
CREATE INDEX idx_generated_ads_created_at ON generated_ads(created_at);

-- Voice clones table
CREATE TABLE IF NOT EXISTS voice_clones (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  voice_name VARCHAR(255) NOT NULL,
  reference_audio_url VARCHAR(255) NOT NULL,
  voice_consent_affirmed BOOLEAN NOT NULL DEFAULT FALSE,
  voice_consent_timestamp TIMESTAMP,
  voice_consent_ip_address VARCHAR(45),
  voice_quality_verified BOOLEAN DEFAULT FALSE,
  snr_score DECIMAL(5,1),  -- Signal-to-Noise Ratio
  voice_embedding_vector BYTEA,  -- 512-dimensional vector
  voice_similarity_score DECIMAL(3,2),  -- PESQ score (1-4.5)
  usage_count INTEGER DEFAULT 0,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMP
);

-- Create indexes on voice clones
CREATE INDEX idx_voice_clones_user_id ON voice_clones(user_id);
CREATE INDEX idx_voice_clones_voice_consent_affirmed ON voice_clones(voice_consent_affirmed);
CREATE INDEX idx_voice_clones_created_at ON voice_clones(created_at);

-- Subscriptions table
CREATE TABLE IF NOT EXISTS subscriptions (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  stripe_subscription_id VARCHAR(255) UNIQUE,
  stripe_customer_id VARCHAR(255),
  tier VARCHAR(50) NOT NULL,  -- FREE, PRO
  status VARCHAR(50) DEFAULT 'active',  -- active, trial, cancelled, failed, suspended
  current_period_start DATE NOT NULL,
  current_period_end DATE NOT NULL,
  next_billing_date DATE,
  auto_renew BOOLEAN DEFAULT TRUE,
  monthly_price DECIMAL(10,2),
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  cancelled_at TIMESTAMP
);

-- Create indexes on subscriptions
CREATE INDEX idx_subscriptions_user_id ON subscriptions(user_id);
CREATE INDEX idx_subscriptions_stripe_subscription_id ON subscriptions(stripe_subscription_id);
CREATE INDEX idx_subscriptions_status ON subscriptions(status);

-- Chat messages table
CREATE TABLE IF NOT EXISTS chat_messages (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  session_id UUID NOT NULL,
  campaign_id UUID REFERENCES campaigns(id) ON DELETE SET NULL,
  ad_id UUID REFERENCES generated_ads(id) ON DELETE SET NULL,
  content TEXT NOT NULL,
  role VARCHAR(20) NOT NULL,  -- USER, ASSISTANT
  is_edited BOOLEAN DEFAULT FALSE,
  version INTEGER DEFAULT 1,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes on chat messages
CREATE INDEX idx_chat_messages_user_id ON chat_messages(user_id);
CREATE INDEX idx_chat_messages_session_id ON chat_messages(session_id);
CREATE INDEX idx_chat_messages_campaign_id ON chat_messages(campaign_id);
CREATE INDEX idx_chat_messages_created_at ON chat_messages(created_at);

-- Invoices table
CREATE TABLE IF NOT EXISTS invoices (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  stripe_invoice_id VARCHAR(255) UNIQUE,
  invoice_number VARCHAR(50) UNIQUE,
  amount DECIMAL(10,2) NOT NULL,
  currency VARCHAR(3) DEFAULT 'EUR',
  status VARCHAR(50) DEFAULT 'draft',  -- draft, issued, paid, failed, refunded
  pdf_url VARCHAR(255),
  payment_date TIMESTAMP,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  due_date TIMESTAMP
);

-- Create indexes on invoices
CREATE INDEX idx_invoices_user_id ON invoices(user_id);
CREATE INDEX idx_invoices_status ON invoices(status);
CREATE INDEX idx_invoices_created_at ON invoices(created_at);

-- Create function to update 'updated_at' timestamps
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = CURRENT_TIMESTAMP;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create triggers for updated_at columns
CREATE TRIGGER users_updated_at BEFORE UPDATE ON users
  FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER campaigns_updated_at BEFORE UPDATE ON campaigns
  FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER generated_ads_updated_at BEFORE UPDATE ON generated_ads
  FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER subscriptions_updated_at BEFORE UPDATE ON subscriptions
  FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER chat_messages_updated_at BEFORE UPDATE ON chat_messages
  FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Email verification tokens table
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

-- Create indexes for performance optimization
CREATE INDEX idx_users_subscription_tier_created_at ON users(subscription_tier, created_at);
CREATE INDEX idx_generated_ads_user_campaign ON generated_ads(user_id, campaign_id);
CREATE INDEX idx_voice_clones_user_created ON voice_clones(user_id, created_at);