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

Common commands after initialization:

```bash
bin/hvx get vms
bin/hvx start vm web01 --wait
bin/hvx create -f examples/task.example.json
bin/hvx get requests
bin/hvx retry REQUEST_ID
bin/hvx get tasks
```

Keys are generated and saved automatically before submission. Use `retry` to recover
an uncertain request; running a new task command creates a new operation.
Named contexts select environments and accounts through `hvx config use-context`.
