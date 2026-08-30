ALTER TABLE integration_commit_fingerprints
    ADD COLUMN IF NOT EXISTS state VARCHAR(16) NOT NULL DEFAULT 'PROCESSED',
    ADD COLUMN IF NOT EXISTS attempt_count INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS build_state TEXT,
    ADD COLUMN IF NOT EXISTS last_error TEXT,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ADD COLUMN IF NOT EXISTS processed_at TIMESTAMP;

ALTER TABLE platform_integrations
    ALTER COLUMN polling_cadence SET DEFAULT '15_MINUTES';

UPDATE platform_integrations
SET polling_cadence = CASE polling_cadence
    WHEN '5m' THEN '5_MINUTES'
    WHEN '15m' THEN '15_MINUTES'
    WHEN '1h' THEN '1_HOUR'
    WHEN 'daily' THEN 'DAILY_CRON'
    ELSE polling_cadence
END;

UPDATE integration_commit_fingerprints
SET state='PROCESSED', processed_at=COALESCE(processed_at, discovered_at), updated_at=discovered_at
WHERE state IS NULL OR state='PROCESSED';

ALTER TABLE integration_commit_fingerprints
    DROP CONSTRAINT IF EXISTS chk_commit_fingerprint_state;

ALTER TABLE integration_commit_fingerprints
    ADD CONSTRAINT chk_commit_fingerprint_state
    CHECK (state IN ('PROCESSING','FAILED','PROCESSED'));
