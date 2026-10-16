-- 되돌리면 FK 가 다시 생긴다 — 그러면 full 복구의 플랫폼 DB 단계가 다시 막힌다(up 주석 참조).
-- 고아 행이 있으면 여기서 멈춘다. 그런 환경은 사람이 봐야 한다.
ALTER TABLE backup_runs
    ADD CONSTRAINT backup_runs_org_id_fkey
    FOREIGN KEY (org_id) REFERENCES organizations(id);
