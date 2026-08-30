ALTER TABLE platform_integrations
    ADD COLUMN IF NOT EXISTS repository_provider VARCHAR(32),
    ADD COLUMN IF NOT EXISTS repository_url VARCHAR(1024),
    ADD COLUMN IF NOT EXISTS target_branch VARCHAR(255),
    ADD COLUMN IF NOT EXISTS pipeline_engine VARCHAR(32),
    ADD COLUMN IF NOT EXISTS ci_base_url VARCHAR(1024),
    ADD COLUMN IF NOT EXISTS ci_username VARCHAR(255),
    ADD COLUMN IF NOT EXISTS job_name VARCHAR(255),
    ADD COLUMN IF NOT EXISTS polling_cadence VARCHAR(16) NOT NULL DEFAULT '15_MINUTES',
    ADD COLUMN IF NOT EXISTS auto_rebuild BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS auto_ai_triage BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS connection_status VARCHAR(32) NOT NULL DEFAULT 'DISCONNECTED',
    ADD COLUMN IF NOT EXISTS last_polled_commit VARCHAR(128),
    ADD COLUMN IF NOT EXISTS last_poll_time TIMESTAMP,
    ADD COLUMN IF NOT EXISTS next_poll_time TIMESTAMP;

ALTER TABLE platform_integrations
    ALTER COLUMN github_token TYPE TEXT,
    ALTER COLUMN jenkins_api_token TYPE TEXT;

UPDATE platform_integrations
SET repository_provider = COALESCE(repository_provider, 'GITHUB'),
    repository_url = COALESCE(repository_url,
        CASE WHEN github_repo IS NOT NULL THEN 'https://github.com/' || github_repo END),
    target_branch = COALESCE(target_branch, github_branch, 'main'),
    pipeline_engine = COALESCE(pipeline_engine, 'JENKINS'),
    ci_base_url = COALESCE(ci_base_url, jenkins_url),
    ci_username = COALESCE(ci_username, jenkins_username),
    job_name = COALESCE(job_name, jenkins_job_name, 'sre-copilot-pipeline'),
    connection_status = 'DISCONNECTED',
    next_poll_time = NULL;

UPDATE platform_integrations
SET polling_cadence = CASE polling_cadence
    WHEN '5m' THEN '5_MINUTES'
    WHEN '15m' THEN '15_MINUTES'
    WHEN '1h' THEN '1_HOUR'
    WHEN 'daily' THEN 'DAILY_CRON'
    ELSE polling_cadence
END;

-- Legacy values were plaintext. Require an explicit reconnect so no plaintext secret survives cutover.
UPDATE platform_integrations
SET github_token = NULL
WHERE github_token IS NOT NULL AND github_token NOT LIKE 'enc:v1:%';

UPDATE platform_integrations
SET jenkins_api_token = NULL
WHERE jenkins_api_token IS NOT NULL AND jenkins_api_token NOT LIKE 'enc:v1:%';

CREATE TABLE IF NOT EXISTS integration_commit_fingerprints (
    id BIGSERIAL PRIMARY KEY,
    integration_id BIGINT NOT NULL REFERENCES platform_integrations(id) ON DELETE CASCADE,
    fingerprint VARCHAR(64) NOT NULL,
    commit_sha VARCHAR(128) NOT NULL,
    commit_message TEXT,
    discovered_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(integration_id, fingerprint)
);

CREATE INDEX IF NOT EXISTS idx_integrations_poll_due
    ON platform_integrations(connection_status, next_poll_time);
