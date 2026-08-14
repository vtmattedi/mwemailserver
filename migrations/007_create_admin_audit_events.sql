CREATE TABLE IF NOT EXISTS admin_audit_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    event_type TEXT NOT NULL,
    actor_type TEXT NOT NULL
        CHECK (actor_type IN ('local', 'federated')),
    actor_identity_sub TEXT,
    actor_local_admin_id INTEGER,

    project_id INTEGER NOT NULL,
    project_name TEXT NOT NULL,
    request_id TEXT NOT NULL DEFAULT '',
    occurred_at TEXT NOT NULL,

    CHECK (
        (actor_type = 'federated'
            AND actor_identity_sub IS NOT NULL
            AND actor_identity_sub <> ''
            AND actor_local_admin_id IS NULL)
        OR
        (actor_type = 'local'
            AND actor_local_admin_id IS NOT NULL
            AND actor_identity_sub IS NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_admin_audit_events_project
    ON admin_audit_events(project_id, occurred_at);

CREATE TRIGGER IF NOT EXISTS trg_admin_audit_events_no_update
BEFORE UPDATE ON admin_audit_events
BEGIN
    SELECT RAISE(ABORT, 'admin audit events are immutable');
END;

CREATE TRIGGER IF NOT EXISTS trg_admin_audit_events_no_delete
BEFORE DELETE ON admin_audit_events
BEGIN
    SELECT RAISE(ABORT, 'admin audit events are immutable');
END;
