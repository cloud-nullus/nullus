# Admin

After login you land on the **Organization** (`/admin/organization`) screen. All 19
sidebar items are visible, and this is the only role that can call all of
`/api/v1/admin/**`.

![The Organization screen that opens when you log in as admin](../assets/screens/admin-organization-light.png#only-light)
![The Organization screen that opens when you log in as admin](../assets/screens/admin-organization-dark.png#only-dark)

## First-time setup order

1. Register the target cluster in **[Cluster Management](../features/admin/clusters.md)**.
   Paste a kubeconfig; the register button unlocks only after **Verify Connection**
   shows `connected`. Do this first
2. In **[Organization](../features/admin/organization.md)**, set the name, domain,
   and **Cluster Access Scope**
3. In **[User Management](../features/admin/users.md)**, add members and assign
   roles
4. Register **[Notification Channels](../features/admin/notifications.md)** and
   confirm them with **Send test**
5. Run a **[Backup](../features/admin/backup.md)** once to check that the destination
   settings are correct

From there on, the path is the same as for devops — see [DevOps](devops.md).

## What only admin can do

| Task | Screen |
|----|------|
| Organization details and cluster access scope | [Organization](../features/admin/organization.md) |
| Add members, assign roles, deactivate | [User Management](../features/admin/users.md) |
| Register and verify clusters | [Cluster Management](../features/admin/clusters.md) |
| Manage the compatibility matrix | [Stack Version Management](../features/stack/versions.md#stack-version-management) |
| Check token sources | [Token Management](../features/admin/tokens.md) |
| Backup and restore | [Backup & Restore](../features/admin/backup.md) |
| Notification channels | [Notification Channels](../features/admin/notifications.md) |
| Create, edit, and delete stack templates | [Stack Template](../features/stack/templates.md) |
| Create, edit, and delete CI/CD templates | [CI/CD Template](../features/cicd/templates.md) |
| Manage monitoring embed tabs | [Monitoring Dashboard](../features/observability/monitoring.md#embedded-tabs-manage-tabs) |

## Check before going to production

!!! danger "Auth mode"
    With the default, `session`, **the server trusts the role claimed in the request
    header as is.** In production, set both `NULLUS_AUTH_MODE=oidc` and the
    frontend's `authMode=oidc`. They must not differ.

!!! danger "Server mode"
    With `server.mode=development`, authentication and role checks are **all
    turned off.** The chart default is `production`.

!!! warning "Admin features that do not work today"
    Each page covers these in detail, but they are worth knowing up front when you
    plan operations.

    | Feature | Status |
    |------|------|
    | Invite links | Placeholder: the server returns an empty response. There is no accept path either |
    | Role Permissions tab | Saved only in browser localStorage. Has no effect |
    | Organization status (Status) | Not saved, and no code reads it to block anything |
    | Compatibility matrix CRUD | Routes not registered — 404 |
    | Token rotate, approve, and pause buttons | Only change the status text. The server scheduler does the actual reissuing |
    | Password login | Cannot succeed with any account |
    | Restore | No screen. API only |
    | Audit log viewing | The API exists, but there is no screen to view it |
