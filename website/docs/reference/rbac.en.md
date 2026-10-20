# Permissions

There are three roles: **admin · devops · developer**. Access is restricted in three places.

1. **Sidebar** — whether a menu item is shown (`web/src/components/layout/nav-model.tsx`)
2. **Route** — whether entry by URL is blocked (`ProtectedRoute allowedRoles` in `web/src/app/routes.tsx`)
3. **Server** — whether an API call is rejected

The three layers do not always agree. Some screens hidden from the sidebar still open
by URL, so treat **the server as the source of truth for permissions**.

## First screen after login

`web/src/features/auth/role-landing.ts` is the single source. The login redirect and
the start button on the home screen read the same value.

| Role | First screen |
|------|---------|
| admin | `/admin/organization` |
| devops | `/stack/templates` |
| developer | `/cicd/developer-deploy` |

If the role is unknown, the user is sent to home (`/`) — sending them to a path
outside their permissions would bounce them right away.

## Route permissions — four groups

`routes.tsx` wraps the screens in four groups.

| Wrapper | Allowed roles | Paths inside |
|------|-----------|----------------|
| `<ProtectedRoute />`<br>(no role restriction = login is enough) | admin · devops · developer | `/`<br>`/stack/templates`<br>`/stack/list`<br>`/stack/history/:stackId?`<br>`/stack/version`, `/stack/versions`<br>`/stack/logs/:deploymentId`<br>`/stack/deployments/:deploymentId/retry-history`<br>`/observability/monitoring`<br>`/observability/alert-history`<br>`/observability/security/dashboard` |
| `allowedRoles={['admin','devops']}` | admin · devops | `/stack/install`<br>`/stack/:id/add-tools`<br>`/stack/deploy/:id`<br>`/stack/oss-resource-default` |
| `allowedRoles={['admin','devops','developer']}` | admin · devops · developer | `/cicd/developer-deploy`<br>`/cicd/templates`<br>`/cicd/create`<br>`/cicd/golden-paths`<br>`/cicd/list`<br>`/cicd/history`<br>`/cicd/pipelines/:id/logs` |
| `allowedRoles={['admin']}` | admin | `/admin/organization`, `/admin/organizations`<br>`/admin/users`<br>`/admin/clusters`<br>`/admin/known-issues`<br>`/admin/token-management`<br>`/admin/stack-versions`<br>`/admin/backup`<br>`/admin/notifications`<br>`/admin/assistant` |

## Where the sidebar and routes disagree

The screens below are **hidden from the menu but open by URL.** This may or may not be
intentional, so for screens that handle sensitive information, check the server-side
permissions too.

| Screen | Roles that see it in the sidebar | Roles the route allows | Difference |
|------|----------------------|----------------------|------|
| `/stack/templates` | admin · devops | **Every logged-in user** | developer can open it by URL |
| `/stack/list` | admin · devops | **Every logged-in user** | Same |
| `/stack/history` | admin · devops | **Every logged-in user** | Same |
| `/stack/version` | admin · devops | **Every logged-in user** | Same |
| `/cicd/templates` | admin · devops | admin · devops · developer | Same |
| `/cicd/golden-paths` | admin · devops | admin · devops · developer | Same |

There are no mismatches in the other direction (visible in the menu but blocked by the route).

## Server-side permissions — the real boundary

The API checks roles per group. **This is the real boundary.**

| Group | Path | Allowed roles |
|------|------|-----------|
| Admin | `/api/v1/admin/**` | **admin only** |
| Stack | `/api/v1/stacks/**` | admin · devops |
| CI/CD | `/api/v1/cicd/**` | admin · devops · developer |
| Golden path writes | `POST`·`PUT`·`DELETE /api/v1/cicd/golden-paths` | admin · devops |
| Observability | `/api/v1/observability/**` | **No role check** (authentication only) |
| Security Dashboard installed OSS | `GET /api/v1/observability/security/stacks` | admin · devops |
| Login | `POST /api/v1/auth/login` | Before authentication |

Role checks are **an allowlist, not a hierarchy** — even admin passes only if it is on
the list.

Golden paths and the Security Dashboard's installed OSS path are narrower than their
group. Golden paths are the install combinations the whole organization chooses from,
so reads stay open as in the CI/CD group, while changes are narrowed to admin and
devops. The installed OSS path gathers the same data as the stack group (`/stacks/**`),
so it accepts only the stack group's roles.

!!! danger "The observability group does not check roles"
    **Every authenticated role** can call it, up to and including creating, editing,
    and deleting alert rules. The screen has been taken down, but the API is still open.
    The one exception is the Security Dashboard's `/observability/security/stacks`,
    which carries its own role check on the path.

!!! danger "development mode turns off all checks"
    If `server.mode` is `development`, the auth middleware and role checks are not
    registered at all. All four groups are open.

!!! note "An unused permission matrix remains in the code"
    The repository contains middleware with `roleAPIMatrix` and a per-path role table,
    but **it is not wired up anywhere.** Paths in that table, such as
    `/api/v1/clusters`, do not even exist. Do not rely on that code when you check
    permissions.

## Screens that open but show no data

Several frontend screens call APIs that are narrower than the screen's own
permission. So the screen opens, but its lists look empty.

| Screen | Screen permission | APIs called | API permission |
|------|-----------|-----------|----------|
| [Stack Install](../features/stack/install.md) | admin · devops | `/api/v1/admin/clusters`<br>`…/clusters/{id}/namespaces`<br>`…/clusters/{id}/storage-classes`<br>`…/organization/resource-profiles` | **admin** |
| [App Deploy](../features/cicd/app-deploy.md) | All three roles | `/api/v1/admin/clusters`<br>`…/clusters/{id}/namespaces`<br>`/api/v1/stacks`<br>`/api/v1/stacks/{id}/integrations` | admin / admin · devops |
| [CI/CD List](../features/cicd/list.md) | All three roles | `/api/v1/admin/clusters`<br>`/api/v1/stacks` | admin / admin · devops |
| [Monitoring Dashboard](../features/observability/monitoring.md) | All three roles | `/api/v1/admin/clusters`<br>`/api/v1/stacks/{id}/monitoring` | admin / admin · devops |
| [Alert History](../features/observability/alert-history.md) | All three roles | `/api/v1/admin/clusters`<br>`/api/v1/stacks` | admin / admin · devops |
| Home | All three roles | `/api/v1/admin/clusters` | admin |

The larger impact is on **App Deploy, developer's first screen**, and **the devops
install wizard**.

## Menu items visible to each role

| Group | admin | devops | developer |
|------|:-----:|:------:|:---------:|
| DevSecOps Stack | 6 | 5 | — |
| CI/CD | 4 | 4 | 2 |
| Observability | 3 | 3 | 3 |
| Admin | 7 | — | — |
| **Total** | **20** | **12** | **5** |

- The one stack menu item devops does not see is **Stack Version Management** (`/admin/stack-versions`), which is admin-only.
- The 5 items developer sees are **CI/CD List · CI/CD History · Monitoring Dashboard · Alert History · Security Dashboard**.
- **Security Dashboard** sits under the **Security** sub-group of Observability (Observability › Security › Security Dashboard). Sub-groups are not counted as menu items.
