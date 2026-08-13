# MW Email Gateway — How to Implement

The gateway exposes one main endpoint for sending email:

```http
POST https://mailgo.mattediworks.com/v1/emails
```

Client applications should call this endpoint from their backend whenever possible. The project API key is the real authentication mechanism.

---

## 1. Required headers

Every request must include:

```http
Authorization: Bearer mw_live_<project-key>
Content-Type: application/json
Idempotency-Key: <unique-operation-key>
```

### Authorization

Each project has its own API key:

```http
Authorization: Bearer mw_live_...
```

Do not expose this key in public frontend code.

### Idempotency-Key

Every send operation must include an `Idempotency-Key`.

Example:

```http
Idempotency-Key: appointment-reminder-9812-2026-08-12
```

The key identifies one logical send operation.

If the same key is sent again with the same request body, the gateway returns the previous result instead of sending the email again.

If the same key is reused with a different request body, the gateway rejects the request.

A UUID is also fine:

```text
550e8400-e29b-41d4-a716-446655440000
```

Keep the key at 256 characters or fewer.

---

## 2. Request body

Example:

```json
{
  "from": "MattediWorks <no-reply@mattediworks.com>",
  "to": [
    "user@example.com"
  ],
  "subject": "Your appointment",
  "html": "<h1>Hello</h1><p>Your appointment is confirmed.</p>",
  "text": "Hello\n\nYour appointment is confirmed."
}
```

### Fields

| Field | Description |
|---|---|
| `from` | Sender address accepted by the gateway/provider configuration. |
| `to` | Array of recipient email addresses. |
| `subject` | Email subject. |
| `html` | Optional HTML body. |
| `text` | Optional plain-text body. |

Provide at least one message body: `html`, `text`, or both.

The gateway intentionally does not require applications to integrate directly with Resend. The gateway handles the upstream provider request and tracks the send attempt centrally.

---

## 3. Example request

### curl

```bash
curl -X POST "https://mailgo.mattediworks.com/v1/emails" \
  -H "Authorization: Bearer mw_live_YOUR_PROJECT_KEY" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: test-$(date +%s)" \
  -d '{
    "from": "MattediWorks <no-reply@mattediworks.com>",
    "to": ["user@example.com"],
    "subject": "Gateway test",
    "html": "<p>This message was sent through MW Email Service.</p>",
    "text": "This message was sent through MW Email Service."
  }'
```

### JavaScript / TypeScript

```ts
const response = await fetch(
  "https://mailgo.mattediworks.com/v1/emails",
  {
    method: "POST",
    headers: {
      Authorization: `Bearer ${process.env.MW_EMAIL_API_KEY}`,
      "Content-Type": "application/json",
      "Idempotency-Key": crypto.randomUUID(),
    },
    body: JSON.stringify({
      from: "MattediWorks <no-reply@mattediworks.com>",
      to: ["user@example.com"],
      subject: "Gateway test",
      html: "<p>Hello from MW Email Service.</p>",
      text: "Hello from MW Email Service.",
    }),
  },
);

const body = await response.json();

if (!response.ok) {
  throw new Error(body.error ?? `Email request failed: ${response.status}`);
}
```

---

## 4. Request-origin restriction

A project API key may optionally be restricted to specific HTTP `Origin` domains.

Examples of project rules:

```text
*
*.mydomain.com
a.com,b.com
mydomain.com,*.mydomain.com
```

Meaning:

```text
*                            unrestricted
*.mydomain.com               subdomains only
a.com,b.com                  exact a.com OR b.com
mydomain.com,*.mydomain.com  apex + all subdomains
```

For restricted keys, the request must include a matching `Origin` header.

Browser requests provide this automatically.

For example:

```http
Origin: https://app.mydomain.com
```

The API key remains the actual authentication mechanism. Origin checking is an additional abuse-control layer.

A restricted key used without an `Origin` header is rejected.

---

## 5. Responses

The gateway always returns an HTTP status code indicating the result.

### Successful send

A successful send returns a `2xx` response with the gateway result.

Example:

```json
{
  "id": "01J...",
  "replayed": false
}
```

`replayed: false` means the gateway processed this idempotency key as a new send.

### Idempotent replay

If the same `Idempotency-Key` is submitted again with the same request:

```json
{
  "id": "01J...",
  "replayed": true
}
```

The gateway does not send a duplicate email.

### Invalid or missing API key

```http
401 Unauthorized
```

Example:

```json
{
  "error": "unauthorized"
}
```

### Request origin not allowed

For a key restricted to specific request domains:

```http
403 Forbidden
```

```json
{
  "error": "request origin is not allowed"
}
```

If the key is restricted and no `Origin` header is present:

```http
403 Forbidden
```

```json
{
  "error": "origin header required for this api key"
}
```

### Invalid request

Malformed JSON, invalid email data, missing required fields, or another request-validation failure returns a `4xx` response.

Example:

```json
{
  "error": "..."
}
```

### Idempotency conflict

If an existing `Idempotency-Key` is reused with a different request body:

```http
409 Conflict
```

The client must not silently retry with the conflicting body. Use a new idempotency key for a new logical send operation.

### Provider / gateway failure

If the gateway cannot complete the upstream send, it returns a non-`2xx` response.

Example:

```json
{
  "error": "..."
}
```

Clients should treat any non-`2xx` response as a failed request.

---

## 6. Retry behavior

For network errors or temporary failures, retry the request using the **same `Idempotency-Key`**.

Do not generate a new key for a retry of the same logical email.

Correct:

```text
Attempt 1
Idempotency-Key: appointment-123
        ↓ timeout

Attempt 2
Idempotency-Key: appointment-123
```

Incorrect:

```text
Attempt 1
Idempotency-Key: random-A
        ↓ timeout

Attempt 2
Idempotency-Key: random-B
```

Using a new key could cause a duplicate email if the first request actually reached the gateway.

---

## 7. Recommended integration pattern

```text
Application backend
        │
        │ POST /v1/emails
        │ Bearer project API key
        │ Idempotency-Key
        ▼
MW Email Gateway
        │
        ├── authenticate project
        ├── check optional request-origin rule
        ├── enforce idempotency
        ├── validate request
        ├── send through provider
        └── centrally track the attempt
```

Store the project API key in the application's server-side environment:

```env
MW_EMAIL_API_KEY=mw_live_...
MW_EMAIL_API_URL=https://mailgo.mattediworks.com
```

Do not commit the API key to source control and do not expose it in browser bundles.
