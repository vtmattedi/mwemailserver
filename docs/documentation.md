# MW Email Service

**MW Email Service** is the centralized email gateway used by MattediWorks applications.

Production base URL:

```text
https://mailgo.mattediworks.com
```

The service sits between application projects and external email providers. Resend is the initial provider, but client applications integrate with **MW Email Service**, not directly with Resend.

## 1. Purpose

MW Email Service provides a single place to handle:

- project authentication
- centralized email usage tracking
- idempotent email sending
- provider abstraction
- delivery-status tracking
- provider webhook handling
- project quotas
- administrative visibility
- future provider replacement without changing client applications

High-level flow:

```text
Application
    |
    | HTTPS
    v
MW Email Service
    |
    | provider adapter
    v
Resend
```

Client applications should consider MW Email Service to be their email provider.

## 2. Production Endpoint

```text
https://mailgo.mattediworks.com
```

The service is deployed behind StackPort/Nginx.

Typical reverse-proxy block:

```nginx
location / {
    proxy_pass http://127.0.0.1:<service-port>;
    proxy_http_version 1.1;

    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

Use HTTPS in production.

## 3. Health Check

```http
GET /health
```

Example:

```bash
curl https://mailgo.mattediworks.com/health
```

Successful response:

```json
{
  "status": "ok"
}
```

## 4. Send Email API

Endpoint:

```http
POST /v1/emails
```

Production URL:

```text
https://mailgo.mattediworks.com/v1/emails
```

Required headers:

```http
Authorization: Bearer <project-api-key>
Idempotency-Key: <unique-idempotency-key>
Content-Type: application/json
```

Example project API key:

```text
mw_live_...
```

Each application/project receives its own API key. The raw API key is only shown when it is created or rotated. MW Email Service stores only its SHA-256 hash.

## 5. Request Body

Current request format:

```json
{
  "from": "Example App <no-reply@example.com>",
  "to": [
    "recipient@example.com"
  ],
  "subject": "Example email",
  "html": "<p>Hello</p>",
  "text": "Hello"
}
```

Fields:

| Field       |    Required | Description                                       |
| ----------- | ----------: | ------------------------------------------------- |
| `from`    |         yes | Sender address, optionally including display name |
| `to`      |         yes | Array containing one or more recipient addresses  |
| `subject` |         yes | Email subject                                     |
| `html`    | conditional | HTML body                                         |
| `text`    | conditional | Plain-text body                                   |

At least one of `html` or `text` must be present.

Client integrations should not depend on provider-specific request properties unless MW Email Service explicitly adds them to its API.

## 6. Successful Send Response

Example:

```json
{
  "id": "provider-email-id",
  "replayed": false
}
```

Fields:

| Field        | Description                                                          |
| ------------ | -------------------------------------------------------------------- |
| `id`       | Email ID returned by the active upstream provider                    |
| `replayed` | `true` when this request reused an already-known idempotent result |

A successful synchronous response means the upstream provider accepted the email request. It does **not** necessarily mean the email was delivered. Delivery is tracked separately through provider webhooks.

## 7. Idempotency

`Idempotency-Key` is required for email sends.

Example:

```http
Idempotency-Key: appointment-confirmation-84192
```

The key should identify one logical email operation.

Good examples:

```text
appointment-confirmation-84192
invoice-2026-000431
password-reset-3e7b56c8
session-reminder-9127-2026-08-12
```

Expected behavior:

```text
same project + same key + same request
    -> reuse the same logical operation/result

same project + same key + different request
    -> HTTP 409 Conflict
