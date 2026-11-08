# Task protocol

The CLI submits HTTP tasks. The backend builds internal protocol v2 envelopes and
coordinates authorization, resource locks, quotas, dispatch, and host outcomes.

## Submit and retry

Each new task command represents a new operation. The CLI generates a random
request ID and idempotency key, saves the resolved HTTP body before sending it,
and prints a receipt to stderr:

```bash
hvx start vm web01
# Request: REQUEST_ID (retry: hvx retry REQUEST_ID)
```

After a timeout, lost connection, or uncertain response, recover that exact request:

```bash
hvx get requests
hvx retry REQUEST_ID
```

Retry uses the saved bytes and key, not current flags, name resolution, or the
original file. Editing the file cannot change a recorded request. Restore the same
endpoint, API mode, credential, and TLS settings before retrying. The tenant and
resolved resource remain those in the saved payload. Accepted receipts replay the
saved response locally without another POST.

Running `start vm web01` again is a new intentional operation, with a fresh identity,
even if the preceding command timed out. Use `retry` to recover an uncertain request.
`--idempotency-key KEY` remains an advanced override for scripts; CLI receipts are
still saved. Reusing a key with different content is rejected by the backend.

## Local persistence

Receipts live in `requests/` beside the file selected by `HVX_CONFIG`, or under
`~/.config/hvx/requests` by default. They retain the complete HTTP body and accepted
response, but no bearer token. Credential digests bind replay to the original
credential. The body is base64 encoded to preserve exact bytes; it is not encrypted
and can contain sensitive cloud-init content. Protect this directory and backups.

Records use private files, temporary-file replacement, flushed writes, and per-request
file locks. Unix also syncs directory entries. Keep state on a local filesystem
supporting file locks and atomic replacement. Windows protection depends on ACLs;
power-loss guarantees depend on the platform and filesystem. Corrupt payloads are
refused. Concurrent retries of the same receipt are rejected while one is active.

There is no automatic receipt expiry. Do not delete uncertain receipts to force
re-execution. The lower-level HTTP adapter also retains a key-only pending cache
for direct callers without CLI receipts; ordinary CLI task commands use receipts.

## Acceptance and execution

HTTP 202 means acceptance, not completed host execution:

```bash
hvx create -f task.json --wait --timeout 5m
hvx get task TASK_ID
hvx describe task TASK_ID
hvx wait task TASK_ID --timeout 5m
hvx get tasks
```

`queued`, `sent`, and `reconciling` are not success. Waiting performs GET requests
until `done` or `error`, failing on terminal error, timeout, or polling failure.
It never resubmits. Terminal wait output is task JSON; diagnostics and request/task
IDs go to stderr so `-o json` remains usable in scripts.

`get tasks` queries only tasks recorded locally for the current endpoint and
credential. It is not a global server-side task history. Outbox delivery and host
execution outcome remain separate in task details. CLI retries cannot resolve an
unknown host effect or bypass backend reservations.
