ALTER TABLE stacks
    DROP COLUMN IF EXISTS last_failure_reason,
    DROP COLUMN IF EXISTS last_failed_step,
    DROP COLUMN IF EXISTS last_completed_step,
    DROP COLUMN IF EXISTS current_step;
