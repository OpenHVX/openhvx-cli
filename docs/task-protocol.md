# Task protocol

The CLI submits HTTP task requests. Protocol v2 between the backend and executors
is implemented by those services; the CLI neither constructs `commandBody` nor
calculates `commandFingerprint`.

## Submit and retry

Every task submission requires `--idempotency-key KEY`, including convenience VM
and disk commands. Choose a stable key for one logical operation and retain it
until the outcome is known.

```bash
bin/hvx --idempotency-key start-vm-001 create task --file examples/task.example.json
```

The client sends the key as the HTTP `Idempotency-Key` header. On a connection
failure or timeout, acceptance may already have committed. Retry the same request
with the same key, credentials, API mode, and tenant. Do not invent a fresh key to
recover an uncertain submission. The backend returns the original accepted task or
rejects changed content with HTTP 409.

Changing admin tenant context or credentials can change the server's idempotency
scope. Keep them stable across retries. JSON task files contain the HTTP payload,
not the internal AMQP envelope; see [the example](../examples/task.example.json).

## Acceptance and execution

HTTP 202 means durable acceptance, not successful execution. Use the returned task
ID to inspect progress:

```bash
bin/hvx get task --id TASK_ID
bin/hvx describe task TASK_ID
bin/hvx wait task TASK_ID --timeout 5m --interval 2s
```

`queued` and `sent` describe pending execution. `reconciling` means the host outcome
is uncertain and is not treated as success. `done` is successful completion; `error`
is terminal failure. Waiting uses only GET requests and exits nonzero on failure,
timeout, or a polling error. It never submits the task again. Its terminal output is
the API task JSON, including when the task failed.

`get task` shows execution outcome and outbox delivery separately when present.
Transport delivery is not a substitute for host completion. Backend reservations
and locks for uncertain operations cannot be resolved by forcing a CLI retry.
