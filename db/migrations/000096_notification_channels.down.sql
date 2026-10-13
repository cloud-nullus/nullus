DROP INDEX IF EXISTS idx_notification_history_org_created;
ALTER TABLE notification_history
    DROP COLUMN IF EXISTS attempts,
    DROP COLUMN IF EXISTS channel_id;

DROP INDEX IF EXISTS idx_notification_configs_org;
-- 암호화한 비밀은 되돌릴 자리가 없다 — 내리면 채널을 다시 설정해야 한다.
ALTER TABLE notification_configs
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS secret_encrypted,
    DROP COLUMN IF EXISTS name;
