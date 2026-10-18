# Known Issues

<span class="route">/admin/known-issues</span>
<span class="role role-admin">admin</span>

A **read-only** table of known limitations and their workarounds.

![Known issues list](../../assets/screens/admin-known-issues-light.png#only-light)
![Known issues list](../../assets/screens/admin-known-issues-dark.png#only-dark)

## Table columns

| Column | Content |
|------|------|
| ID | Issue identifier |
| Severity | Issue severity |
| Title | Title and description |
| Status | Issue status |
| Workaround | How to work around it |

There are no create, edit, or delete buttons.

## Where the content comes from

The `known_issues` table in the database
(`GET /api/v1/admin/known-issues` → `{items: […]}`).

The migration `db/migrations/000013_known_issues.up.sql` creates the table and inserts
the **first 3 entries** — Helm installation requires cluster-admin, dashboard metrics lag,
and certificates are not renewed automatically.

!!! warning "You cannot edit the list from the product"
    There is **no write path at all.** The API has no create, update, or delete, and
    neither does the UI. To add an entry, write a new migration or insert it directly
    into the DB.
