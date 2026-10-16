-- 시크릿 탐지 단계(gitleaks)의 동작을 스택 스캔 정책에 더한다 (nullus-plan#80,
-- docs/11_기능설계/Nullus_시크릿_탐지_설계.md).
--
-- block(기본) · warn · off. 이미지 스캔·SAST 와 달리 "off" 가 있다 — 스택에 설치하는 것이 없어
-- 단계가 모든 파이프라인에 렌더되므로, 끄는 길은 정책이어야 한다. 이 열이 생기기 전에 저장된
-- 행은 차단으로 읽힌다 — 빈 값을 경고나 끄기로 읽으면 조용히 게이트가 풀린다.
ALTER TABLE image_scan_policies
    ADD COLUMN IF NOT EXISTS secret_detection VARCHAR(10) NOT NULL DEFAULT 'block'
        CHECK (secret_detection IN ('block', 'warn', 'off'));
