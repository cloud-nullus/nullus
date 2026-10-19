-- 설치 이미지 스캔이 스택 네임스페이스 밖의 공용 구성요소(cert-manager · metrics-server · External Secrets)도
-- 본다(nullus-plan#78). 어느 네임스페이스의 것인지, 공용인지 남긴다.
ALTER TABLE stack_image_scans
    ADD COLUMN IF NOT EXISTS namespace VARCHAR(253) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS shared    BOOLEAN      NOT NULL DEFAULT false;
