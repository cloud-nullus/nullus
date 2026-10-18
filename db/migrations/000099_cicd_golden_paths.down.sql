-- 사용자가 추가한 골든패스도 함께 사라진다 — 메모리에 있던 시절로 되돌리는 것이므로
-- 종전 세 건은 코드가 다시 공급하지만, 그 뒤에 만든 것은 돌려줄 자리가 없다.
DROP INDEX IF EXISTS idx_cicd_golden_paths_order;
DROP TABLE IF EXISTS cicd_golden_paths;
