-- 000083_stack_image_vulnerabilities.up.sql
-- 스택 설치 이미지의 취약점 목록.
--
-- 건수만으로는 무엇을 고쳐야 하는지 알 수 없다. 설치 이미지는 CI 리포트가 없어 스캔
-- 결과를 다시 읽을 곳이 없으므로 스캔할 때 목록을 저장한다. 스캔 결과 행과 함께 교체되고
-- (ON DELETE CASCADE), 이미지별로만 읽는다.
--
-- vulnerabilities_recorded 는 목록을 저장했는지다. 목록 기능 전에 스캔한 이미지와
-- 취약점이 0건인 이미지를 가르는 데 쓴다 — 둘 다 행이 없다.

ALTER TABLE stack_image_scans
    ADD COLUMN IF NOT EXISTS vulnerabilities_recorded BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE IF NOT EXISTS stack_image_vulnerabilities (
    stack_id          VARCHAR(100) NOT NULL,
    image_digest      VARCHAR(255) NOT NULL,
    vulnerability_id  TEXT         NOT NULL,
    pkg_name          TEXT         NOT NULL DEFAULT '',
    installed_version TEXT         NOT NULL DEFAULT '',
    fixed_version     TEXT         NOT NULL DEFAULT '',
    severity          VARCHAR(10)  NOT NULL,
    class             VARCHAR(20)  NOT NULL,
    target            TEXT         NOT NULL DEFAULT '',
    primary_url       TEXT         NOT NULL DEFAULT '',
    FOREIGN KEY (stack_id, image_digest)
        REFERENCES stack_image_scans (stack_id, image_digest) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_stack_image_vulnerabilities_image
    ON stack_image_vulnerabilities (stack_id, image_digest);
