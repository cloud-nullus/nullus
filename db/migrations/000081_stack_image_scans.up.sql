-- 000081_stack_image_scans.up.sql
-- 스택이 설치한 OSS 이미지의 취약점 스캔 결과 (보고용, 설치를 막지 않는다).
--
-- 파이프라인 스캔(image_scan_results)과 테이블을 나눈다. 그쪽은 pipelines 에
-- 매달린 CI 실행 기록이고, 이것은 stacks 모듈이 소유하는 설치 상태다 — 모듈은
-- 서로의 테이블을 읽지 않는다.
--
-- 결과는 digest 에 붙인다(태그는 움직인다). 다시 스캔하면 스택의 행을 통째로 바꿔
-- 더는 돌지 않는 이미지의 결과가 남지 않게 한다.

CREATE TABLE IF NOT EXISTS stack_image_scans (
    stack_id        VARCHAR(100) NOT NULL REFERENCES stacks(id) ON DELETE CASCADE,
    image_digest    VARCHAR(255) NOT NULL,
    image           VARCHAR(500) NOT NULL DEFAULT '',
    release_name    VARCHAR(255) NOT NULL DEFAULT '',
    workloads       JSONB        NOT NULL DEFAULT '[]'::jsonb,
    status          VARCHAR(20)  NOT NULL,
    error           TEXT         NOT NULL DEFAULT '',
    -- 스캔하지 못한 이미지는 NULL 이다. 0 으로 채우면 "취약점 0건" 으로 읽힌다.
    counts          JSONB,
    fixable_counts  JSONB,
    scanner         VARCHAR(50)  NOT NULL DEFAULT 'trivy',
    scanner_version VARCHAR(50)  NOT NULL DEFAULT '',
    db_updated_at   TIMESTAMPTZ,
    scanned_at      TIMESTAMPTZ  NOT NULL,
    PRIMARY KEY (stack_id, image_digest),
    CONSTRAINT stack_image_scans_status_check CHECK (status IN ('scanned', 'failed'))
);
