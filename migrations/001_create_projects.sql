CREATE TABLE IF NOT EXISTS projects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    name TEXT NOT NULL,

    -- SHA-256 of the project API token. Never store the raw token.
    token_hash TEXT NOT NULL UNIQUE,

    enabled INTEGER NOT NULL DEFAULT 1
        CHECK (enabled IN (0, 1)),

    monthly_limit INTEGER
        CHECK (monthly_limit IS NULL OR monthly_limit >= 0),

    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_projects_enabled
    ON projects(enabled);
