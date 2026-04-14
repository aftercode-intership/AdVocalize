-- migrations/sql/003_support_tables.sql

CREATE TABLE IF NOT EXISTS support_escalations (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  session_id UUID NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  reason TEXT NOT NULL,
  status VARCHAR(50) DEFAULT 'pending',  -- pending, assigned, in_progress, resolved
  assigned_to UUID REFERENCES users(id) ON DELETE SET NULL,
  resolution TEXT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  resolved_at TIMESTAMP
);

CREATE INDEX idx_support_escalations_user_id ON support_escalations(user_id);
CREATE INDEX idx_support_escalations_status ON support_escalations(status);
CREATE INDEX idx_support_escalations_created_at ON support_escalations(created_at);
CREATE INDEX idx_support_escalations_assigned_to ON support_escalations(assigned_to);