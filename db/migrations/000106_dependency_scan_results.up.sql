-- 000106_dependency_scan_results.up.sql
-- 파이프라인 의존성 취약점 스캔(SCA, trivy fs)의 판정과 요약을 남긴다
-- (nullus-plan#79 · #65 Security Dashboard 설계 §8 G1).
--
-- 이미지 스캔(000078)과 같은 틀이다 — 요약 + 판정 + 원본 위치만 둔다. 이미지 칸은 없다(소스를 본다).
-- 원본 리포트(dependency-scan-report.json)는 CI 산출물로만 남고, report_ref 로 다시 읽는다.
--
-- 건수 컬럼은 NULL 을 허용한다. 리포트를 읽지 못한 실행에 0 을 채우면 "취약점 0건" 으로 읽힌다.
-- targets 는 리포트의 Results 수(검사한 락파일 수)다. 0 이면 검사할 의존성이 없었다는 뜻이라
-- 대시보드가 통과가 아니라 "대상 없음" 으로 보인다 — 스캐폴딩은 락파일을 만들지 않는다(#79 §1).
--
-- deployment_id 에 FK 를 두지 않는다 — image_scan_results 와 같다. 실행 기록과 스캔 기록의
-- 동기화 순서를 묶지 않는다.
CREATE TABLE IF NOT EXISTS dependency_scan_results (
    id               VARCHAR(180) PRIMARY KEY,
    pipeline_id      VARCHAR(100) NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
    deployment_id    VARCHAR(160),

    scanner          VARCHAR(50)  NOT NULL,
    scanner_version  VARCHAR(50)  NOT NULL DEFAULT '',
    db_updated_at    TIMESTAMPTZ,

    critical_count   INTEGER,
    high_count       INTEGER,
    medium_count     INTEGER,
    low_count        INTEGER,
    unknown_count    INTEGER,
    targets          INTEGER,

    gate_result      VARCHAR(20)  NOT NULL
        CHECK (gate_result IN ('pass', 'warn', 'block', 'error')),
    report_uri       TEXT         NOT NULL DEFAULT '',
    report_ref       JSONB,
    scanned_at       TIMESTAMPTZ  NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_dependency_scan_results_pipeline_scanned
    ON dependency_scan_results (pipeline_id, scanned_at DESC);
