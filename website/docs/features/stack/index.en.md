# DevSecOps Stack

This area installs validated OSS combinations on Kubernetes clusters and
manages them. Every API lives under `/api/v1/stacks/**`, and only
**admin and devops** can call them.

<div class="nullus-cards" markdown>

[**Stack Template**<span>Pick a validated combination and go to install.</span>](templates.md)

[**Stack Install**<span>Fill in 10 tabs and deploy.</span>](install.md)

[**Stack List**<span>See installed stacks' status, workloads, config, and vulnerabilities.</span>](list.md)

[**Stack History**<span>Compare changes and roll them back.</span>](history.md)

[**Stack Version**<span>The tool-version compatibility matrix.</span>](versions.md)

[**OSS Default Resource**<span>Per-tool CPU and memory defaults.</span>](oss-defaults.md)

[**Installed OSS CVE Response**<span>How CVEs in installed OSS are reported and fixed, and by when.</span>](cve-response.md)

</div>

## The path to an installed stack

```mermaid
graph LR
  T[Stack Template] -->|Use This Template| I[Stack Install<br/>10 tabs]
  I -->|Deploy| D[Deploy progress & logs]
  D --> L[Stack List]
  L --> H[Stack History]
```

1. On **Stack Template**, pick a combination and click `Use This Template`
2. You land on **Stack Install**; fill in the stack name, cluster, and namespace
3. Click `Deploy` to go to the **deploy progress screen** (`/stack/deploy/:id`) and watch the logs live
4. Manage the finished stack from **Stack List**

!!! note "Stack Install is not in the sidebar"
    It has no menu entry of its own; you reach it from a template. You can also
    open `/stack/install` directly by URL.

## Where devops gets stuck during install

!!! warning "The install wizard calls admin-only APIs"
    Both **admin and devops** can open `/stack/install`, but four pieces of
    data the screen needs come from **admin-only** paths.

    | What the wizard uses | API it calls | Role |
    |------------------|-----------|------|
    | Target cluster list | `GET /api/v1/admin/clusters` | admin |
    | Namespace list | `GET /api/v1/admin/clusters/{id}/namespaces` | admin |
    | StorageClass list | `GET /api/v1/admin/clusters/{id}/storage-classes` | admin |
    | Organization resource profiles | `GET /api/v1/admin/organization/resource-profiles` | admin |

    With authentication enabled, a devops user will most likely see these
    dropdowns empty because the calls return 403. We confirmed this by reading
    the code wiring; we have not reproduced it at runtime.
