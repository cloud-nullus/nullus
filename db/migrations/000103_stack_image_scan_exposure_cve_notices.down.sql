DROP TABLE IF EXISTS stack_cve_notices;
DROP INDEX IF EXISTS idx_stack_image_vulnerabilities_stack_severity;
ALTER TABLE stack_image_scans
    DROP COLUMN IF EXISTS exposed_hosts,
    DROP COLUMN IF EXISTS exposed;
