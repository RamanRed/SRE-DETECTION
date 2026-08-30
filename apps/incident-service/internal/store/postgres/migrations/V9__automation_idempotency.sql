ALTER TABLE incidents
    ADD COLUMN IF NOT EXISTS source_event_key VARCHAR(128);

CREATE UNIQUE INDEX IF NOT EXISTS uq_incidents_source_event_key
    ON incidents(source_event_key)
    WHERE source_event_key IS NOT NULL;

ALTER TABLE pipeline_builds
    ADD COLUMN IF NOT EXISTS external_build_id VARCHAR(255);
