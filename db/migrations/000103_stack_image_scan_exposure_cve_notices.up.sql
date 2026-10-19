-- 설치 이미지 스캔 결과에 외부 노출을 남긴다(nullus-plan#78).
-- 같은 CVE 라도 게이트웨이(HTTPRoute)로 클러스터 밖에 열린 서비스의 것을 먼저 고친다.
ALTER TABLE stack_image_scans
    ADD COLUMN IF NOT EXISTS exposed       BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS exposed_hosts JSONB   NOT NULL DEFAULT '[]'::jsonb;

-- 새 CVE 고지가 지난 스캔의 "수정판 있는 CRITICAL" 을 스택 단위로 읽는다.
CREATE INDEX IF NOT EXISTS idx_stack_image_vulnerabilities_stack_severity
    ON stack_image_vulnerabilities (stack_id, severity);

-- 스택에 한 번 알린 CVE. 이미지 하나의 스캔이 실패하면 그 이미지의 취약점 행이 사라지므로, 지난 스캔
-- 결과만으로는 "이미 알린 것" 을 알 수 없다 — 다음 스캔이 같은 CVE 를 새로 발견한 것처럼 다시 알린다.
-- first_notified_at 은 대응 기한(정책 §4)의 기준 시각이다.
CREATE TABLE IF NOT EXISTS stack_cve_notices (
    stack_id          VARCHAR(100) NOT NULL REFERENCES stacks(id) ON DELETE CASCADE,
    vulnerability_id  TEXT         NOT NULL,
    first_notified_at TIMESTAMPTZ  NOT NULL,
    PRIMARY KEY (stack_id, vulnerability_id)
);
