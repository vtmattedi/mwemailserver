# MW Email Server

Centralized email gateway for MattediWorks services.

## Stack

- Go
- SQLite
- Resend
- HTTP API
- Project API-key authentication

## Admin federation

MW Email supports local, hybrid, and federated admin authentication. In hybrid or federated mode, MW BFF can call `/admin/api/*` with a short-lived opaque delegated token for audience `mw-mail`; MW Email validates it through authenticated MW Identity introspection and enforces a route-specific `mail.*` scope.

See [Federated admin authentication](docs/federation.md) for configuration, scope mapping, rollout behavior, and the error contract. The project-key authenticated `POST /v1/emails` API is unchanged.
