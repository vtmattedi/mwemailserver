CREATE TABLE IF NOT EXISTS email_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    email_attempt_id INTEGER,

    provider TEXT NOT NULL,

    -- For Resend this is the `svix-id` request header.
    -- Resend provides at-least-once delivery, so this is used to deduplicate retries/replays.
    provider_event_id TEXT NOT NULL,

    -- Provider email ID from webhook payload, e.g. Resend data.email_id.
    provider_email_id TEXT,

    -- e.g. email.sent, email.delivered, email.bounced, email.opened, email.clicked.
    event_type TEXT NOT NULL,

    -- Provider's event creation timestamp from the webhook payload.
    occurred_at TEXT NOT NULL,

    -- Time our gateway accepted/persisted this webhook.
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
