CREATE TABLE IF NOT EXISTS pipeline_builds (
    id VARCHAR(36) PRIMARY KEY,
    pipeline_name VARCHAR(100) NOT NULL,
    build_number INT NOT NULL,
    ci_tool VARCHAR(50) NOT NULL DEFAULT 'JENKINS',
    status VARCHAR(50) NOT NULL,
    git_commit VARCHAR(50),
    git_branch VARCHAR(100),
    commit_message VARCHAR(255),
    author VARCHAR(100),
    duration_seconds INT,
    tests_passed INT DEFAULT 0,
    tests_failed INT DEFAULT 0,
    vulnerabilities_detected INT DEFAULT 0,
    environment VARCHAR(50) DEFAULT 'production',
    log_snippet TEXT,
    build_url VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pipeline_builds_name ON pipeline_builds(pipeline_name);
CREATE INDEX IF NOT EXISTS idx_pipeline_builds_status ON pipeline_builds(status);
CREATE INDEX IF NOT EXISTS idx_pipeline_builds_created_at ON pipeline_builds(created_at DESC);
