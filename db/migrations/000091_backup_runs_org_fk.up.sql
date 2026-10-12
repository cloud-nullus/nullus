-- backup_runs.org_id 가 organizations 를 참조하게 한다.
--
-- FK 가 없어 어떤 조직 ID 로도 백업이 만들어졌다. 목록은 로그인 사용자의 조직으로만
-- 거르므로, 없는 조직으로 만든 백업은 어떤 화면에도 보이지 않았다 — 로컬 테스트
-- 가이드가 적어 둔 유령 조직(00000000-0000-0000-0000-000000000001)으로 실제로 그랬다.
--
-- 이미 들어간 고아 행은 시드 기본 조직(000017)으로 옮긴다. 시드 조직이 없는 환경이면
-- 가장 오래된 조직으로. 조직이 하나도 없는데 고아 행이 있으면 NOT NULL 에 걸려 여기서
-- 멈춘다 — 그런 환경은 사람이 봐야 한다.
UPDATE backup_runs
SET org_id = COALESCE(
    (SELECT id FROM organizations WHERE id = '11111111-1111-1111-1111-111111111111'),
    (SELECT id FROM organizations ORDER BY created_at ASC LIMIT 1)
)
WHERE org_id NOT IN (SELECT id FROM organizations);

ALTER TABLE backup_runs
    ADD CONSTRAINT backup_runs_org_id_fkey
    FOREIGN KEY (org_id) REFERENCES organizations(id);
