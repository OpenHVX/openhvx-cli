# Architecture

The executable loads its local configuration, selects a tenant or admin endpoint,
and sends authenticated HTTP requests to the gateway. Backend services perform
permission checks, idempotent admission, dispatch, and host-result reconciliation.

| Component | Responsibility |
| --- | --- |
| `src/cmd/hvx` | Process entry, help, errors, and exit status |
| `src/internal/cli` | Commands, local validation, tables, JSON output, and task polling |
| `src/internal/api` | HTTP requests, payloads, bearer authentication, and response parsing |
| `src/internal/config` | Configuration loading, legacy migration, and private file replacement |

## Request safety

The active mode selects its own PAT and endpoint. Changing an endpoint through the
configuration setters clears that endpoint's credential. Legacy single-PAT files
migrate the token only into their saved mode.

Task commands create durable local receipts before sending. Explicit retry preserves
body bytes, key, endpoint, and credentials; new invocations represent new operations.
`internal/requests` owns receipt persistence and per-request file locks.

HTTP clients verify TLS by default, use a 10-second request timeout, refuse redirects,
reject embedded URL credentials, and bound response reads to 16 MiB. Waiting adds
an overall context deadline and never issues a task submission.

## Known limitations

The CLI is a client, not an authorization boundary. Backend validation remains
necessary even when local validation succeeds. Tests use simulated HTTP endpoints;
they do not run a complete gateway, backend, RabbitMQ, and host-agent deployment.

VM/disk task mutations and generic file submission support tenant and admin APIs. Not every backend
endpoint has a dedicated CLI command.

Credentials and named contexts are stored in local JSON, not an OS credential vault.
Receipts retain exact payload bytes, which may contain sensitive cloud-init data. Unix saves
use private file permissions; Windows access protection depends on filesystem ACLs.
Configuration replacement avoids partial JSON files but does not merge simultaneous
updates from multiple CLI processes. Avoid concurrent context/init changes.
