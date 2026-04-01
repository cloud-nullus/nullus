CREATE TABLE IF NOT EXISTS deployment_logs (
  id BIGSERIAL PRIMARY KEY,
  deployment_id VARCHAR(64) NOT NULL,
  timestamp TIMESTAMPTZ NOT NULL,
  level VARCHAR(16) NOT NULL,
  step VARCHAR(128) NOT NULL,
  phase VARCHAR(16) NOT NULL DEFAULT '',
  message TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_deployment_logs_deployment_time
  ON deployment_logs (deployment_id, timestamp, id);
