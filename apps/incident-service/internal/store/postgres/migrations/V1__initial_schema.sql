CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS incidents (
    id            VARCHAR(36)  PRIMARY KEY DEFAULT gen_random_uuid()::VARCHAR,
    title         VARCHAR(255) NOT NULL,
    service_name  VARCHAR(100) NOT NULL,
    raw_logs      TEXT,
    firing_rule   VARCHAR(255),
    environment   VARCHAR(50)  NOT NULL DEFAULT 'production',
    status        VARCHAR(50)  NOT NULL DEFAULT 'OPEN',
    severity      VARCHAR(50),
    created_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at   TIMESTAMP,
    created_by    VARCHAR(100) NOT NULL DEFAULT 'system',
    CONSTRAINT chk_incidents_status CHECK (status IN ('OPEN', 'ANALYZING', 'RESOLVED', 'CLOSED')),
    CONSTRAINT chk_incidents_severity CHECK (severity IS NULL OR severity IN ('LOW', 'MEDIUM', 'HIGH', 'CRITICAL')),
    CONSTRAINT chk_incidents_env CHECK (environment IN ('development', 'staging', 'production'))
);

CREATE TABLE IF NOT EXISTS remediation_records (
    id                  VARCHAR(36) PRIMARY KEY DEFAULT gen_random_uuid()::VARCHAR,
    incident_id         VARCHAR(36) NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    ai_root_cause       TEXT,
    suggested_action    TEXT,
    script_type         VARCHAR(50),
    executable_script   TEXT,
    confidence_score    VARCHAR(10),
    affected_components TEXT,
    requires_approval   BOOLEAN NOT NULL DEFAULT TRUE,
    applied_by          VARCHAR(100),
    execution_status    VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    created_at          TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    applied_at          TIMESTAMP,
    CONSTRAINT chk_remediation_status CHECK (execution_status IN ('PENDING', 'APPROVED', 'EXECUTING', 'APPLIED', 'FAILED', 'REJECTED'))
);

CREATE INDEX IF NOT EXISTS idx_incidents_status ON incidents(status);
CREATE INDEX IF NOT EXISTS idx_incidents_service_name ON incidents(service_name);
CREATE INDEX IF NOT EXISTS idx_incidents_created_at ON incidents(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_remediation_incident_id ON remediation_records(incident_id);
