---
name: mw-email-service-provider
description: Integrate an application with MattediWorks MW Email Service as its email provider. Use when replacing direct Resend/SMTP/provider usage with the centralized MW Email Service, creating an email provider adapter, or adding server-side email sends.
---

# MW Email Service Provider Integration

Use **MW Email Service** as the application's email provider.

Production base URL:

```text
https://mailgo.mattediworks.com
```

Do not integrate the client application directly with Resend unless explicitly instructed to bypass MW Email Service.

## Goal

Implement or modify the application's email abstraction so that:

```text
Application
    |
    v
MW Email Service
    |
    v
upstream provider
```

The application should depend on the MW Email Service API contract, not on the current upstream provider.

## Required Configuration

Determine:

```text
MW_EMAIL_BASE_URL
MW_EMAIL_API_KEY
```

Recommended environment values:

```env
MW_EMAIL_BASE_URL=https://mailgo.mattediworks.com
MW_EMAIL_API_KEY=
```

Never hardcode the project API key in source code.

Never expose the project API key to browser/client-side JavaScript.

If the application has no trusted server-side environment in which a secret can be stored, stop and explain that a direct browser integration is inappropriate.

## API Contract

Send email using:

```http
POST /v1/emails
```

Production URL:

```text
https://mailgo.mattediworks.com/v1/emails
```

Required headers:

```http
Authorization: Bearer <MW_EMAIL_API_KEY>
Idempotency-Key: <stable-key-for-this-logical-email>
Content-Type: application/json
```

Request body:

```json
{
  "from": "Application <no-reply@example.com>",
  "to": [
    "recipient@example.com"
  ],
  "subject": "Subject",
  "html": "<p>Body</p>",
  "text": "Body"
}
```

Rules:

- `from` is required.
- `to` must contain at least one recipient.
- `subject` is required.
- at least one of `html` or `text` is required.
- send only fields supported by the MW Email Service contract.
- do not forward arbitrary Resend-specific payload fields.

Successful response:

```json
{
  "id": "provider-email-id",
  "replayed": false
}
```

`id` is the upstream provider email ID.

`replayed=true` means the gateway reused the known result for the same idempotent operation.

A successful send response means **accepted by the provider**, not necessarily delivered.

## Idempotency Is Mandatory

Every send request must have:

```http
Idempotency-Key: ...
```

The key identifies one logical email operation.

Good examples:

```text
appointment-confirmation-1842
invoice-2026-000431
password-reset-7fd31a58
booking-reminder-9182-2026-08-12
welcome-user-4721
```

For retrying the same logical send:

```text
REUSE the same Idempotency-Key.
```

Do not generate a new key for every retry.

Do not reuse the same key for two different payloads.

The gateway rejects:

```text
same project + same idempotency key + different request
```

with a conflict response.

## Provider Adapter Design

Prefer creating a local provider abstraction if the project does not already have one.

Go:

```go
type EmailProvider interface {
    Send(
        ctx context.Context,
        message Email,
        idempotencyKey string,
    ) (*SendResult, error)
}
```

TypeScript:

```ts
export interface EmailProvider {
  send(
    message: EmailMessage,
    idempotencyKey: string,
  ): Promise<SendEmailResult>;
}
```

Implementation names may include:

```text
MWEmailProvider
MWEmailServiceProvider
MattediWorksEmailProvider
```

Do not call the application-facing abstraction `ResendProvider` when it communicates with MW Email Service.

## TypeScript Example

```ts
export type EmailMessage = {
  from: string;
  to: string[];
  subject: string;
  html?: string;
  text?: string;
};

export type SendEmailResult = {
  id: string;
  replayed: boolean;
};

export class MWEmailProvider {
  constructor(
    private readonly baseUrl: string,
    private readonly apiKey: string,
  ) {}

  async send(
    message: EmailMessage,
    idempotencyKey: string,
  ): Promise<SendEmailResult> {
    const response = await fetch(
      `${this.baseUrl}/v1/emails`,
      {
        method: "POST",
        headers: {
          Authorization: `Bearer ${this.apiKey}`,
          "Idempotency-Key": idempotencyKey,
          "Content-Type": "application/json",
        },
        body: JSON.stringify(message),
      },
    );

    if (!response.ok) {
      const body = await response.text();

      throw new Error(
        `MW Email Service returned ${response.status}: ${body}`,
      );
    }

    return response.json();
  }
}
```

## PHP / Laravel Example

Use Laravel's HTTP client from server-side code:

```php
use Illuminate\Support\Facades\Http;

$response = Http::withToken(config('services.mw_email.key'))
    ->withHeaders([
        'Idempotency-Key' => $idempotencyKey,
    ])
    ->post(
        rtrim(config('services.mw_email.url'), '/') . '/v1/emails',
        [
            'from' => $from,
            'to' => $to,
            'subject' => $subject,
            'html' => $html,
            'text' => $text,
        ],
    );

$response->throw();

$result = $response->json();
```

Recommended Laravel config:

```php
// config/services.php

'mw_email' => [
    'url' => env(
        'MW_EMAIL_BASE_URL',
        'https://mailgo.mattediworks.com'
    ),
    'key' => env('MW_EMAIL_API_KEY'),
],
```

