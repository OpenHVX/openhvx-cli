# Repository context

This is the OpenHVX CLI repository. Its Go module lives under `src`, with the
executable in `src/cmd/hvx`. Backend, host agents, and deployment infrastructure
belong to separate repositories. Start with [README.md](README.md).

Maintain English documentation under `docs/` with lowercase, topic-based filenames.
Use `examples/` for configuration and HTTP task examples, never real credentials.

Core constraints:

- `internal/cli` parses commands and formats output; `internal/api` owns HTTP requests.
- `internal/config` persists contexts and credentials; tenant and admin tokens stay separate.
- Every task submission requires an explicit stable idempotency key.
- HTTP acceptance is not host completion; waiting only reads task status.
- The CLI does not publish AMQP messages or construct agent protocol v2 envelopes.
- TLS verification is enabled by default; HTTP redirects are not followed.
- Do not print stored credentials or forward them to an endpoint after changing its URL.

See [checks](docs/development.md#checks) and
[known limitations](docs/architecture.md#known-limitations).
