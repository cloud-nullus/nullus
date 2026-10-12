DROP INDEX IF EXISTS idx_backup_runs_set_id;
ALTER TABLE backup_runs DROP COLUMN IF EXISTS set_id;
