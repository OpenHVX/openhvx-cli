# OpenHVX CLI

`hvx` is a Go command-line client for the OpenHVX tenant and admin HTTP APIs.
It manages contexts, lists resources, and submits and inspects asynchronous tasks.
The backend builds the internal agent protocol envelopes.

**Status:** active development. HTTP behavior is tested with local test servers;
validation against a complete deployed OpenHVX environment remains necessary.

## Documentation

| Guide | Contents |
| --- | --- |
| [Architecture](docs/architecture.md) | Client boundaries, safety rules, and limitations |
| [Configuration](docs/configuration.md) | Endpoints, credentials, environment, and migration |
| [Development](docs/development.md) | Builds, tests, and CI |
| [Task protocol](docs/task-protocol.md) | HTTP idempotency, accepted tasks, and polling |
| [API](docs/api.md) | Commands and HTTP route mapping |

## Getting started

Use Go 1.25 or newer. From `src`:

```bash
go build -o ../bin/hvx ./cmd/hvx
```

Run `bin/hvx --help`, then follow the [configuration guide](docs/configuration.md).
Use a patched Go toolchain when building distributable binaries.

Example task submission after initialization:

```bash
bin/hvx --idempotency-key start-vm-001 create task --file examples/task.example.json
bin/hvx get task --id TASK_ID
bin/hvx wait task TASK_ID --timeout 5m
```

Replace sample identifiers with resources owned by your authenticated tenant.
Keep the same key and task content when retrying an uncertain submission.
