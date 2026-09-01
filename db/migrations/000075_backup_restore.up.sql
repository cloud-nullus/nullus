-- 000075_backup_restore.up.sql
-- 백업/복구 실행 이력을 남긴다.
--
-- 설계: docs/11_기능설계/Nullus_백업복구_설계.md §7.1 (nullus-plan#75)
--
-- 이력이 없으면 "언제 백업이 끊겼나" 를 알 수 없고, 그것을 모르는 상태가
-- 백업 실패 자체보다 나쁘다(설계 §9 F10). 그래서 산출물이 보존 정책으로
-- 지워진 뒤에도 실행 기록은 남긴다.

-- backup_runs : 한 번의 백업 실행
CREATE TABLE IF NOT EXISTS backup_runs (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id             UUID        NOT NULL,
    -- E1(워크로드 데이터) 대상 스택. NULL 이면 플랫폼 전용 백업이다.
    stack_id           UUID,
    trigger            VARCHAR(20) NOT NULL,
    -- full | platform_only | stack_only (설계 §6.5)
    mode               VARCHAR(20) NOT NULL,
    scope              TEXT[]      NOT NULL DEFAULT '{}',
    -- pending | running | succeeded | partial | failed
    --
    -- partial 이 따로 있는 이유: 일부 컴포넌트만 성공하는 경우가 실제로 생긴다.
    -- failed 로 뭉뚱그리면 "부분적으로 쓸 수 있는 백업" 과 "아무것도 없는 상태"
    -- 가 구분되지 않고, 복구 시점에 그 차이가 결정적이다.
    status             VARCHAR(20) NOT NULL DEFAULT 'pending',
    -- 백업 시점의 schema_migrations 최신값. 복구 시 정합성 판단 기준(설계 §6.2).
    schema_version     INTEGER,
    -- 정지 창. 사용자가 감수한 실제 다운타임이며, 이력이 없으면 정지 창 정책을
    -- 조정할 근거가 없다(설계 §3.4).
    quiesce_started_at TIMESTAMPTZ,
    quiesce_ended_at   TIMESTAMPTZ,
    -- 매니페스트. 비밀값은 한 조각도 담지 않는다(설계 §4.4).
    manifest           JSONB       NOT NULL DEFAULT '{}'::jsonb,
    total_bytes        BIGINT,
    error              TEXT,
    started_at         TIMESTAMPTZ,
    finished_at        TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 조회는 "이 조직의 최근 백업" 이다. 보존 정책도 같은 축으로 훑는다.
CREATE INDEX IF NOT EXISTS idx_backup_runs_org_created
    ON backup_runs (org_id, created_at DESC);

-- 스케줄러가 "마지막 성공 시점" 을 묻는다(설계 §9 F10).
CREATE INDEX IF NOT EXISTS idx_backup_runs_status_created
    ON backup_runs (status, created_at DESC);

-- backup_artifacts : 산출물 1건
CREATE TABLE IF NOT EXISTS backup_artifacts (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    backup_run_id     UUID        NOT NULL REFERENCES backup_runs(id) ON DELETE CASCADE,
    -- platform_db | keycloak_db | openbao_kv | ns_resources | volume
    component         VARCHAR(40) NOT NULL,
    -- component=volume 일 때 PVC 이름. 그 외에는 빈 문자열.
    resource_name     TEXT        NOT NULL DEFAULT '',
    -- 오브젝트 스토리지 경로. 값 자체는 담지 않는다.
    location          TEXT        NOT NULL,
    size_bytes        BIGINT      NOT NULL DEFAULT 0,
    checksum_sha256   TEXT        NOT NULL,
    -- 어떤 키로 잠갔는지만 남긴다. 키 자체는 절대 담지 않는다(설계 §5.1).
    encryption_key_id TEXT        NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_backup_artifacts_run
    ON backup_artifacts (backup_run_id);

-- restore_runs : 복구 실행과 검사 결과
CREATE TABLE IF NOT EXISTS restore_runs (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    backup_run_id    UUID        REFERENCES backup_runs(id),
    mode             VARCHAR(20) NOT NULL,
    status           VARCHAR(20) NOT NULL DEFAULT 'pending',
    -- 스키마 버전 정합성 검사 결과(설계 §6.2)
    schema_check     JSONB       NOT NULL DEFAULT '{}'::jsonb,
    -- token_sources ↔ OpenBao dangling 목록(설계 §6.4).
    -- 조용히 넘어가지 않게 하는 것이 핵심이라 결과를 반드시 남긴다.
    integrity_report JSONB       NOT NULL DEFAULT '{}'::jsonb,
    error            TEXT,
    started_at       TIMESTAMPTZ,
    finished_at      TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_restore_runs_backup
    ON restore_runs (backup_run_id, created_at DESC);
