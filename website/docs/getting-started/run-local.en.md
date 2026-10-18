# Run locally

This is the shortest path to bring up just the platform (API + web) and look at the screens. For
the full process of actually installing a stack on a cluster, see the [E2E guide](e2e.md).

## The shortest path

```bash
./scripts/runbook_local.sh preflight   # Check tools
./scripts/runbook_local.sh up          # Infrastructure + migrations + API + frontend
```

When it finishes, check the result.

```bash
curl http://localhost:8090/health
# {"status":…, "db":"connected", "version":…}
```

The platform is up properly only when `db` shows `connected`.

| Address | What |
|------|------|
| `http://localhost:8090` | API |
| `http://localhost:5173` | Web (Vite) |
| `http://localhost:8180` | Keycloak |

The frontend does not take the API address from an environment variable. It sends requests to the
relative path `/api/v1`, and Vite forwards `/api` and `/ws` to `localhost:8090`.

## The manual path

```bash
make dev        # Start Docker Compose + apply migrations
make run        # Build bin/api and run it  → http://localhost:8090
make web-dev    # Vite dev server           → http://localhost:5173
```

`make dev` starts **four** containers — PostgreSQL, object storage, Redis, and **Keycloak**.

!!! note "The onboarding doc leaves out Keycloak and says three"
    `make dev-up` brings up every service without naming any, so Keycloak starts as well.

!!! note "The object storage service is named `minio`, but the implementation is SILO"
    It uses an AGPL fork of MinIO (`pgsty/silo`). It also keeps the `MINIO_ROOT_*` environment
    variables (for compatibility), so the names make it easy to mistake it for MinIO. The banner
    that `make dev` prints still says "MinIO", too.

## Common make targets

| Target | What it does |
|------|---------|
| `dev-up` / `dev-down` / `dev-clean` | Start Compose / stop it / stop it and delete the volumes too |
| `dev-status` / `dev-logs` | Show status / follow logs |
| `dev` | `dev-up` + `migrate-up` |
| `build` | Builds `bin/api` and `bin/nullus-bootstrap` |
| `run`, `run-dev` | Runs the API, reading `.env.dev` |
| `test` | `go test ./...` — **untagged tests only** |
| `test-integration` | `go test -tags integration ./e2e/` (starts Postgres itself with testcontainers) |
| `test-golden-path` | **60-minute E2E that actually deploys to kind `nullus-platform`** |
| `test-cover` | `coverage.out` and `coverage.html` |
| `lint` | `golangci-lint run ./...` — **Go only** |
| `migrate-up`, `migrate-down`, `migrate-status` | Migrations |
| `db-shell` | Connects with `psql` |
| `web-dev`, `web-build`, `web-test` | Frontend |
| `clean` | Deletes `bin/`, `coverage.*`, and `web/dist` |

!!! note "Four things that are easy to mix up"
    - `make build` builds **only the Go binaries** — `bin/api` and `bin/nullus-bootstrap`.
      The frontend build output (`web/dist`) comes from `make web-build`. No target builds
      `cmd/nullus` (the CLI)
    - `make lint` checks **Go only**. For the frontend, run `cd web && npm run lint`
    - `make test` runs without build tags, so the `integration` and `e2e` tests are **left out**.
      For those, use `make test-integration`
    - `make test-golden-path` is a **60-minute** E2E that actually deploys to a kind cluster

## Frontend commands

```bash
cd web
npm run dev          # Vite dev server
npm run build        # tsc -b && vite build
npm run lint         # eslint .
npx vitest run       # Unit tests
npm run e2e          # Playwright (chromium)
```

!!! note "There is no `test` script"
    Run unit tests with `npx vitest run` (for watch mode, `npx vitest` with no arguments).
    There is no `lint:fix` either, so auto-fix is `npx eslint . --fix`. Prettier is not a
    dependency; you fetch and run it with `npx prettier --write .` — the format check in CI uses
    its own prettier.

Other available scripts: `preview`, `e2e:visual`, `e2e:visual:update`,
`e2e:headed`, `e2e:report`, `e2e:stack-critical`, `e2e:shots`,
`inventory[:check]`, `theme:generate[:check]`, `responsive:audit[:check]`

## Database

| Item | Value |
|------|-----|
| Tool | golang-migrate (v4.20.1 in the image; installed automatically with `go install` locally) |
| Location | `db/migrations/` — 96 up/down pairs, numbered `000001`–`000098` |
| Apply locally | `make migrate-up` |
| Connection string (local default) | `postgres://nullus:nullus_dev@localhost:5433/nullus?sslmode=disable` |

When you deploy to Kubernetes, **the API does not run the migrations itself.** A pre-upgrade hook
Job in the chart runs `migrate` with the API image (`migration.enabled` defaults to `true`).

!!! note "The Compose Postgres does not run migrations"
    It mounts `db/migrations` into a subdirectory of `/docker-entrypoint-initdb.d`, and
    the Postgres init step does not look into subdirectories.
    `migrate` always applies the migrations.

## Changing environment variables

Environment variable names follow the config keys directly — `database.host` becomes
`NULLUS_DATABASE_HOST`. `.env.example`, `.env.dev`, and the `Makefile` use these names.

!!! note "The old `NULLUS_DB_*` names have no effect"
    Shortened names such as `NULLUS_DB_HOST` match no config key, so they are **silently
    ignored.** If the `.env` you have uses those names, changing the port or password has no
    effect — switch to `NULLUS_DATABASE_*`.
    (`internal/shared/config/database_env_test.go` pins down both cases.)

    The three lines `NULLUS_MINIO_ENDPOINT`, `_ACCESS_KEY`, and `_SECRET_KEY` are not used at
    all, because the config structure has no matching fields.

The correct names are listed in [Environment variables](../reference/config.md).
Also, `make run`/`run-dev` read **`.env.dev`**, not `.env`.

!!! note "Backend hot reload is not set up"
    The setup guide tells you to install Air, but the repository has no `.air.toml` and no
    script calls `air`.
