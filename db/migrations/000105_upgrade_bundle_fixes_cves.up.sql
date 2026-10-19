-- 업그레이드 번들이 고치는 CVE(nullus-plan#78 ⑤). 번들을 리허설해 verified 로 올릴 때 함께 적는다 —
-- 업그레이드 후보는 그중 스택의 설치 이미지 스캔에서 지금 발견된 것을 보인다.
ALTER TABLE upgrade_bundles
    ADD COLUMN IF NOT EXISTS fixes_cves JSONB NOT NULL DEFAULT '[]';
