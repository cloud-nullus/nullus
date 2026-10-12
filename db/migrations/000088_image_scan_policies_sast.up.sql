-- 000088_image_scan_policies_sast.up.sql
-- 스택 스캔 정책에 소스 정적 분석(SAST)의 Quality Gate 실패 시 동작을 더한다.
--
-- 기본은 차단이다 — 이미지 스캔과 같다. 경고(warn)는 Quality Gate 를 막 들인 팀이
-- 기존 코드의 문제로 배포가 멈추지 않게 하는 길이다. 플랫폼은 이 값을 CI 변수
-- NULLUS_SAST_ON_GATE_FAILURE 로 싣는다(Jenkins 는 nullus-scan-policy ConfigMap).
--
-- 이미 있는 행은 차단으로 읽힌다. 분석을 수행하지 못했을 때(SonarQube 장애)는 이미지
-- 스캔과 같은 on_scanner_unreachable 을 따른다.
ALTER TABLE image_scan_policies
    ADD COLUMN IF NOT EXISTS sast_on_gate_failure VARCHAR(10) NOT NULL DEFAULT 'block'
        CHECK (sast_on_gate_failure IN ('block', 'warn'));
