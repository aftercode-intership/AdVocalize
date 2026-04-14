-- migrations/004_campaigns_and_ads.sql

-- Campaigns table (if not already created in Sprint 1)
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
  status VARCHAR(50) DEFAULT 'draft',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_campaigns_user_id ON campaigns(user_id);
CREATE INDEX idx_campaigns_status ON campaigns(status);

-- Campaign products (draft data while user is filling form)
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

CREATE INDEX idx_campaign_products_campaign_id ON campaign_products(campaign_id);