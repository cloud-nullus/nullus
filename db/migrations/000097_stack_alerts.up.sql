-- 스택 심각 경보 전달(nullus-plan#87, docs/11_기능설계/Nullus_스택_경보_전달_설계.md).
--
-- 플랫폼이 스택 Alertmanager 에서 가져온 심각 경보를 발생부터 해제까지 한 행으로 남긴다. 같은 경보
-- (stack_id, fingerprint)가 열려 있는 동안은 한 행뿐이다 — API 레플리카가 여럿이어도 새 행은 하나만
-- 생기고, 알림 차례(last_notified_at)와 해제는 행 잠금으로 가린다. 스택 테이블은 스택 모듈의 것이라
-- 외래 키를 걸지 않는다(지워진 스택의 열린 경보는 수집기가 닫는다).
CREATE TABLE IF NOT EXISTS stack_alerts (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id           VARCHAR(255) NOT NULL,
    stack_id         VARCHAR(100) NOT NULL,
    stack_name       VARCHAR(255) NOT NULL DEFAULT '',
    fingerprint      VARCHAR(64)  NOT NULL,
    alertname        VARCHAR(255) NOT NULL,
    severity         VARCHAR(20)  NOT NULL,
    summary          TEXT         NOT NULL DEFAULT '',
    description      TEXT         NOT NULL DEFAULT '',
    runbook_url      TEXT         NOT NULL DEFAULT '',
    labels           JSONB        NOT NULL DEFAULT '{}',
    started_at       TIMESTAMPTZ  NOT NULL,
    last_seen_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    resolved_at      TIMESTAMPTZ,
    last_notified_at TIMESTAMPTZ,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_stack_alerts_open ON stack_alerts (stack_id, fingerprint) WHERE resolved_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_stack_alerts_org_started ON stack_alerts (org_id, started_at DESC);

-- 데모 이력을 지운다(000021 시드). 화면이 실제 경보·발송을 보여 주기 시작하면 데모 행은 진짜와 섞여
-- 무엇이 실제로 일어났는지 가린다. 000021 에서도 뺐다 — 이 문장은 이미 시드가 들어간 DB 를 위한 것이다.
DELETE FROM alerts WHERE id IN ('al-1', 'al-2', 'al-3', 'al-4', 'al-5', 'al-6');
DELETE FROM notification_history
WHERE org_id = '11111111-1111-1111-1111-111111111111'
  AND channel_id IS NULL
  AND (event, created_at) IN (
      ('pipeline_failure', TIMESTAMPTZ '2026-03-19T10:10:00Z'),
      ('deploy_rollback',  TIMESTAMPTZ '2026-03-19T10:12:00Z'),
      ('high_cpu',         TIMESTAMPTZ '2026-03-20T09:01:00Z'),
      ('storage_warning',  TIMESTAMPTZ '2026-03-21T08:01:00Z')
  );
