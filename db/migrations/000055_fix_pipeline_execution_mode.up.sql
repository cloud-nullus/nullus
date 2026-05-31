ALTER TABLE pipelines
  ADD COLUMN IF NOT EXISTS execution_mode VARCHAR(50) NOT NULL DEFAULT 'emergency_direct';
