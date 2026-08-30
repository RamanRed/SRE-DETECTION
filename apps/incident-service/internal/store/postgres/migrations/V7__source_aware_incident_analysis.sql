ALTER TABLE incident_analyses
    ADD COLUMN IF NOT EXISTS unified_diff TEXT,
    ADD COLUMN IF NOT EXISTS verification_plan TEXT,
    ADD COLUMN IF NOT EXISTS rollback_plan TEXT,
    ADD COLUMN IF NOT EXISTS cited_source_paths TEXT NOT NULL DEFAULT '[]',
    ADD COLUMN IF NOT EXISTS repository_url VARCHAR(2048),
    ADD COLUMN IF NOT EXISTS commit_sha VARCHAR(128),
    ADD COLUMN IF NOT EXISTS commit_message TEXT,
    ADD COLUMN IF NOT EXISTS repository_provider VARCHAR(32),
    ADD COLUMN IF NOT EXISTS target_branch VARCHAR(255),
    ADD COLUMN IF NOT EXISTS ci_provider VARCHAR(32),
    ADD COLUMN IF NOT EXISTS build_url VARCHAR(2048);

ALTER TABLE remediation_records
    ADD COLUMN IF NOT EXISTS unified_diff TEXT,
    ADD COLUMN IF NOT EXISTS verification_plan TEXT,
    ADD COLUMN IF NOT EXISTS rollback_plan TEXT;
