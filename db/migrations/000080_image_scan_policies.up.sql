-- 000080_image_scan_policies.up.sql
-- 스택별 이미지 스캔 정책 (설계 §6 — 정책 푸시 방식).
--
-- 정책은 스택 단위다. 스캐너가 스택마다 서고 장애 영향도 스택 하나로 막힌다.
-- 플랫폼은 이 값을 CI 변수(GitLab·GitHub)나 스택 네임스페이스의 ConfigMap(Jenkins)으로
-- 푸시하고, CI 가 그 값으로 스스로 판정한다.
--
-- 행이 없으면 기본 정책(CRITICAL 차단 · unfixed 제외 · 스캔 불가 시 차단)이다.
-- 기본값을 행으로 미리 넣지 않는다 — 운영자가 고른 값과 기본값을 구분할 수 없게 된다.
CREATE TABLE IF NOT EXISTS image_scan_policies (
    stack_id               VARCHAR(100) PRIMARY KEY REFERENCES stacks(id) ON DELETE CASCADE,
    block_severity         VARCHAR(10)  NOT NULL
        CHECK (block_severity IN ('CRITICAL', 'HIGH', 'MEDIUM', 'LOW')),
    ignore_unfixed         BOOLEAN      NOT NULL,
    on_scanner_unreachable VARCHAR(10)  NOT NULL
        CHECK (on_scanner_unreachable IN ('block', 'allow')),
    updated_by             VARCHAR(255) NOT NULL DEFAULT '',
    updated_at             TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
