-- 설치 이미지 스캔 결과에 외부 노출을 남긴다(nullus-plan#78).
-- 같은 CVE 라도 게이트웨이(HTTPRoute)로 클러스터 밖에 열린 서비스의 것을 먼저 고친다.
ALTER TABLE stack_image_scans
    ADD COLUMN IF NOT EXISTS exposed       BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS exposed_hosts JSONB   NOT NULL DEFAULT '[]'::jsonb;

-- 새 CVE 고지가 지난 스캔의 "수정판 있는 CRITICAL" 을 스택 단위로 읽는다.
CREATE INDEX IF NOT EXISTS idx_stack_image_vulnerabilities_stack_severity
    ON stack_image_vulnerabilities (stack_id, severity);
