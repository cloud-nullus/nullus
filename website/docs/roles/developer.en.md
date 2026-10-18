# Developer

After login you land on the **App Deploy** (`/cicd/developer-deploy`) screen.
The sidebar shows **4 items**.

| Group | Items |
|------|------|
| CI/CD | CI/CD List · CI/CD History |
| Observability | Monitoring Dashboard · Alert History |

![The App Deploy screen that opens right after you log in as developer. The sidebar has 4 items](../assets/screens/app-deploy-light.png#only-light)
![The App Deploy screen that opens right after you log in as developer. The sidebar has 4 items](../assets/screens/app-deploy-dark.png#only-dark)

## What you do

1. In **[App Deploy](../features/cicd/app-deploy.md)**, fill in the app name,
   repository, cluster, namespace, and resources, then click **Execute**
2. Follow progress in **[Pipeline Logs](../features/cicd/list.md#pipeline-logs)**
3. In **[CI/CD List](../features/cicd/list.md)**, check the status and the
   vulnerability and SAST results
4. Review past deployments in **[CI/CD History](../features/cicd/history.md)**

## Good to know

!!! warning "Selection boxes look empty — because of lookup APIs, not permissions"
    The **Stack, Cluster, and Namespace selections** in App Deploy and the
    **cluster and stack filters** in Monitoring Dashboard and Alert History all come
    from lookup APIs restricted to admin and devops. With authentication enabled, if
    you sign in as developer, these selections come up **empty with 403s**, and App
    Deploy cannot get as far as creating the pipeline.

    The server **does grant** developer the permission to create and deploy pipelines
    — only the lookups that populate the screen are blocked. For now, the reliable
    route is to ask a devops or admin user to create the pipeline, then run it from
    CI/CD List with **Execute**. The alert list itself is available without any role
    restriction.

    The table of which screen comes up empty because of which API is in
    [Permissions](../reference/rbac.md#screens-that-open-but-show-no-data).

!!! note "You will use screens that are not in the menu"
    App Deploy and Pipeline Logs have no sidebar entry. Reach them from the start
    button on the home screen or from the buttons in CI/CD List.

    CI/CD Template and Golden Paths have routes that allow developer, but they are
    not in the sidebar. Once you open one from home, you cannot find your way back
    to it through the menu.

## Screens that open by URL but are not in the menu

These screens are only hidden from the sidebar; their routes have no role
restriction, so they open if you type the URL directly. Their data APIs are blocked,
though, so they mostly look empty.

`/stack/templates` · `/stack/list` · `/stack/history` · `/stack/version` ·
`/cicd/templates` · `/cicd/golden-paths`

The full comparison is in the [Permissions](../reference/rbac.md) table.
