# Getting started

If you are new to Nullus, read the pages in this order.

<div class="nullus-cards" markdown>

[**Prerequisites**<span>Which tools you need, and which versions. Values checked against the code and CI.</span>](prerequisites.md)

[**Run locally**<span>The shortest path to bring up the backend and frontend and check that they are healthy.</span>](run-local.md)

[**End to end (E2E)**<span>15 steps covering stack install, pipelines, backup, and restore.</span>](e2e.md)

</div>

## What Nullus does

Nullus is a platform that lets DevOps engineers **pick a verified combination of CI/CD tools**,
**configure it in the web UI without writing code**, and, with a single click, **automatically
install a complete DevSecOps stack on a Kubernetes cluster**.

It focuses on tying "prepare the cluster → choose and install a stack → observe the deployment"
into a single flow. Platform teams maintain the standard templates and the version compatibility
policy, and development teams use verified pipelines right away, without repetitive infrastructure
setup.

## Two paths

| | Short path | Full walkthrough |
|---|---------|---------|
| What | Bring up only the platform (API + web) | From a kind cluster all the way to backup and restore |
| Time | A few minutes | A little over an hour |
| What you need | Docker Compose | The above, plus kind, kubectl, and helm |
| Docs | [Run locally](run-local.md) | [E2E guide](e2e.md) |

## Know the implementation scope first

!!! warning "The GitLab path is the most tested"
    Stack automation is built around the GitLab-based flow. GitHub + Argo CD and Gitea +
    Jenkins + Argo CD are also available as templates, but GitLab has seen the most testing,
    and stabilization of the CI/CD area is ongoing.

!!! warning "The default authentication mode is not for production"
    If you do not configure it, the authentication mode is `session`, and in this mode **the
    server trusts the role claimed in the request header as is.** It is meant for development.
    In production you must switch to `oidc` — this is covered in the
    [Admin overview](../features/admin/index.md#check-the-authentication-mode-first).

Features that do not work yet on a screen are marked as such on that screen's page.
See [Features at a glance](../features/index.md) for the full list.