Environment:

```env
MW_EMAIL_BASE_URL=https://mailgo.mattediworks.com
MW_EMAIL_API_KEY=mw_live_...
```

If the Laravel project already has a mail/provider abstraction, implement MW Email Service behind that abstraction instead of scattering HTTP calls through controllers or jobs.

## Go Example

```go
type Client struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func (c *Client) Send(
	ctx context.Context,
	message EmailMessage,
	idempotencyKey string,
) (*SendResult, error) {
	body, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(c.baseURL, "/")+"/v1/emails",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Idempotency-Key", idempotencyKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf(
			"MW Email Service returned %d",
			resp.StatusCode,
		)
	}

	var result SendResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}
```

Use a bounded HTTP timeout.

## Retry Policy

Email retries must preserve idempotency.

Suggested behavior:

```text
400
    invalid request
    do not retry automatically

401
    invalid/disabled project credentials
    do not retry automatically

409
    idempotency conflict/protected retry issue
    do not silently generate a new key

429
    retry according to policy

5xx
    bounded exponential backoff
    SAME Idempotency-Key

network timeout / connection failure
    bounded exponential backoff
    SAME Idempotency-Key
```

Never convert a retry into a new logical email by generating a different idempotency key.

## Queue / Job Integration

When sending from a background job, persist or deterministically derive the idempotency key from the logical operation.

Good:

```text
invoice-email:<invoice-id>
appointment-reminder:<appointment-id>:<scheduled-date>
welcome-email:<user-id>
```

Bad:

```text
random UUID generated inside every retry attempt
```

If the job framework retries automatically, ensure the same key survives those retries.

## Migration From Direct Resend Usage

When replacing Resend in an existing project:

1. locate direct Resend SDK/API calls
2. identify or create the project's email abstraction
3. add `MW_EMAIL_BASE_URL`
4. add `MW_EMAIL_API_KEY`
5. replace direct provider calls with MW Email Service
6. add deterministic idempotency keys
7. remove the client project's dependency on the centralized Resend API key
8. remove provider-specific response assumptions where possible
9. update tests/mocks
10. verify retries reuse the same idempotency key

The client project normally should no longer require:

```env
RESEND_API_KEY=
```

The centralized MW Email Service owns the upstream provider credential.

## Security Requirements

Always:

- use HTTPS in production
- keep `MW_EMAIL_API_KEY` server-side
- redact API keys from logs
- avoid logging complete message bodies unless explicitly required
- use the project's existing secret-management mechanism
- validate input before sending
- use bounded HTTP timeouts
- use idempotency for every send
- preserve the same idempotency key across retries

Never:

- put `MW_EMAIL_API_KEY` in frontend bundles
- send it from a React/Vue/browser SPA
- use admin API credentials as project credentials
- call `/admin/api/*` to send application emails
- call `/webhooks/resend` from a client project
- depend on `ADMIN_ORIGIN`, `SP_HOST`, or route concealment as client API authentication

## Health Check

Connectivity check:

```http
GET https://mailgo.mattediworks.com/health
```

Expected response:

```json
{
  "status": "ok"
}
```

Do not use the health endpoint as proof that a project's API key is valid.

## Do Not Reimplement Server Responsibilities

Client projects should not implement:

- Resend webhook verification
- centralized delivery-event storage
- MW Email Service usage counters
- MW admin authentication
- MW admin CSRF handling
- StackPort Host concealment
- direct access to the MW Email Service SQLite database

Those belong to MW Email Service.

The client provider only needs to authenticate and send email requests.

## Testing Strategy

Unit-test the provider with an HTTP mock/stub.

Cover at minimum:

```text
successful 2xx send
replayed=true response
400 invalid request
401 invalid API key
409 idempotency conflict
500 gateway/provider failure
network timeout
```

Verify that the adapter sends:

```text
Authorization
Idempotency-Key
Content-Type
```

and the expected JSON body.

When testing retry logic, assert that retries reuse the same idempotency key.

Do not use the production service for normal unit tests.

## Acceptance Checklist

Before considering an integration complete:

- [ ] application sends only from trusted server-side code
- [ ] `MW_EMAIL_BASE_URL` is configurable
- [ ] `MW_EMAIL_API_KEY` is configurable and secret
- [ ] requests use `POST /v1/emails`
- [ ] Bearer authentication is present
- [ ] `Idempotency-Key` is always present
- [ ] retries reuse the same key
- [ ] request contains `from`, `to`, and `subject`
- [ ] request contains `html` and/or `text`
- [ ] HTTP timeout is bounded
- [ ] non-2xx responses are handled
- [ ] 409 conflicts are not silently retried with a new key
- [ ] direct Resend dependency is removed when replacing it
- [ ] API key is never exposed to client-side JavaScript
- [ ] tests/mocks cover the gateway contract

## Service Reference

Service:

```text
MW Email Service
```

Production:

```text
https://mailgo.mattediworks.com
```

Send endpoint:

```text
POST /v1/emails
```

Authentication:

```text
Authorization: Bearer mw_live_...
```

Required idempotency:

```text
Idempotency-Key: ...
```

Health:

```text
GET /health
```

Treat this contract as the stable provider boundary.
