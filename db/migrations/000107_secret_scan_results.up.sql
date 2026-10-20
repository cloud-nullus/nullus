-- 000107_secret_scan_results.up.sql
-- 파이프라인 시크릿 탐지(gitleaks)의 판정과 탐지 위치를 남긴다
-- (nullus-plan#80 · #65 Security Dashboard 설계 §8 G2).
--
-- 비밀값은 어디에도 두지 않는다. gitleaks 리포트의 Secret · Match 는 --redact 로 가려져 있지만, 동기화는
-- 그 필드를 파싱하지도 않는다 — 탐지 위치(규칙 · 파일 · 줄 · 커밋 · Fingerprint)만 둔다. 이것만으로
-- 무엇을 폐기·회전할지, 오탐이면 .gitleaksignore 에 무엇을 적을지 알 수 있다.
--
-- findings_count 는 NULL 을 허용한다. 리포트가 없는 실행(스캔 실패)에 0 을 채우면 "탐지 없음" 으로 읽힌다.
-- findings 는 앞의 일부(동기화가 자르는 상한)만 두고 건수는 findings_count 가 정확히 갖는다 — 오탐
-- 규칙 하나가 수천 건을 내도 행이 커지지 않게 한다. rules 는 잘리지 않은 규칙별 건수다.
--
-- 판정은 리포트에 종료 코드가 없어 단계 결과와 탐지 건수로 추론한다(설계 §8 표). 스택 정책이
-- off 인 실행은 남기지 않는다(대시보드에서 "미적용").
CREATE TABLE IF NOT EXISTS secret_scan_results (
    id               VARCHAR(180) PRIMARY KEY,
    pipeline_id      VARCHAR(100) NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
    deployment_id    VARCHAR(160),

    scanner          VARCHAR(50)  NOT NULL,
    scanner_version  VARCHAR(50)  NOT NULL DEFAULT '',

    findings_count   INTEGER,
    rules            JSONB        NOT NULL DEFAULT '[]'::jsonb,
    findings         JSONB        NOT NULL DEFAULT '[]'::jsonb,

    gate_result      VARCHAR(20)  NOT NULL
        CHECK (gate_result IN ('pass', 'warn', 'block', 'error')),
    report_uri       TEXT         NOT NULL DEFAULT '',
    report_ref       JSONB,
    scanned_at       TIMESTAMPTZ  NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_secret_scan_results_pipeline_scanned
    ON secret_scan_results (pipeline_id, scanned_at DESC);
