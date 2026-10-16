-- 백업 세트 — 플랫폼 + 조직의 모든 스택을 한 번에 뜬 실행들을 묶는 식별자(백업복구 설계 §6.7).
--
-- 한 실행(backup_run)은 플랫폼 하나 또는 스택 하나만 담는다 — 어댑터가 스택 클러스터마다 다르고
-- 금고가 스택마다 있어 한 실행으로 여러 스택을 뜰 수 없다. "플랫폼 전체" 는 그래서 실행의 묶음이고,
-- 묶음을 복구할 때 플랫폼 → 스택 순서로 되돌릴 수 있어야 하니 어느 실행이 한 세트인지 남긴다.
--
-- NULL 은 세트에 속하지 않는 단독 실행이다(기존 행 전부). FK 는 두지 않는다 — 세트는 테이블이 아니라
-- 실행들의 공통 꼬리표이고, 백업 이력 테이블은 어디도 참조하지 않는다는 000092 의 원칙을 따른다.
ALTER TABLE backup_runs ADD COLUMN IF NOT EXISTS set_id UUID;

CREATE INDEX IF NOT EXISTS idx_backup_runs_set_id ON backup_runs (set_id) WHERE set_id IS NOT NULL;
