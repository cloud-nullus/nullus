DROP TABLE IF EXISTS stack_image_vulnerabilities;
ALTER TABLE stack_image_scans DROP COLUMN IF EXISTS vulnerabilities_recorded;
