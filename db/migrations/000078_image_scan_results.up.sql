-- 000078_image_scan_results.up.sql
-- 이미지 스캔 게이트의 판정과 요약을 남긴다 (설계 §8).
--
-- 원본 리포트는 넣지 않는다 — 이미지 하나에 수 MB 다. 요약 + 판정 + 원본 위치만 둔다.
--
-- 건수 컬럼은 NULL 을 허용한다. 설계 초안은 NOT NULL DEFAULT 0 이었으나,
-- 리포트를 읽지 못한 실행(CI 단계 상태로만 판정한 경우)에 0 을 채우면
-- "취약점 0건" 으로 읽힌다. NULL 은 "모름" 이다.
--
-- gate_result 에 error 를 둔다. 스캔이 못 돈 것과 취약점이 없는 것은 다르다.
CREATE TABLE IF NOT EXISTS image_scan_results (
    id               VARCHAR(160) PRIMARY KEY,
    pipeline_id      VARCHAR(100) NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
    deployment_id    VARCHAR(160),

    image_repository VARCHAR(500) NOT NULL DEFAULT '',
    image_tag        VARCHAR(255) NOT NULL DEFAULT '',
    image_digest     VARCHAR(255) NOT NULL DEFAULT '',

    scan_source      VARCHAR(20)  NOT NULL,
    scanner          VARCHAR(50)  NOT NULL,
    scanner_version  VARCHAR(50)  NOT NULL DEFAULT '',
    db_updated_at    TIMESTAMPTZ,

    critical_count   INTEGER,
    high_count       INTEGER,
    medium_count     INTEGER,
    low_count        INTEGER,
    unknown_count    INTEGER,

    gate_result      VARCHAR(20)  NOT NULL
        CHECK (gate_result IN ('pass', 'warn', 'block', 'error')),
    report_uri       TEXT         NOT NULL DEFAULT '',
    scanned_at       TIMESTAMPTZ  NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_image_scan_results_pipeline_scanned
    ON image_scan_results (pipeline_id, scanned_at DESC);
