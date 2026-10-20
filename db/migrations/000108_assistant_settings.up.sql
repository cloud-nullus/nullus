-- AI 어시스턴트 설정(nullus-plan#94). 조직마다 모델 공급자 · 주소 · 모델 · API 키를 관리 화면에서 등록한다.
-- 키는 ENCRYPTION_KEY 로 암호화해 둔다(pkg/crypto, 알림 채널 비밀과 같은 방식). assistant 모듈 소유.
CREATE TABLE IF NOT EXISTS assistant_settings (
    org_id            VARCHAR(255) PRIMARY KEY,
    enabled           BOOLEAN      NOT NULL DEFAULT false,
    provider          VARCHAR(50)  NOT NULL,
    base_url          TEXT         NOT NULL DEFAULT '',
    model             VARCHAR(200) NOT NULL DEFAULT '',
    api_key_encrypted TEXT         NOT NULL DEFAULT '',
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_by        VARCHAR(255) NOT NULL DEFAULT ''
);
