# Environment variables

The server reads `configs/config.yaml` and then overrides it with environment variables.

## Naming rule

Config key `a.b.c` → environment variable **`NULLUS_A_B_C`**.

!!! warning "Keys missing from the config file cannot be overridden by environment variables alone"
    Automatic mapping looks **only at keys present in the config file**. Keys that are
    not there must be bound separately in code; otherwise, setting the environment
    variable is **silently ignored** — the backup sealing key once fell into this trap
    and blocked startup.

    Keys bound separately: `platform.namespace` · `backup.seal_key` ·
    `backup.destination.secret_key` · `backup.keycloak_database.password`.

## Server and database

| Environment variable | Default |
|----------|--------|
| `NULLUS_SERVER_PORT` | 8090 |
| `NULLUS_SERVER_MODE` | development |
| `NULLUS_DATABASE_HOST` | localhost |
| `NULLUS_DATABASE_PORT` | 5433 |
| `NULLUS_DATABASE_NAME` | nullus |
| `NULLUS_DATABASE_USER` | nullus |
| `NULLUS_DATABASE_PASSWORD` | nullus_dev |
| `NULLUS_DATABASE_SSLMODE` | disable |
| `NULLUS_DATABASE_MAX_OPEN_CONNS` / `_MAX_IDLE_CONNS` | 25 / 10 |
| `NULLUS_DATABASE_CONN_MAX_LIFETIME` / `_CONN_MAX_IDLE_TIME` | 5m / 1m |

!!! note "The old `NULLUS_DB_*` names do not work"
    Abbreviated names such as `NULLUS_DB_HOST` match no config key and are ignored.
    `.env.example`, `.env.dev`, and the `Makefile` now use the correct names, but if
    you copied a `.env` earlier, you need to update that file.

## Authentication

| Environment variable | Default | Notes |
|----------|--------|------|
| `NULLUS_AUTH_MODE` | **session** | `oidc` or anything else. Use `oidc` in production |
| `NULLUS_AUTH_SESSION_SECRET` | dev-secret-change-in-production | HMAC key for issuing local tokens. **If empty, the `/auth/login` route is not registered** |
| `NULLUS_AUTH_SESSION_MAX_AGE` | 86400 | A value of 0 or less falls back to 24 hours |
| `NULLUS_AUTH_OIDC_PROVIDER` | keycloak | |
| `NULLUS_AUTH_OIDC_ISSUER_URL` | `http://localhost:8180/realms/nullus` | If empty in `oidc` mode, **the server refuses to start** |
| `NULLUS_AUTH_OIDC_AUDIENCE` | nullus-app | |

## Keycloak (variables with aliases)

| Environment variable | Alias | Default |
|----------|------|--------|
| `NULLUS_KEYCLOAK_ADMIN_URL` | `KEYCLOAK_URL` | Empty — leaving it empty skips the entire OSS SSO integration |
| `NULLUS_KEYCLOAK_PUBLIC_URL` | `KEYCLOAK_PUBLIC_URL` | If empty, inherits the issuer |
| `NULLUS_KEYCLOAK_REALM` | `KEYCLOAK_REALM` | nullus |
| `NULLUS_KEYCLOAK_ADMIN_USER` | `KEYCLOAK_ADMIN_USER` | admin |
| `NULLUS_KEYCLOAK_ADMIN_PASSWORD` | `KEYCLOAK_ADMIN_PASSWORD` | Empty |

The first (prefixed) name takes precedence.

## Other config keys

| Environment variable | Default |
|----------|--------|
| `NULLUS_HELM_TIMEOUT` / `NULLUS_HELM_NAMESPACE_PREFIX` | 600s / `nullus-` |
| `NULLUS_PROMETHEUS_URL` | Empty |
| `NULLUS_LOG_LEVEL` / `NULLUS_LOG_FORMAT` | debug / text |
| `NULLUS_PLATFORM_NAMESPACE` | Injected by the chart |
| `NULLUS_BACKUP_*` | See the [backup docs](../features/admin/backup.md) |

## Variables that do not follow the prefix rule

These bypass the config loader; the code reads them directly.

| Environment variable | What it does | Default |
|----------|---------|--------|
| **`ENCRYPTION_KEY`** | Encrypts and decrypts kubeconfigs and tokens. **Exactly 32 bytes** | None — runtime 500 if empty |
| `OPENBAO_ADDR` / `OPENBAO_TOKEN` | Global OpenBao, local only | Registered only when both are set |
| `NULLUS_GITLAB_URL` / `NULLUS_GITEA_URL` / `NULLUS_JENKINS_URL` | Override tool addresses | Empty |
| `NULLUS_HELM_OCI_REGISTRY` | Air-gapped OCI registry. The installed-image scan also pulls images from this host (the same registry the nodes' containerd mirrors use) | Empty |
| `NULLUS_TELEGRAM_API_URL` | For notification tests | Empty |
| `NULLUS_STACK_ALERT_INTERVAL` | Alert collection interval | 1m (`off` or `0` turns it off) |
| `TOKEN_ROTATION_INTERVAL` | Token rotation check interval | **5m** |
| `CICD_RUN_SYNC_INTERVAL` | CI run sync interval | **10m** |
| `STACK_IMAGE_RESCAN_INTERVAL` | Installed-image rescan interval (air-gapped installs included). Newly found fixable CRITICALs go out as the `stack_cve_found` notification | **24h** |
| `NULLUS_SCM_GROUP` | GitLab group for pipelines | **`nullus`** |
| `NULLUS_ASSISTANT_API_KEY` | LLM API key for the [AI Assistant](../features/assistant.md) (Upstage). Empty turns the assistant off | Empty |
| `NULLUS_ASSISTANT_BASE_URL` / `NULLUS_ASSISTANT_MODEL` | OpenAI-compatible base URL / model name for the assistant | `https://api.upstage.ai/v1` / `solar-pro4` |
| `NULLUS_DEFAULT_ORG_ID` | Default organization ID | Fixed UUID in code |

### CLI and MCP only

`NULLUS_SERVER` · `NULLUS_TOKEN` · `NULLUS_CONFIG_DIR` ·
`NULLUS_MCP_ALLOW_WRITE` · `NULLUS_SCM_TOKEN`

### `nullus-bootstrap` only

`KEYCLOAK_URL`(required) · `KEYCLOAK_REALM`(nullus) · `KEYCLOAK_ADMIN_USER`(admin) ·
`KEYCLOAK_ADMIN_PASSWORD` · `BOOTSTRAP_CLIENT_ID`(nullus-bootstrap) ·
`BOOTSTRAP_ROLE`(admin)

### `token-source-sync` only

`DATABASE_URL` · `OPENBAO_ADDR`(required) · `OPENBAO_TOKEN`(required) ·
`TOKEN_SOURCE_ENV`(default `dev`)

## The config file path cannot be changed

The server reads `configs/config.yaml` from a **hard-coded** path. There is no flag or
environment variable to change it. In containers, the chart mounts the ConfigMap at
the right location to match.

`configs/config.dev.yaml` is not referenced anywhere — not by the Makefile, scripts,
workflows, or code (it appears to be an orphan file).
