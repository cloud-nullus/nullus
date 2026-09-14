-- 000082_image_scan_report_ref.up.sql
-- 스캔 결과에 CI 리포트의 위치를 남긴다.
--
-- 취약점 목록은 볼 때 CI 리포트를 다시 읽어 만든다. 원본 리포트는 DB 에 넣지 않는다
-- (이미지 스캔 설계 §8). 다시 읽으려면 CI 마다 필요한 식별자(GitLab 잡 id, GitHub
-- 실행 id, Jenkins 빌드 번호)가 있어야 한다. 비어 있으면 목록을 보일 수 없다.
ALTER TABLE image_scan_results ADD COLUMN IF NOT EXISTS report_ref JSONB;
