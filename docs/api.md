# API

`hvx` follows resource-oriented command conventions. Flags can appear before or
after positional arguments. `-o json` selects machine-readable responses; receipt
and waiting diagnostics go to stderr.

## Resources

```bash
hvx get vms
hvx get disks
hvx get resources
hvx describe vm web01
hvx describe disk data01
hvx create vm web01 --ram 4GB --cpu 2 --disk-id DISK_REF
hvx create disk data01 --image-id IMAGE_ID --size-mb 4096
hvx start vm web01 --wait
hvx stop vm web01
hvx restart vm web01
hvx delete vm web01 --agent-id host-1
hvx delete disk data01
```

Names and references resolve against current tenant inventory. Multiple matches
fail before submission; use the canonical reference and `--agent-id` to disambiguate.
Resolved VM mutations require canonical GUIDs. The saved request retains that
identity across retry, even if the resource name later changes.

Task mutations support tenant and admin modes. Admin mode requires `--tenant` or
the configured tenant; tenant mode derives identity from authentication. Convenience
creation supports host selection, HA, generation, and cloud-init options. Legacy
flag-only deletion (`--ref-id`, `--guid`) remains available.

## Files and asynchronous operations

```bash
hvx create -f task.json
hvx create task --file task.json
hvx retry REQUEST_ID
hvx get requests
hvx get tasks
hvx get task TASK_ID
hvx wait task TASK_ID --timeout 5m --interval 2s
```

Files contain the HTTP `action`, `target`, and `data`, not Kubernetes manifests or
AMQP envelopes. `--file -` reads stdin. In admin mode, `tenantId` comes from the file,
`--tenant`, or current context; conflicting file/flag values are rejected.

`get requests` lists local receipts without dumping payloads or credentials.
`get tasks` queries locally known accepted task IDs. There is currently no global
task-list or execution-log endpoint exposed by the backend, so no `logs` or `apply`
command is provided. See [task semantics](task-protocol.md).

## Contexts

```bash
hvx init --context lab --mode tenant --tenant-server URL --username EMAIL --password-stdin
hvx init --context prod --mode admin --admin-server URL --username EMAIL --password-stdin
hvx config get-contexts
hvx config use-context lab
hvx config current-context
hvx context show
hvx context save NAME
```

`context set-mode` and `context set-tenant` operate within the selected context.
A new named context starts without inherited credentials. Context lists and details
never print tokens.

## Other routes

| Command | HTTP API |
| --- | --- |
| `init` | Selected `/api/v1/{tenant|admin}/auth/login`, then `/auth/pats` |
| `get resources`, `get disks`, `get vms` | Tenant resources or admin tenant resources |
| `get images`, `describe image` | Images under the selected API prefix; optional resolve endpoint |
| `get quotas` | Tenant quotas or admin `/tenants/{tenantId}/quotas` |
| `get tenants`, `create tenant`, `delete tenant` | Admin `/tenants` routes |
| `create user` | Admin `/auth/tenant/register` |
| `create quota-recalc` | Admin `/tenants/{tenantId}/quotas/recalculate` |
| Task mutations | POST `/api/v1/{tenant|admin}/tasks` |
| Task inspection and waiting | GET `/api/v1/{tenant|admin}/tasks/{taskId}` |

Tenant/user administration is outside the task idempotency contract. Inspect
server state before retrying an uncertain non-task mutation.
