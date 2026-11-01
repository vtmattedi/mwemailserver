# MW Mail

A Go and SQLite email gateway that lets applications send through Resend using their own project API keys. It centralizes provider credentials, send-attempt history, webhook events and administration for the MattediWorks apps.

Resend handles email delivery. MW Mail provides the application-facing HTTP API and project boundaries; it is not an SMTP server or an email inbox.

[Ecosystem case study](https://mwecosystem.mattediworks.com/) · [Application integration](docs/how_to_implement.md) · [API reference](internal/apidocs/docs/v1.md)

## How it fits together

```text
Application backend -- project API key --> MW Mail -- provider credential --> Resend
                                             ^                                 |
                                             +-------- signed webhook ---------+

MW Admin --> MW BFF -- delegated admin token --> MW Mail --> MW Identity introspection
```

- `POST /v1/emails` authenticates a project and requires an idempotency key.
- SQLite records send attempts and provider events. Idempotency is scoped to the project and checked against the request contents.
- Resend webhooks update delivery-event history after signature verification when the webhook verifier is configured.
- The admin API manages projects, key rotation, request-domain policies, email history and usage.
- Shared administration comes from [MW Control Plane](https://github.com/vtmattedi/mw-plane-control). This repository contains the service, not the shared admin SPA.

## Run locally

Requires the Go version declared in [go.mod](go.mod) (currently 1.26.5), a writable SQLite data directory, and a Resend account/API key for real sends.

From the repository root, copy the example configuration:

```sh
cp .env.example .env
```

On PowerShell, use `Copy-Item .env.example .env`. Edit the copy before starting:

| Variable | Local setup |
| --- | --- |
| `APP_ENV` | `development` |
| `PORT` | `8080`, as in the example |
| `DATABASE_PATH` | `./data/gateway.db`, or another writable path |
| `RESEND_API_KEY` | Your provider API key; required at startup |
| `RESEND_WEBHOOK_SECRET` | Your Resend webhook signing secret when receiving provider events |
| `ADMIN_AUTH_MODE` | `local` for an independent local setup; default if omitted |
| `ADMIN_ORIGIN` / `ADMIN_DEV_ORIGIN` | Exact origins of the admin client you use; the example's production origin is a placeholder |
| `CHECK_SP_HOST` | Set `false` for direct localhost requests; the example enables a deployment-specific host check |

`ADMIN_API_KEY` in the existing example is a legacy field; the current server configuration does not use it for admin authentication. Do not treat it as a bootstrap credential.

```sh
go run ./cmd/server
```

The server loads `.env`, opens SQLite and applies migrations. Check `GET http://localhost:8080/health`.

### Create a project and send

In a separate terminal at the same repository root:

```sh
go run ./scripts/create-project.go "Local development"
```

The command writes a project to the configured database and displays its API key once; only its hash is stored. Keep the key in the calling backend's configuration. Follow [Application integration](docs/how_to_implement.md) for the request body, headers, sender setup and response handling. A real send requires a sender accepted by your Resend account.

For the existing standalone local admin flow, `go run ./cmd/create-admin` prompts for an email and password. These bootstrap commands modify the database selected by `DATABASE_PATH`; use a separate local database when exploring the service.

## Shared administration

`ADMIN_AUTH_MODE` selects the existing authentication path:

| Mode | Admin API authentication |
| --- | --- |
| `local` | Local session, allowed Origin and CSRF protection |
| `hybrid` | Local flow or a delegated bearer token; a supplied bearer token never falls back to a cookie |
| `federated` | Delegated bearer tokens from MW Identity; local login is disabled |

In hybrid/federated mode, configure an Identity resource-server client with audience `mw-mail`. Mail introspects each delegated token and enforces the route's `mail.*` scope. Each resource server needs its own introspection credentials. This does not change project-key authentication for sending email.

See [Federated admin authentication](docs/mwmail_federation.md) for configuration and scope contracts.

## Deployment and persistence

[docker-compose.yml](docker-compose.yml) builds the service, sets container port `8082`, and mounts `./data` at `/app/data`. It exposes that port to the container network without publishing a host port. Route your reverse proxy or Stackport service target to `gateway:8082`; the direct Go example above uses `8080` instead.

Keep the SQLite data directory persistent. Configure HTTPS, the allowed hosts/origins, `APP_ENV` and webhook verification for your deployment. Do not commit `.env`, provider credentials, project keys or runtime databases.

## Behavior and limits

- Provider acceptance is not proof of delivery to the recipient's inbox. Delivery events arrive separately through webhooks.
- Repeating a completed request with the same project/key and body returns the stored result. Reusing the key for a different body conflicts. Uncompleted attempts cannot be retried through the service after the 24-hour provider idempotency window.
- Project keys are backend credentials. Request-domain rules and CORS do not make embedding a secret in a public frontend safe.
- This service was built for the MW applications. The tests and implementation are available for inspection; no independent security audit is claimed.

## Development

```sh
go test ./...
```

| Path | Purpose |
| --- | --- |
| `cmd/` | Server and local admin bootstrap commands |
| `internal/` | HTTP API, authentication, federation, email provider and persistence |
| `migrations/` | SQLite schema migrations |
| `docs/` | Integration, federation and implementation notes |

See [Service documentation](docs/documentation.md) for further implementation details.

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE). Third-party dependencies retain their respective licenses.
