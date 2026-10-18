# API

The base path is **`/api/v1`**. The frontend uses `/api/v1` as its base address and
calls it with relative paths.

## Groups and permissions

Endpoints fall into four groups, and role checks apply per group.

| Group | Path | Allowed roles |
|------|------|-----------|
| Admin | `/api/v1/admin/**` | **admin** |
| Stack | `/api/v1/stacks/**` | admin · devops |
| CI/CD | `/api/v1/cicd/**` | admin · devops · developer |
| Observability | `/api/v1/observability/**` | **No role check** — authentication only |
| Login | `POST /api/v1/auth/login` | Before authentication |

Every group also has a per-user rate limit.

!!! danger "The observability group has no role check"
    Once authenticated, **anyone** can call `/api/v1/observability/**`.
    This includes creating, editing, and deleting alert rules — that screen has been
    taken down from the UI, but the API is still open.

!!! danger "development mode turns off all checks"
    If `server.mode` is `development`, the auth middleware and role checks are **not
    registered.** All four groups are open.

## Screen permissions differ from API permissions

The sidebar and routes are frontend mechanisms; **the real source of truth for
permissions is the server groups**. Which screen is blocked by which API is in
[Permissions](rbac.md).

Most notably, `/stack/install` and `/cicd/developer-deploy` have broader screen
permissions, but the lookup APIs that populate them are **admin-only**, so the screens
open but their data looks empty.

## Response shape

List endpoints mostly return `{items: [...], total: N}`.

!!! warning "Some lists ignore query parameters"
    `GET /api/v1/stacks` and `GET /api/v1/cicd/pipelines` **do not read** values such
    as `status` or `search` even when you send them, and return the organization's
    full list. The screen filters and sorts what it receives. No endpoint reads
    `page`, `page_size`, or `sort`, and `limit` and `offset` work only on the
    vulnerability list, audit logs, backup history, stack alerts, and notification
    delivery history.

## Health check

```
GET /health
```

Returns `status`, `db`, and `version`. The server is healthy when `db` is `connected`.

## Full specification

Per-endpoint details are in the repository's
[API design document (Korean)](https://github.com/cloud-nullus/nullus/blob/main/docs/20_%EC%95%84%ED%82%A4%ED%85%8D%EC%B2%98/Nullus_API_%EC%84%A4%EA%B3%84.md).

That document is a design proposal. To see the routes that are actually live, check
this site's feature pages — each screen's page lists the APIs it calls.
