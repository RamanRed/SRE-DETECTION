CREATE TABLE IF NOT EXISTS incident_analyses (
    id                   VARCHAR(36) PRIMARY KEY,
    incident_id          VARCHAR(36) NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    root_cause           TEXT NOT NULL,
    immediate_mitigation TEXT,
    confidence_score     VARCHAR(50),
    severity             VARCHAR(50),
    affected_components  TEXT NOT NULL DEFAULT '[]',
    created_at           TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_incident_analyses_incident_created
    ON incident_analyses(incident_id, created_at DESC);