```

Different projects may safely use the same idempotency key because idempotency is project-scoped.

MW Email Service stores:

```text
SHA-256(Idempotency-Key)
SHA-256(normalized email request)
```

The raw idempotency key does not need to be persisted.

## 8. Retry Rules

When retrying the same logical email, always reuse the **same `Idempotency-Key`**.

Suggested behavior:

- `2xx`: success
- `400`: invalid request; do not retry automatically
- `401`: invalid/disabled project API key; do not retry automatically
- `409`: idempotency conflict/protected retry issue; surface clearly
- `429`: quota/rate limit; retry according to policy
- `5xx`: bounded retry with exponential backoff and the same idempotency key
- network timeout/connection failure: bounded retry with the same idempotency key

Never turn a retry into a new logical send by silently generating a different key.

## 9. Project Authentication

Project API authentication uses:

```http
Authorization: Bearer mw_live_...
```

Do not:

- commit project API keys to Git
- expose them in browser/client-side JavaScript
- include them in public logs
- reuse one project's key for unrelated projects

Recommended client configuration:

```env
MW_EMAIL_BASE_URL=https://mailgo.mattediworks.com
MW_EMAIL_API_KEY=mw_live_...
```

## 10. Provider Abstraction

The current upstream provider is Resend.

Applications should **not** integrate with Resend directly when MW Email Service is being used.

Correct:

```text
App -> MW Email Service -> Resend
```

Avoid:

```text
App -> Resend
```

Internally, the Go server uses an email-provider abstraction similar to:

```go
type Provider interface {
    Send(
        ctx context.Context,
        message Message,
        idempotencyKey string,
    ) (*SendResult, error)
}
```

Future upstream providers can be added without changing client applications.

## 11. Persistence

SQLite is currently used as the service database.

Typical production path:

```text
/app/data/gateway.db
```

The Docker deployment must persist `/app/data`.

`data/.gitkeep` is intentionally empty. Its only purpose is to keep the local `data/` directory in Git.

## 12. Database Concepts

### Projects

A project represents one application/client of MW Email Service.

Relevant data:

- name
- API token hash
- enabled status
- optional monthly limit
- timestamps

### Email attempts

Relevant data:

- project
- provider
- provider email ID
- sender
- subject
- recipient count
- synchronous send status
- asynchronous delivery status
- idempotency-key hash
- request hash
- error information
- timestamps

MW Email Service intentionally does not store the complete HTML/text body by default.

### Email events

Provider webhook events are stored separately.

Examples:

```text
email.sent
email.delivered
email.delivery_delayed
email.bounced
email.failed
email.suppressed
email.complained
email.opened
email.clicked
```

Engagement events such as `opened` and `clicked` are stored as events and do not replace delivery state.

## 13. Send Status vs Delivery Status

Synchronous provider status:

```text
pending
sent
failed
```

`sent` means the upstream provider accepted the operation.

Asynchronous delivery status:

```text
sent
delivered
delivery_delayed
bounced
failed
suppressed
complained
```

These values are updated from trusted provider webhooks.

## 14. Resend Webhook

Endpoint:

```http
POST /webhooks/resend
```

Production URL:

```text
https://mailgo.mattediworks.com/webhooks/resend
```

This endpoint is public because Resend must call it.

It is not protected by:

- project API-key authentication
- admin authentication
- CSRF
- admin-origin concealment

Instead, it verifies the Resend/Svix signature using:

```env
RESEND_WEBHOOK_SECRET=whsec_...
```

A valid processed webhook returns:

```text
204 No Content
```

An invalid signature normally returns:

```text
400 Bad Request
```

Webhook events are deduplicated using the provider/Svix event ID, and provider timestamps are kept so older out-of-order events do not overwrite newer delivery state.

## 15. Admin API

The admin dashboard is intentionally a **separate repository/application**. MW Email Service contains only the server-side admin API.

Admin routes live under:

```text
/admin/api/*
```

Current routes:

```text
POST /admin/api/auth/login
GET  /admin/api/auth/me
POST /admin/api/auth/csrf
POST /admin/api/auth/logout

GET   /admin/api/health
GET   /admin/api/docs
GET   /admin/api/docs/{version}
GET   /admin/api/projects
POST  /admin/api/projects
PATCH /admin/api/projects/{id}
DELETE /admin/api/projects/{id}
POST  /admin/api/projects/{id}/rotate-key
GET   /admin/api/projects/{id}/request-domains
PUT   /admin/api/projects/{id}/request-domains

GET /admin/api/emails
GET /admin/api/emails/{id}
GET /admin/api/emails/by-project
```

## 16. Admin Route Concealment

Configuration:

```env
ADMIN_ORIGIN=https://admin.example.com
ADMIN_DEV_ORIGIN=http://localhost:5173

CHECK_SP_HOST=true
SP_HOST=mailgo.mattediworks.com
```

When `CHECK_SP_HOST=true`, an admin request must satisfy both:

```text
Host == SP_HOST
Origin == ADMIN_ORIGIN or ADMIN_DEV_ORIGIN
```

Otherwise the server returns the same ordinary response as an unknown route:

```text
404 page not found
```

This is **concealment only**. It does not replace authentication or CSRF protection.

StackPort/Nginx must preserve:

```nginx
proxy_set_header Host $host;
```

## 17. Admin Authentication

Admin accounts are separate from projects.

```text
project identity -> machine-to-machine API authentication
admin identity   -> human dashboard authentication
```

Admin passwords use Argon2id.

Admin sessions use random high-entropy tokens. The browser receives the raw session token only as an HttpOnly cookie; SQLite stores only its SHA-256 hash.

Production session cookie properties:

```text
HttpOnly
Secure
SameSite=Strict
Path=/admin/api
```

## 18. CSRF Protection

State-changing authenticated admin requests require:

```http
X-CSRF-Token: ...
```

SQLite stores only:

```text
SHA-256(csrf-token)
```

After an SPA reload, the admin frontend can rotate/recover an in-memory CSRF token through:

```http
POST /admin/api/auth/csrf
```

That route still requires a valid admin session and accepted admin origin/host concealment checks.

## 19. Creating an Admin

Local:

```bash
go run ./cmd/create-admin
```

Production Docker:

```bash
docker compose run --rm --entrypoint /app/create-admin gateway
```

Default plaintext admin credentials are intentionally **not** stored in `.env`.

## 20. Environment Variables

Representative production configuration:

```env
APP_ENV=production
PORT=8080

DATABASE_PATH=/app/data/gateway.db

RESEND_API_KEY=re_...
RESEND_WEBHOOK_SECRET=whsec_...

ADMIN_ORIGIN=https://admin.example.com
ADMIN_DEV_ORIGIN=

ADMIN_AUTH_MODE=hybrid
MW_IDENTITY_INTERNAL_BASE_URL=http://mw-identity:7100
MW_IDENTITY_INTROSPECTION_CLIENT_ID=<mw-mail-resource-client-id>
MW_IDENTITY_INTROSPECTION_CLIENT_SECRET=<secret>
MW_IDENTITY_AUDIENCE=mw-mail
MW_IDENTITY_HTTP_TIMEOUT_SECONDS=5

CHECK_SP_HOST=true
SP_HOST=mailgo.mattediworks.com
```

For the admin federation modes, Identity introspection contract, and route-to-scope mapping, see [Federated admin authentication](federation.md).

Client applications normally only need:

```env
MW_EMAIL_BASE_URL=https://mailgo.mattediworks.com
MW_EMAIL_API_KEY=mw_live_...
```

## 21. StackPort Deployment

Conceptual deployment:

```text
Internet
    |
    | HTTPS
    v
StackPort / Nginx
    |
    | reverse proxy
    v
MW Email Service
    |
    +---- SQLite persistent volume
    |
    +---- Resend API
```

The internal Go port does not need to be publicly exposed when Nginx is the public entry point.

## 22. Client Integration Example

```bash
curl -X POST   https://mailgo.mattediworks.com/v1/emails   -H "Authorization: Bearer $MW_EMAIL_API_KEY"   -H "Idempotency-Key: appointment-confirmation-84192"   -H "Content-Type: application/json"   -d '{
    "from": "My App <no-reply@example.com>",
    "to": ["user@example.com"],
    "subject": "Appointment confirmation",
    "html": "<p>Your appointment is confirmed.</p>"
  }'
```

TypeScript example:

```ts
type SendEmailInput = {
  from: string;
  to: string[];
  subject: string;
  html?: string;
  text?: string;
};

type SendEmailResult = {
  id: string;
  replayed: boolean;
};

export async function sendEmail(
  email: SendEmailInput,
  idempotencyKey: string,
): Promise<SendEmailResult> {
  const response = await fetch(
    `${process.env.MW_EMAIL_BASE_URL}/v1/emails`,
    {
      method: "POST",
      headers: {
        Authorization: `Bearer ${process.env.MW_EMAIL_API_KEY}`,
        "Idempotency-Key": idempotencyKey,
        "Content-Type": "application/json",
      },
      body: JSON.stringify(email),
    },
  );

  if (!response.ok) {
    throw new Error(
      `MW Email Service returned ${response.status}`,
    );
  }

  return response.json();
}
```

## 23. Integration Rules

When adding MW Email Service to another project:

1. Treat MW Email Service as the project's email provider.
2. Do not expose the project API key to browsers.
3. Send through `/v1/emails`.
4. Always provide an idempotency key.
5. Reuse the same idempotency key for retries of the same logical email.
6. Do not infer successful delivery from the synchronous send response.
7. Do not bypass MW Email Service by sending directly to Resend.
8. Keep provider-specific behavior inside MW Email Service.
9. Use HTTPS in production.
10. Keep application email-provider code behind a local abstraction where practical.

## 24. Service Identity

Service name:

```text
MW Email Service
```

Production API hostname:

```text
mailgo.mattediworks.com
```

Repository/module identity:

```text
github.com/vtmattedi/mw-email-server
```

These names serve different purposes:

- **MW Email Service** — product/service name
- **mailgo.mattediworks.com** — production deployment URL
- **mw-email-server** — Go repository/module name
