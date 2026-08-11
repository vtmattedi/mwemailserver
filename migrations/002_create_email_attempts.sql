CREATE TABLE IF NOT EXISTS email_attempts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    project_id INTEGER NOT NULL,

    -- Provider abstraction. Initially "resend".
    provider TEXT NOT NULL DEFAULT 'resend',

    -- Provider-side email identifier returned by the synchronous send request.
    provider_id TEXT,

    sender TEXT NOT NULL,

    -- Basic audit/debug information only.
    -- We intentionally do not persist the HTML/text email body.
    subject TEXT NOT NULL,

    recipient_count INTEGER NOT NULL
        CHECK (recipient_count > 0),

    -- Synchronous provider API state.
    -- "sent" here means the provider accepted the send request.
    -- Actual delivery state is tracked separately via webhooks.
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN (
            'pending',
            'sent',
            'failed'
        )),

    error_message TEXT,

    -- Asynchronous delivery state learned from provider webhooks.
    delivery_status TEXT
        CHECK (
            delivery_status IS NULL
            OR delivery_status IN (
                'sent',
                'delivered',
                'delivery_delayed',
                'bounced',
                'failed',
                'suppressed',
                'complained'
            )
        ),

    -- Canonical UTC RFC3339 timestamp of the provider event that most recently
    -- changed delivery_status.
    delivery_status_at TEXT,

    -- SHA-256 of the client-provided Idempotency-Key.
    -- The raw key is not persisted.
    idempotency_key_hash TEXT,

    -- SHA-256 of a deterministic representation of the email request.
    -- Same key + different request_hash => HTTP 409 conflict.
    request_hash TEXT,

    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (project_id)
        REFERENCES projects(id)
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_email_attempts_project_id
    ON email_attempts(project_id);

CREATE INDEX IF NOT EXISTS idx_email_attempts_created_at
    ON email_attempts(created_at);

CREATE INDEX IF NOT EXISTS idx_email_attempts_project_created
    ON email_attempts(project_id, created_at);

CREATE INDEX IF NOT EXISTS idx_email_attempts_status
    ON email_attempts(status);

CREATE INDEX IF NOT EXISTS idx_email_attempts_delivery_status
    ON email_attempts(delivery_status);

-- Fast lookup when a provider webhook references its email ID.
CREATE UNIQUE INDEX IF NOT EXISTS idx_email_attempts_provider_email
    ON email_attempts(provider, provider_id)
    WHERE provider_id IS NOT NULL
      AND provider_id <> '';

-- Idempotency is scoped to one gateway project.
CREATE UNIQUE INDEX IF NOT EXISTS idx_email_attempts_project_idempotency
    ON email_attempts(project_id, idempotency_key_hash)
    WHERE idempotency_key_hash IS NOT NULL
      AND idempotency_key_hash <> '';

CREATE TRIGGER IF NOT EXISTS trg_email_attempts_idempotency_insert
BEFORE INSERT ON email_attempts
FOR EACH ROW
WHEN NEW.idempotency_key_hash IS NOT NULL
     AND (NEW.request_hash IS NULL OR NEW.request_hash = '')
BEGIN
    SELECT RAISE(
        ABORT,
        'request_hash is required when idempotency_key_hash is set'
    );
END;

CREATE TRIGGER IF NOT EXISTS trg_email_attempts_idempotency_update
BEFORE UPDATE OF idempotency_key_hash, request_hash ON email_attempts
FOR EACH ROW
WHEN NEW.idempotency_key_hash IS NOT NULL
     AND (NEW.request_hash IS NULL OR NEW.request_hash = '')
BEGIN
    SELECT RAISE(
        ABORT,
        'request_hash is required when idempotency_key_hash is set'
    );
END;
