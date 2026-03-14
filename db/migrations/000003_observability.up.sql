-- Alert Rules
CREATE TYPE alert_channel  AS ENUM ('slack', 'email');

CREATE TABLE alert_rules (
    id         VARCHAR(100) PRIMARY KEY,
    name       VARCHAR(255) NOT NULL,
    condition  TEXT         NOT NULL,
    threshold  DOUBLE PRECISION NOT NULL DEFAULT 0,
    channel    alert_channel NOT NULL DEFAULT 'slack',
    enabled    BOOLEAN      NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Alerts (fired alert history)
CREATE TYPE alert_severity AS ENUM ('critical', 'warning', 'info');

CREATE TABLE alerts (
    id          VARCHAR(100) PRIMARY KEY,
    rule_id     VARCHAR(100) NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
    severity    alert_severity NOT NULL DEFAULT 'info',
    message     TEXT         NOT NULL,
    fired_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

CREATE INDEX idx_alerts_rule_id   ON alerts(rule_id);
CREATE INDEX idx_alerts_severity  ON alerts(severity);
CREATE INDEX idx_alerts_fired_at  ON alerts(fired_at DESC);
