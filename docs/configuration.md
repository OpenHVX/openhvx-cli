# Configuration

The default path is `~/.config/hvx/config.json`. `HVX_CONFIG` selects another path.
The [example configuration](../examples/config.example.json) documents the shape.
Real `config.json` and `.env` files are ignored by Git; dotenv files are not loaded.

## Initialize authentication

Run from the repository root after building:

```bash
bin/hvx init --mode tenant \
  --tenant-server https://api.openhvx.example.test \
  --username user@example.test --password-stdin
```

Supply the password on stdin through your preferred secret-input mechanism. A
password can also come from `HVX_PASSWORD`, or the legacy `--password` option.
Prefer stdin or an environment secret over placing credentials in command history.
`--password-stdin` cannot be combined with `--password` and does not fall back to
the environment if stdin is empty.

Initialization logs in and creates a PAT, then stores it without printing token
material. `--pat-label` and `--pat-expiry-days` configure the new PAT. Admin login
uses `--mode admin --admin-server URL`; it retains the existing tenant credential.
`--insecure` explicitly disables certificate verification for that configuration.

## Context and credentials

| Setting | Meaning |
| --- | --- |
| `profile.mode` | Active `tenant` or `admin` API surface |
| `profile.currentTenant` | Default tenant for admin operations |
| `endpoints.tenant.server` | Tenant gateway URL |
| `endpoints.admin.server` | Admin gateway URL |
| `auth.tenantPat` | Tenant credential |
| `auth.adminPat` | Admin credential |
| `auth.insecure` | Disable TLS verification; default false |

Use `context show`, `context set-mode tenant|admin`, and
`context set-tenant TENANT_ID`. Showing context does not print credentials.
Switching modes requires that the selected mode has its own credential.
Tenant identity comes from authentication; `--tenant` overrides are admin-only.

Changing an endpoint through `init` invalidates its old PAT before the new one is
stored. If editing JSON manually, update or remove the corresponding token too.
Use separate `HVX_CONFIG` files for independent environments and accounts.

## Environment variables

- `HVX_CONFIG`: configuration path.
- `HVX_PASSWORD`: login password fallback when initializing without password stdin.

Endpoints and PATs are configured in JSON. The CLI needs no RabbitMQ, MongoDB, Ceph,
or host-journal credentials.

## Legacy files and persistence

Both the earlier flat configuration and nested `auth.pat` remain readable. The
legacy token is associated only with its saved mode; subsequent saves write the
separate credential fields. Reinitialize the other mode if needed.

Saves write a temporary private file, sync it, and rename it over the selected file.
Temporary files are removed after errors. Secure the parent directory and backups;
Windows users should set appropriate account ACLs. Do not run simultaneous config
updates from multiple processes.

## Named contexts and receipts

Use `init --context NAME` to initialize a separate environment/account. New contexts
start empty; existing contexts retain only their own credentials. `config use-context`
switches mode, tenant, endpoints, and authentication together. `context save NAME`
names the current setup without overwriting another named context.

Saved configuration includes `currentContext` and a `contexts` map. Earlier root-only
configuration remains readable and becomes a default context on save. When manually
editing a named configuration, edit its entry in `contexts`, not just the active
root snapshot. Prefer context/init commands to keep these synchronized.

Request receipts are stored beside this configuration in `requests/`. Their bodies
may include sensitive cloud-init data; protect files and backups with the same care
as credentials. See [replay rules](task-protocol.md#local-persistence).
