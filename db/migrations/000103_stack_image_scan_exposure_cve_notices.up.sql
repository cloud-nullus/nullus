-- 설치 이미지 스캔 결과에 외부 노출을 남긴다(nullus-plan#78).
-- 같은 CVE 라도 게이트웨이(HTTPRoute)로 클러스터 밖에 열린 서비스의 것을 먼저 고친다.
ALTER TABLE stack_image_scans
    ADD COLUMN IF NOT EXISTS exposed       BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS exposed_hosts JSONB   NOT NULL DEFAULT '[]'::jsonb;

-- 스택에 알린 CVE. 새 CVE 고지는 이것으로 새것을 가린다 — 지난 스캔 결과로 가리면 이미지 하나의 스캔이
-- 실패해 그 이미지의 취약점 행이 사라졌을 때 다음 스캔이 같은 CVE 를 다시 알린다. 보내기 전에 행을
-- 선점해(기본 키) API 레플리카 둘이 한 번만 보낸다. first_notified_at 은 대응 기한(정책 §4)의 기준 시각이다.
CREATE TABLE IF NOT EXISTS stack_cve_notices (
    stack_id          VARCHAR(100) NOT NULL REFERENCES stacks(id) ON DELETE CASCADE,
    vulnerability_id  TEXT         NOT NULL,
    first_notified_at TIMESTAMPTZ  NOT NULL,
    PRIMARY KEY (stack_id, vulnerability_id)
);
