-- 알림 채널(nullus-plan#63, docs/11_기능설계/Nullus_알림_채널_설계.md) — Telegram · 이메일(SMTP) · Slack.
--
-- notification_configs 한 행이 채널 하나다. config 는 비밀이 아닌 설정(chat_id · SMTP 서버 · 받는
-- 주소)만 담고, 비밀(봇 토큰 · 웹훅 URL · SMTP 비밀번호)은 secret_encrypted 에 ENCRYPTION_KEY 로
-- 암호화해 둔다(AES-256-GCM, pkg/crypto). 이 열이 생기기 전에 config 에 평문으로 들어간 비밀은 API 가
-- 시작할 때 옮겨 암호화한다(알림 모듈 wiring).
ALTER TABLE notification_configs
    ADD COLUMN IF NOT EXISTS name VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS secret_encrypted TEXT,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

UPDATE notification_configs SET name = channel WHERE name = '';

CREATE INDEX IF NOT EXISTS idx_notification_configs_org ON notification_configs (org_id, created_at);

-- 발송 이력은 어느 채널로 갔는지(채널을 지워도 이력은 남는다)와 몇 번 보내 봤는지를 담는다.
ALTER TABLE notification_history
    ADD COLUMN IF NOT EXISTS channel_id UUID REFERENCES notification_configs (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS attempts INT NOT NULL DEFAULT 1;

CREATE INDEX IF NOT EXISTS idx_notification_history_org_created ON notification_history (org_id, created_at DESC);
