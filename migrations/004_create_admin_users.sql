CREATE TABLE IF NOT EXISTS admin_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    email TEXT NOT NULL COLLATE NOCASE UNIQUE,

    -- PHC-formatted Argon2id hash.
    password_hash TEXT NOT NULL,

    enabled INTEGER NOT NULL DEFAULT 1
        CHECK (enabled IN (0, 1)),

    last_login_at TEXT,

    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_admin_users_enabled
    ON admin_users(enabled);
