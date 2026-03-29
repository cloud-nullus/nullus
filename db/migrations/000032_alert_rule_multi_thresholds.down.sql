ALTER TABLE alert_rules
  DROP COLUMN IF EXISTS critical_threshold,
  DROP COLUMN IF EXISTS warning_threshold;
