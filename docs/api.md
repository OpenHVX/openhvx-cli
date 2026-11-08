# API

Run `hvx --help` for the full command overview. Global `-o json` preserves raw API
responses for most reads and task submissions; context/init commands return their
own credential-free JSON summaries. Text mode provides tables and task details.

## Route mapping

| Command | HTTP API |
| --- | --- |
| `init --mode tenant` | POST `/api/v1/tenant/auth/login`, then `/auth/pats` under the same tenant prefix |
| `init --mode admin` | POST `/api/v1/admin/auth/login`, then `/auth/pats` under the same admin prefix |
| `get resources`, `get disks`, `get vms` | Tenant resources or admin tenant resources; disks filter storage resources |
| `get images`, `describe image` | Images under the selected API prefix; `--resolve` calls the resolve endpoint |
| `get quotas` | Tenant quotas or admin `/tenants/{tenantId}/quotas` |
| `get tenants`, `create tenant`, `delete tenant` | Admin `/tenants` routes |
| `create user` | Admin POST `/auth/tenant/register` |
| `create quota-recalc` | Admin POST `/tenants/{tenantId}/quotas/recalculate` |
| `create disk`, `delete disk`, `create vm`, `delete vm` | Admin POST `/api/v1/admin/tasks` |
| `create task --file FILE` | POST `/api/v1/{tenant|admin}/tasks` |
| `get task`, `describe task`, `wait task` | GET `/api/v1/{tenant|admin}/tasks/{taskId}` |

## Generic task submission

`create task --file FILE` accepts a JSON object with `action`, `target`, and `data`.
Use `--file -` to read stdin. Files are limited to 1 MiB. The backend validates the
full action-specific schema and authorization.

In admin mode, `tenantId` can come from the file, `--tenant`, or the configured
current tenant. Conflicting file/flag tenants are rejected. In tenant mode, omit
`tenantId`; authentication establishes tenant identity.

## Convenience mutations

- `create disk` requires `--name`, `--image-id`, and a positive `--size-mb`.
- `delete disk` requires `--ref-id`; `--agent-id` can identify the storage connector.
- `create vm` requires `--name`, `--ram`, positive `--cpu`, and `--disk-id`.
  It also supports generation, HA, host selection, and cloud-init options.
- `delete vm` accepts `--ref-id` or `--guid` as the canonical Hyper-V GUID.
  If both are provided, they must identify the same VM. `--force` requests force stop.
- Task mutations require `--idempotency-key`. Other admin changes are not covered by
  the task idempotency contract; inspect server state before retrying an uncertain
  tenant creation, user creation, or deletion.

Most command flags must precede positional arguments because parsing uses Go's
standard flag package. For example, use `describe image --resolve IMAGE_ID`.
`wait task TASK_ID` explicitly supports options following that task ID.
