-- 지운 데모 이력은 되돌리지 않는다(데모 데이터다).
DROP INDEX IF EXISTS idx_stack_alerts_org_started;
DROP INDEX IF EXISTS uq_stack_alerts_open;
DROP TABLE IF EXISTS stack_alerts;
