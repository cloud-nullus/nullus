# Stack Version

<span class="route">/stack/version</span>
<span class="role role-admin">admin</span>
<span class="role role-devops">devops</span>

A compatibility matrix that shows which tool-version combinations have been
validated. `/stack/versions` renders the same screen.

![Tool-version compatibility matrix](../../assets/screens/stack-versions-light.png#only-light)
![Tool-version compatibility matrix](../../assets/screens/stack-versions-dark.png#only-dark)

## Table

The columns are fixed at seven: **GitLab · Argo CD · Prometheus · Grafana ·
OpenTelemetry · K8s · Status**. A matrix whose ID contains `gitlab-argocd` gets
a `Recommended` badge. You can narrow the list with `Search stacks or tools...`.

!!! warning "Only matrices that include GitLab are shown"
    The table draws only matrices whose name contains `gitlab`. Matrices based
    on GitHub or Gitea **do not appear on this screen**, even if they are
    registered. See the full list on the
    [Stack Version Management](#stack-version-management) screen.

## Validate Current Stack

!!! warning "This button does not work right now"
    The button calls the API with the string `current` in place of the stack
    ID, and the server interprets that as the mode that inspects a stored
    stack. But the validation use case is assembled **without a stack
    repository**, so the call returns a 400 with
    `"persisted validate mode requires a stack repository"`.

    For the same reason, **every call** to
    `POST /api/v1/stacks/{stackId}/validate` in stored-stack mode fails. The
    client-side checks used by the install wizard are not affected.

## Stack Version Management

<span class="route">/admin/stack-versions</span>
<span class="role role-admin">admin</span>

The admin screen for registering and editing matrices. In the sidebar it sits
inside the "DevSecOps Stack" group, but its path is under `/admin/`.

On the left is the matrix list (name, a `verified` / `untested` /
`unsupported` badge, ID, and `K8s <range> · N tools`). The detail view on the
right shows the Kubernetes version range, a tools table (Name · Helm · App ·
Arch · Min K8s · Tier), and a cluster compatibility table (Cluster · Node
Architectures · Compatibility). Each cluster has a **Refresh Discovery** button
that re-reads its node architectures
(`POST /api/v1/admin/clusters/{id}/refresh-discovery`).

The search and status filter bar appears **only when there are more than 5
matrices**.

!!! warning "New matrix, Edit, and Delete return 404"
    The three buttons call `/api/v1/admin/compatibility/matrices`, but the
    function that registers those routes is **never called** on the server.
    Even if it were, the management option is missing, so it would do nothing.

    Right now, the only way to add a matrix is through **migration seed
    data**.

## APIs it calls

| Action | Endpoint |
|------|-----------|
| Matrix list | `GET /api/v1/stacks/compatibility` |
| Validate stack | `POST /api/v1/stacks/{stackId}/validate` |
| Refresh node discovery | `POST /api/v1/admin/clusters/{id}/refresh-discovery` |
