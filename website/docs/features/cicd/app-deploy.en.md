# App Deploy

<span class="route">/cicd/developer-deploy</span>
<span class="role role-admin">admin</span>
<span class="role role-devops">devops</span>
<span class="role role-developer">developer</span>

The screen for putting one app on a pipeline. **When a developer logs in, they land
here directly.** No role has a sidebar entry for it; you get here from the buttons
on [CI/CD List](list.md) and [CI/CD Template](templates.md), or from the home
screen.

!!! note "The step strip scrolls; it does not navigate"
    At the top are six cells — `1 Basic Info · 2 Code Checkout · 3 Build · 4 Test · 5 Security · 6 Create` —
    but **there are no Next/Previous buttons.** Clicking a cell only scrolls to
    that spot. Steps 2 and 3 share the same section, and **4 (Test) and 5
    (Security) are not rendered at all** — those features are permanently
    disabled in code.

![The App Deploy screen when logged in as developer — the sidebar has 4 entries](../../assets/screens/app-deploy-light.png#only-light)
![The App Deploy screen when logged in as developer — the sidebar has 4 entries](../../assets/screens/app-deploy-dark.png#only-dark)

## What you fill in

### Basic Info

| Field | Rules |
|------|------|
| **App Name** | Required. 2–50 characters |
| **Capabilities** | `CI` · `CD` · `Test` · `Security` checkboxes. CI and CD are on by default; **Test and Security are disabled** |
| **Phase** | `production` · `qa` · `development`. Default: production |

You can choose a **Phase**, but it is not included in the create request — it
never reaches the server.

### Code Checkout and Build (when CI is selected)

First, use **Select Stack** to choose
which stack's tools to use (if there are no stacks, the only option is
`Manual Input`). The default URLs of the three fields below are then filled with
that stack's repository and registry addresses.

1. **Source Repository** — stack URL (not editable) + path
2. **Dockerfile Repository**
3. **Image Registry**

### Create (when CD is selected)

| Group | Fields |
|------|------|
| Deploy YAML Repository | URL + directory, plus a reload button |
| Cluster & Namespace | **Cluster** (locked to the stack's cluster once you pick a stack) · **Namespace** (you can type your own via `New Namespace`) · **Service URL** |
| Resource Configuration | **Replicas** (1–10, default 2) · CPU Request `100m` · CPU Limit `500m` · Memory Request `128Mi` · Memory Limit `512Mi` — all required |
| Environment Variables | Add KEY/value rows with **Add Variable**; delete them with the trash icon |

The **Review Manifest** panel on the right previews the `Deployment` · `Service` ·
`Ingress` YAML that will be generated, and **Execute** sits below it.

!!! warning "The reload button cannot find one of the files"
    It tries to fetch `deployment.yaml` · `service.yaml` · `ingress.yaml`, but
    `deploy/ingress.yaml` is **not** among the files Nullus writes to the
    repository. The third one always returns 404.

## When you click Execute

1. The execution mode is derived from the Capability combination — `ci_cd` · `ci` · `cd`
2. The three manifests from the preview are put into the environment variables
   `NULLUS_MANIFEST_DEPLOYMENT` · `NULLUS_MANIFEST_SERVICE` ·
   `NULLUS_MANIFEST_INGRESS`
3. The pipeline is created with `POST /api/v1/cicd/pipelines`
4. If the response contains warnings, the flow **stops here** and shows a
   "Pipeline created, but setup is incomplete" box
5. If there are no warnings, it calls `POST /api/v1/cicd/pipelines/{id}/deploy`
   and goes to `/cicd/pipelines/{id}/logs`

!!! warning "Resource settings travel only inside the manifest strings"
    The server can accept `replicas` · `port` · `provision_repository` ·
    `registry_credentials`, but **the screen does not send those four.** So the
    Replicas, CPU, and Memory values travel only inside the YAML in the
    environment variables above.

The fields actually sent are `name` · `app_type` · `cluster_id` · `namespace` ·
`template_id` · `stack_id` · `git_repo_url` · `dockerfile_path` ·
`docker_context` · `execution_mode` · `env_vars`.

## Stages when the platform deploys directly

For a pipeline that is not linked to a stack, Nullus builds and applies it itself.
The stage names it logs then **differ** from the stages in the CI definition.

```
Git Clone → Docker Build → Image Load (or Image Push)
→ Create Namespace → Create Deployment → Create Service → Create Ingress
```

A pipeline linked to a stack has **a single stage, `Trigger CI`** — the real work
runs on the GitLab, GitHub, or Jenkins side. For the stages there, see the
[CI/CD overview](index.md#stages-a-pipeline-actually-runs).

!!! warning "developer cannot fill this screen to the end"
    The Stack, Cluster, and Namespace pickers are populated from lookup APIs that
    only admin and devops may call. In an environment with authentication enabled,
    a developer gets **403s that leave these pickers empty** and cannot get as far
    as creating a pipeline — even though this is the developer's landing screen.
    The server itself allows developers to create and deploy pipelines; only the
    lookups that fill the screen are blocked
    ([Permissions](../../reference/rbac.md#screens-that-open-but-show-no-data)).
