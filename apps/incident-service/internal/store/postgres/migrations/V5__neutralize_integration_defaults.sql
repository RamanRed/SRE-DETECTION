ALTER TABLE platform_integrations
    ALTER COLUMN jenkins_url SET DEFAULT 'http://localhost:8080',
    ALTER COLUMN jenkins_job_name SET DEFAULT 'sre-copilot-pipeline';
