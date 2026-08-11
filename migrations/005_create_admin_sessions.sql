CREATE TABLE IF NOT EXISTS admin_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    admin_user_id INTEGER NOT NULL,

    -- SHA-256 of the raw HttpOnly session token.
    token_hash TEXT NOT NULL UNIQUE,

    -- SHA-256 of the current X-CSRF-Token value.
    csrf_token_hash TEXT NOT NULL,

    expires_at TEXT NOT NULL,

    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (admin_user_id)
        REFERENCES admin_users(id)
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_admin_sessions_user
    ON admin_sessions(admin_user_id);

CREATE INDEX IF NOT EXISTS idx_admin_sessions_expires
    ON admin_sessions(expires_at);
