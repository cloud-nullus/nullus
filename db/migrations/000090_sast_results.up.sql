-- 000090_sast_results.up.sql
-- 파이프라인 소스 정적 분석(SAST)의 판정과 요약을 남긴다 (nullus-plan#64 결과 저장 범위).
--
-- 이미지 스캔(000078)과 같은 범위다 — 요약 + 판정 + 원본 위치만 둔다. 이슈 목록 원본은
-- SonarQube 가 갖고, dashboard_url 로 연다.
--
-- 지표 컬럼은 NULL 을 허용한다. 분석하지 못한 실행에 0 을 채우면 "문제 0건" 으로 읽힌다.
-- gate_result 는 파이프라인 관점의 판정이다 — 같은 Quality Gate 실패도 스택 정책에 따라
-- block·warn 으로 갈리고, 분석을 못 한 것은 error 다. quality_gate_status 는 SonarQube 의
-- 판정(OK·ERROR·NONE) 그대로이고, 분석하지 못했으면 비어 있다.
CREATE TABLE IF NOT EXISTS sast_results (
    id                       VARCHAR(160) PRIMARY KEY,
    pipeline_id              VARCHAR(100) NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
    deployment_id            VARCHAR(160),

    project_key              VARCHAR(400) NOT NULL DEFAULT '',
    analysis_id              VARCHAR(100) NOT NULL DEFAULT '',

    quality_gate_status      VARCHAR(10)  NOT NULL DEFAULT '',
    gate_result              VARCHAR(20)  NOT NULL
        CHECK (gate_result IN ('pass', 'warn', 'block', 'error')),
    conditions               JSONB        NOT NULL DEFAULT '[]'::jsonb,

    bugs                     INTEGER,
    vulnerabilities          INTEGER,
    code_smells              INTEGER,
    security_hotspots        INTEGER,
    coverage                 DOUBLE PRECISION,
    duplicated_lines_density DOUBLE PRECISION,
    ncloc                    INTEGER,

    dashboard_url            TEXT         NOT NULL DEFAULT '',
    analyzed_at              TIMESTAMPTZ  NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sast_results_pipeline_analyzed
    ON sast_results (pipeline_id, analyzed_at DESC);
