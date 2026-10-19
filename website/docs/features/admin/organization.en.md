# Organization

<span class="route">/admin/organization</span>
<span class="role role-admin">admin</span>

Manage organization details and the range of clusters the organization can use.
`/admin/organizations` is the same screen.

![Organization details screen](../../assets/screens/admin-organization-light.png#only-light)
![Organization details screen](../../assets/screens/admin-organization-dark.png#only-dark)

## Screen layout

The organization list is on the left, and the detail form is on the right.

| Field | Saved? |
|------|---------|
| Organization Name | ✅ |
| Slug | ❌ — see below |
| Domain | ✅ |
| Status (`active`·`inactive`·`suspended`) | ❌ |
| **Cluster Access Scope** (grid of cluster checkboxes) | ✅ — saved by cluster **name** |

Below them are the **Member Management** table (Name · Email · Role · Status · Actions),
**Add User** (→ [User Management](users.md)), and the **Invite Member** dialog.
The **New Organization** dialog creates a new organization.

!!! warning "Slug and Status are not saved when you edit them"
    The form makes them look editable, but the update API accepts **only three fields:
    `name`, `domain`, and `clusterAccessScope`**. Slug and Status are not sent.

    `suspended` does not exist as a server-side Status value at all (there are only
    `active` and `inactive`). And **no code reads the organization status to block
    access** — switching it to inactive does nothing.

!!! danger "A failed save looks like a success"
    If the update request fails, the screen silently changes **only the in-browser
    state**. After a refresh, the old values are back. Refresh once after saving to confirm.

!!! note "There is no screen for setting the default admin"
    The data model has a default admin field, but there is no UI or API to set it.

## How the organization is chosen

The read API resolves the organization in this order.

1. The `X-Org-ID` header
2. The `?orgId` query parameter
3. The signed-in user's organization
4. **The first organization in the table**

The frontend sends a fixed `X-Org-ID`
(see the [Admin overview](index.md)).

## APIs it calls

| Action | Endpoint |
|------|-----------|
| Read | `GET /api/v1/admin/organization` |
| Update | `PATCH /api/v1/admin/organization` |
| Create | `POST /api/v1/admin/orgs` — validates slug format and uniqueness, and writes an audit log |
| Members | `/api/v1/admin/organizations/{orgId}/members…` |
| Clusters (for access scope) | `GET /api/v1/admin/clusters` |

## Resource profiles

An organization also carries **resource profiles** for stack installation
(`/api/v1/admin/organization/resource-profiles`). But you save and delete them in the
Resources tab of the [Stack Install](../stack/install.md) wizard, not on this screen.
As a result, a screen devops uses calls an admin-only API.
