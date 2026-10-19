ALTER TABLE stack_image_scans
    DROP COLUMN IF EXISTS shared,
    DROP COLUMN IF EXISTS namespace;
