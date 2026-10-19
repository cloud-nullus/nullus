DROP TABLE IF EXISTS stack_cve_notices;
ALTER TABLE stack_image_scans
    DROP COLUMN IF EXISTS exposed_hosts,
    DROP COLUMN IF EXISTS exposed;
