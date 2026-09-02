-- 000076_backup_stack_id_text.down.sql
--
-- 되돌리면 stk_* 형태의 값이 있는 행에서 실패한다. 그것이 맞다 —
-- 그 값들은 애초에 UUID 컬럼에 들어갈 수 없는 것이었다.
ALTER TABLE backup_runs ALTER COLUMN stack_id TYPE UUID USING stack_id::uuid;
