CREATE TABLE IF NOT EXISTS email_attempts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    project_id INTEGER NOT NULL,

    -- Provider abstraction. Initially "resend".
    provider TEXT NOT NULL DEFAULT 'resend',

    -- Provider-side email identifier returned after a successful send request.
    provider_id TEXT,

    sender TEXT NOT NULL,

    -- Kept for basic audit/debugging. We intentionally do not store HTML/text bodies.
    subject TEXT NOT NULL,

    recipient_count INTEGER NOT NULL
        CHECK (recipient_count > 0),

    -- State of the synchronous API call to the provider.
    -- "accepted" means the provider accepted the send request; it does NOT mean delivered.
    send_status TEXT NOT NULL DEFAULT 'pending'
        CHECK (send_status IN (
            'pending',
            'accepted',
            'failed'
        )),

    send_error TEXT,

    -- State learned asynchronously from provider webhooks.
    -- NULL means no delivery-state webhook has been processed yet.
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

    -- Timestamp from the provider event that last changed delivery_status.
    -- This lets the webhook handler ignore older out-of-order events.
    delivery_status_at TEXT,

    -- SHA-256 of the client-provided Idempotency-Key.
    -- The raw key does not need to be persisted.
    idempotency_key_hash TEXT,

    -- SHA-256 of a deterministic/normalized representation of the send request.
    -- Same idempotency key + different request hash => conflict.
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

CREATE INDEX IF NOT EXISTS idx_email_attempts_send_status
    ON email_attempts(send_status);

CREATE INDEX IF NOT EXISTS idx_email_attempts_delivery_status
    ON email_attempts(delivery_status);

-- Fast webhook lookup by provider email ID.
CREATE UNIQUE INDEX IF NOT EXISTS idx_email_attempts_provider_email
    ON email_attempts(provider, provider_id)
    WHERE provider_id IS NOT NULL
      AND provider_id <> '';

-- Idempotency is scoped to one gateway project.
-- Different projects may use the same client-generated key safely.
CREATE UNIQUE INDEX IF NOT EXISTS idx_email_attempts_project_idempotency
    ON email_attempts(project_id, idempotency_key_hash)
    WHERE idempotency_key_hash IS NOT NULL
      AND idempotency_key_hash <> '';

-- If an idempotency key exists, a request hash must exist too.
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
