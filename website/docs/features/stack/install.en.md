# Stack Install

<span class="route">/stack/install</span>
<span class="role role-admin">admin</span>
<span class="role role-devops">devops</span>

On this screen you refine the setup taken from a template and deploy it to a
cluster. It is not in the sidebar; you get here by clicking
**Use This Template** on [Stack Template](templates.md).

It is not a sequence of ordered steps but **10 tabs**. There is no progress
indicator (stepper), and you can click **Deploy** from any tab.

![Install screen where you fill in the setup tab by tab](../../assets/screens/stack-install-light.png#only-light)
![Install screen where you fill in the setup tab by tab](../../assets/screens/stack-install-dark.png#only-dark)

## Fill this in first, not a tab: the header card

This card always sits above the tabs. It holds the inputs that are actually
required.

| Field | Rule |
|------|------|
| **Stack Name** | Required. 2–50 characters: letters, digits, and hyphens (`^[a-zA-Z0-9-]+$`). Rejected if the same cluster already has a stack with that name |
| **Access domain** | Defaults to `{stack-name}.internal`. Each tool is reached at `{tool}.{stack}.internal` |
| Enable Access Domain TLS (cert-manager) | When checked, three fields become required: **TLS Secret Name**, **TLS Secret Namespace**, and **cert-manager Issuer Name** |
| **Target Cluster** | Required. Shown together with its connection status |
| **Namespace** | `기본 (nullus)` (default) · an existing namespace · `새 네임스페이스 생성...` (create a new namespace) |

On the right, a pinned **Configuration Summary** panel keeps showing the values
you have chosen. The two buttons at the top are **Save Draft** and **Deploy**.

## The 10 tabs

| # | Tab | What you choose |
|---|-----|----------|
| 1 | **Authentication** | None, or OpenBao |
| 2 | **Artifacts** | Source Repository · Container Registry · Package Registry. Choosing GitHub opens fields for the Organization, API base URL, and Personal access token |
| 3 | **CI/CD** | CI/CD Platform · CD Tool |
| 4 | **Observability** | Visualization (multiple) · Metrics · Logs · Traces · Exporter / Agent |
| 5 | **Security** | Image Scanner · Static Analysis (SAST) · Third-party Dependency Vulnerabilities (SCA) |
| 6 | **Storage** | Storage Plan (Not selected / Connect existing DB/Storage / Create and connect integrated DB/Storage) · StorageClass · Database · Object Storage |
| 7 | **Resources** | Sizing Profile (Local · Startup · Standard · Enterprise, plus organization profiles) · applied CPU, memory, and storage values per tool |
| 8 | **YAML View** | Edit the Gateway and per-tool manifests directly in a Monaco editor |
| 9 | **Preview Deploy Script** | Review the generated `<stack>-deploy.sh`, read-only |
| 10 | **Dry Run** | Run 10 checks and see PASS/WARN/FAIL |

**Log settings live inside the Observability tab** — there is no separate
Logging tab.

### Seven tabs are always open; three are conditional

To move to **YAML View**, **Preview Deploy Script**, or **Dry Run**, you need a
stack name (not a duplicate), a target cluster, and (in new-namespace mode) a
namespace. Otherwise the screen sends you back to the Artifacts tab.

### The 10 things Dry Run checks

Stack name format · cluster selection · namespace validity · access domain
rules · Gateway TLS · install file generation status · YAML/values validation ·
resource floors · bundled OSS version conflicts · storage plan.

The `Run Dry Run` button only stamps the "last run" time — the checks
themselves are recomputed every time a value changes.

## Deploy

The **Deploy** button stays disabled if any of the following is true:

- The stack name is empty, shorter than 2 characters, or a duplicate
- No cluster is selected
- New-namespace mode is on but the namespace is empty
- There are manifest validation errors
- The client-side compatibility check is `fail`, or it is `warn` and you have not acknowledged it

When you click it, the screen creates the stack with `POST /api/v1/stacks`
(reusing an existing row for the same cluster and name if one is in progress or
`pending`), starts the deployment with `POST /api/v1/stacks/{id}/deploy`, and
then navigates to `/stack/deploy/{id}`.

**Save Draft** calls `POST /api/v1/stacks/draft`.

## Deploy progress screen

<span class="route">/stack/deploy/:id</span> · <span class="route">/stack/logs/:deploymentId</span>

Both paths render the same screen. It shows a stage timeline and progress bar,
the last error on failure, a list of warnings and errors, a collapsible
**Raw Logs** panel, a **Continue** button, and a panel that shows the
namespace's pods in real time.

Logs and pods arrive over WebSocket (`/ws/deployments/{id}/logs`,
`/ws/deployments/{id}/pods`).

!!! note "WebSockets are authenticated only in oidc mode"
    If `auth.mode` is not `oidc`, no authentication middleware is attached to
    the sockets. That is because a browser WebSocket cannot carry `X-User-*`
    headers.

## Good to know

!!! warning "The server Pre-Deploy Gate does not work"
    The screen has a "Server Pre-Deploy Gate" panel and a checkbox to
    acknowledge server warnings, but the deploy handler is assembled without
    compatibility validation, so the server check passes straight through
    (it just returns). The only checks that actually run are the
    **client-side matrix and architecture checks**.

!!! warning "Resource estimates are not computed on the server"
    A `POST /api/v1/stacks/estimate` endpoint exists, but **nothing calls
    it.** The numbers on the Resources tab are computed in the browser from
    `resource-defaults`.

Caveats about export and import are in
[Stack List](list.md#export-and-import).
