# By role

Nullus has three roles. **Who uses the tool for what** differs quite a bit from role
to role, so the fastest way in is the page for your own role.

<div class="nullus-cards" markdown>

[**Admin**<span>Sets up organizations, users, and clusters, and manages backups and notification channels. Sees all 19 menu items.</span>](admin.md)

[**DevOps**<span>Installs stacks and creates pipelines. Sees 11 menu items.</span>](devops.md)

[**Developer**<span>Deploys their own apps and checks the results. Sees 4 menu items.</span>](developer.md)

</div>

## At a glance

| | Admin | DevOps | Developer |
|---|:---:|:---:|:---:|
| First screen after login | `/admin/organization` | `/stack/templates` | `/cicd/developer-deploy` |
| Visible menu items | 19 | 11 | 4 |
| Manage organizations and users | ✅ | — | — |
| Register clusters | ✅ | — | — |
| Install stacks | ✅ | ✅ | — |
| View stack list and history | ✅ | ✅ | Not in the menu[^1] |
| Adjust OSS resource defaults | ✅ | ✅ | — |
| Edit the compatibility matrix | ✅ | — | — |
| Create pipelines | ✅ | ✅ | ✅[^2] |
| Pipeline list and history | ✅ | ✅ | ✅ |
| App deploy screen | ✅ | ✅ | ✅ |
| Monitoring and Alert History | ✅ | ✅ | ✅ |
| Backup and restore | ✅ | — | — |
| Notification channels | ✅ | — | — |

[^1]: Hidden in the sidebar, but the route has no role restriction, so it opens by URL. See [Permissions](../reference/rbac.md#where-the-sidebar-and-routes-disagree).
[^2]: The `/cicd/create` route allows all three roles. However, the developer sidebar has no CI/CD Template or Golden Paths entry, so developers reach the create button from CI/CD List.

## Where roles come from

The frontend reads the role from the token Keycloak issues and stores it
(`extractRoleFromOidc` in `web/src/stores/auth-store.ts`). In other words, **a role is
not something you change inside Nullus; it comes from the IdP**. You grant roles to
users on [User Management](../features/admin/users.md).
