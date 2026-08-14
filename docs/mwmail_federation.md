# Federated admin authentication

MW Email can authenticate human administrative requests delegated by MW BFF while preserving the existing local admin flow and the project API-key email API.

## Configuration

`ADMIN_AUTH_MODE` supports:

- `local` (default): only the existing local session, Origin, and CSRF flow is accepted. A Bearer header is rejected.
- `hybrid`: a request may use either the local browser flow or a delegated Bearer token. A presented Bearer header always selects federation and never falls back to a cookie.
- `federated`: admin business routes require a delegated Bearer token. Local login/session endpoints return a deliberate non-success response.

Hybrid/federated configuration:

```env
ADMIN_AUTH_MODE=hybrid

MW_IDENTITY_INTERNAL_BASE_URL=http://mw-identity:7100
MW_IDENTITY_INTROSPECTION_CLIENT_ID=<mw-mail-resource-client-id>
MW_IDENTITY_INTROSPECTION_CLIENT_SECRET=<secret>
MW_IDENTITY_AUDIENCE=mw-mail
MW_IDENTITY_HTTP_TIMEOUT_SECONDS=5
```

The Identity URL is private service-to-service transport and is not the public issuer URL. The resource-server credentials must never be exposed to a browser.

## Request contract

MW BFF sends:

```http
Authorization: Bearer <opaque-delegated-token>
```

MW Email posts the opaque token to `<MW_IDENTITY_INTERNAL_BASE_URL>/oauth/introspect` using HTTP Basic resource-server credentials. It requires an active token, non-empty `sub`, exact audience `mw-mail`, and the route scope. It preserves `auth_time` and `amr` in the trusted admin principal.

Federated requests do not require a browser `Origin` or CSRF token. `CHECK_SP_HOST`/`SP_HOST`, when enabled, still applies. Local browser requests retain Origin concealment, session authentication, and CSRF on mutations.

## Scope mapping

| Route                                            | Required scope              |
| ------------------------------------------------ | --------------------------- |
| `GET /admin/api/health`                        | `mail.projects.read`      |
| `GET /admin/api/projects`                      | `mail.projects.read`      |
| `POST /admin/api/projects`                     | `mail.projects.write`     |
| `PATCH /admin/api/projects/{id}`               | `mail.projects.write`     |
| `GET /admin/api/projects/{id}/request-domains` | `mail.projects.read`      |
| `PUT /admin/api/projects/{id}/request-domains` | `mail.projects.write`     |
| `POST /admin/api/projects/{id}/rotate-key`     | `mail.credentials.rotate` |
| `DELETE /admin/api/projects/{id}`             | `mail.projects.delete`    |
| `GET /admin/api/emails`                        | `mail.emails.read`        |
| `GET /admin/api/emails/{id}`                   | `mail.emails.read`        |
| `GET /admin/api/emails/by-project`             | `mail.usage.read`         |
| `GET /admin/api/docs`                          | `mail.server.read`        |
| `GET /admin/api/docs/{version}`                | `mail.server.read`        |

Local-only endpoints under `/admin/api/auth/*` are not federated operations.

### Project deletion

`DELETE /admin/api/projects/{id}` requires the dedicated
`mail.projects.delete` scope. It returns `204` after atomically deleting the
project and revoking its API credential, or `404` when the exact project ID no
longer exists. Projects with retained email history are not deleted and return:

```json
{
  "code": "project_has_dependencies",
  "error": "project has retained email history"
}
```

The conflict status is `409`. Successful deletion stores an immutable audit
event with the human actor, deleted project ID and name, request ID, and UTC
timestamp.

## Errors and audit context

Authentication failures, inactive tokens, malformed Bearer headers, and Identity transport/decoding failures return `401`. An audience mismatch or missing scope returns `403`. Identity diagnostics and credentials are not returned to callers.

Successful admin operations produce structured audit log context. Federated context includes the stable human `sub`, audience, authorized route scope, `auth_time`, `amr`, request ID, method, path, and status. Raw tokens and secrets are never logged.

## Unchanged application API

Normal senders continue to call:

```http
POST /v1/emails
Authorization: Bearer mw_live_...
Idempotency-Key: ...
```

Identity delegated tokens do not replace project API keys.
