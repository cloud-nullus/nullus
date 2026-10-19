# Admin

An area for <span class="role role-admin">admin</span> only. The sidebar has 7 screens,
and the backend applies `RequireRole("admin")` to all of `/api/v1/admin/**`, returning
403 to devops and developer.

<div class="nullus-cards" markdown>

[**Organization**<span>Organization details and cluster access scope.</span>](organization.md)

[**User Management**<span>Add members and assign roles.</span>](users.md)

[**Cluster Management**<span>Register target Kubernetes clusters and verify connections.</span>](clusters.md)

[**Token Management**<span>Where the secrets your stacks use come from, and their rotation status.</span>](tokens.md)

[**Known Issues**<span>Known limitations and workarounds.</span>](known-issues.md)

[**Backup & Restore**<span>Capture the platform state and restore it.</span>](backup.md)

[**Notification Channels**<span>Telegram, email, and Slack channels that receive alerts.</span>](notifications.md)

</div>

## Check the authentication mode first

Nullus has two authentication modes. **In production you must use `oidc`.**

| Mode | Setting | Behavior |
|------|------|------|
| `oidc` | `NULLUS_AUTH_MODE=oidc` + `VITE_AUTH_MODE=oidc` | Verifies Keycloak (or Authentik) tokens. For production |
| `session` (default) | Any other value | **Development only** |

!!! danger "session mode is not authentication"
    In `session` mode, the login screen checks the password against a table of test
    accounts bundled in the browser and never asks the server. The server side also
    **trusts the `X-User-ID` and `X-User-Role` headers as is.** In other words, anyone
    can claim any role. Because the default is `session`, a production deployment must
    switch `auth.mode` to `oidc` and set the frontend's `authMode` to match.

!!! danger "development mode turns off every permission check"
    When `server.mode` is `development`, the authentication middleware and every
    `RequireRole` are **not registered.** All four groups are open. The Helm chart
    default is `production`.

## Roles come from the IdP

Nullus does not decide roles; it reads them from the token.

- Keycloak — `realm_access.roles`. If the ID token has none, Nullus **opens the access token** and looks again
- Authentik — `groups`

The priority is admin → devops → **everything else is developer**. A token with no role
claim silently becomes developer.

!!! warning "The organization ID is hard-coded"
    The frontend **hard-codes** the organization ID it sends as `X-Org-ID` to
    `11111111-1111-1111-1111-111111111111`. Running multiple organizations does not
    work through this path yet.
