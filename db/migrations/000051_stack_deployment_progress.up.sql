ALTER TABLE stacks
    ADD COLUMN IF NOT EXISTS current_step VARCHAR(100),
    ADD COLUMN IF NOT EXISTS last_completed_step VARCHAR(100),
    ADD COLUMN IF NOT EXISTS last_failed_step VARCHAR(100),
    ADD COLUMN IF NOT EXISTS last_failure_reason TEXT;
