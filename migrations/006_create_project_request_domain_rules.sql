-- MW_ALLOWED_REQUEST_DOMAINS_V1
-- No row means "*", preserving existing project behavior.
CREATE TABLE IF NOT EXISTS project_request_domain_rules (
    project_id INTEGER PRIMARY KEY,
    rule TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    CHECK (length(rule) BETWEEN 1 AND 2048)
);

CREATE INDEX IF NOT EXISTS idx_project_request_domain_rules_project_id
    ON project_request_domain_rules(project_id);
