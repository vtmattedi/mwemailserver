CREATE TABLE IF NOT EXISTS email_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    email_attempt_id INTEGER,

    provider TEXT NOT NULL,

    -- Resend/Svix request identifier from the svix-id header.
    -- Used to deduplicate at-least-once webhook delivery and replays.
    provider_event_id TEXT NOT NULL,

    -- Provider-side email ID from the webhook payload.
    provider_email_id TEXT,

    -- e.g. email.sent, email.delivered, email.bounced, email.opened.
    event_type TEXT NOT NULL,

    -- Canonical UTC RFC3339 timestamp from the provider payload.
    occurred_at TEXT NOT NULL,

    -- Time our gateway persisted the event.
    received_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (email_attempt_id)
        REFERENCES email_attempts(id)
        ON DELETE SET NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_email_events_provider_event
    ON email_events(provider, provider_event_id);

CREATE INDEX IF NOT EXISTS idx_email_events_attempt
    ON email_events(email_attempt_id);

CREATE INDEX IF NOT EXISTS idx_email_events_provider_email
    ON email_events(provider, provider_email_id);

CREATE INDEX IF NOT EXISTS idx_email_events_type
    ON email_events(event_type);

CREATE INDEX IF NOT EXISTS idx_email_events_occurred_at
    ON email_events(occurred_at);
