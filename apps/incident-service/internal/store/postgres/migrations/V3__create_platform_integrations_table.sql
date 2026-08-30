CREATE TABLE IF NOT EXISTS platform_integrations (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(100) NOT NULL UNIQUE,
    username VARCHAR(100),
    github_token VARCHAR(512),
    github_repo VARCHAR(255),
    github_branch VARCHAR(100) DEFAULT 'main',
    github_status VARCHAR(50) DEFAULT 'DISCONNECTED',
    jenkins_url VARCHAR(255) DEFAULT 'http://localhost:8080',
    jenkins_username VARCHAR(100),
    jenkins_api_token VARCHAR(512),
    jenkins_job_name VARCHAR(255) DEFAULT 'sre-copilot-pipeline',
    jenkins_status VARCHAR(50) DEFAULT 'DISCONNECTED',
    last_sync_time TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_platform_integrations_user_id ON platform_integrations(user_id);
