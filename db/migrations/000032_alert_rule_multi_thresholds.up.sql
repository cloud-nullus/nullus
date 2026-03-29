ALTER TABLE alert_rules
  ADD COLUMN IF NOT EXISTS warning_threshold DOUBLE PRECISION,
  ADD COLUMN IF NOT EXISTS critical_threshold DOUBLE PRECISION;

UPDATE alert_rules
SET
  warning_threshold = COALESCE(warning_threshold, threshold),
  critical_threshold = COALESCE(critical_threshold, threshold);

ALTER TABLE alert_rules
  ALTER COLUMN warning_threshold SET NOT NULL,
  ALTER COLUMN critical_threshold SET NOT NULL;
