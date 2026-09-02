-- 000075_backup_restore.down.sql
DROP INDEX IF EXISTS idx_restore_runs_backup;
DROP TABLE IF EXISTS restore_runs;
DROP INDEX IF EXISTS idx_backup_artifacts_run;
DROP TABLE IF EXISTS backup_artifacts;
DROP INDEX IF EXISTS idx_backup_runs_status_created;
DROP INDEX IF EXISTS idx_backup_runs_org_created;
DROP TABLE IF EXISTS backup_runs;
