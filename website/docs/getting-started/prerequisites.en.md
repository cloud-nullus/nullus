# Prerequisites

The versions below were checked directly against **`go.mod`, `package.json`, the CI workflows,
the Dockerfile, and the chart**, not the README.

## Required

| Tool | Required version | Source |
|------|----------|------|
| **Go** | **1.26.1** or later | `go.mod` declares `go 1.26.1`. CI installs Go with `go-version-file: go.mod`, and the image builder uses `golang:1.26.9-alpine` |
| **Node** | **22** | CI pins `node-version: '22'`. The web image also uses `node:22-alpine`. `package.json` has no `engines` field |
| **Docker / Docker Compose** | The code enforces no minimum version | The local preflight script only checks that they **exist** and just prints the version |
| **PostgreSQL** | See the table below | — |

## PostgreSQL — not a single number

The version depends on where it is used.

| Where | Version |
|------|------|
| CI service container | **18** |
| Local Docker Compose | **17** |
| Installed by the Helm chart | **17.5.0** (Bitnami postgresql chart 16.7.21 — that is the chart version, not the PG version) |
| Client bundled in the API image | `postgresql17-client` |

**Use 17 for local development.** 18 is used only in CI, to check that the tests also pass on 18;
the chart installs 17.5.

## Kubernetes side (for running E2E)

| Tool | Value |
|------|-----|
| **kind node image** | `kindest/node:v1.35.1` |
| kubectl in the API image | v1.34.12 |
| Helm in the API image | v3.22.0 |
| Helm for CI checks | v3.16.4 |
| Air-gapped OCI (plain-http) | Requires Helm **3.14 or later** |

**The supported range is based on the client library.** The server talks to Kubernetes through
`client-go v0.35.2`, and client-go's official compatibility range is the same minor version ±1 —
that is, **1.34–1.36**. The resources the chart uses (`apps/v1`, `batch/v1`, `networking.k8s.io/v1`)
also exist in older versions, but the chart has no `kubeVersion` constraint, so nothing blocks an
older server. The only version actually tested is kind **v1.35.1**.

There are **two** local kind clusters.

| Name | Nodes |
|------|------|
| `nullus-platform` | 1 control-plane + 2 workers |
| `nullus-develop` | 1 control-plane + 1 worker |

## Framework versions (for reference)

Vite `^8.0.0` · React `^19.2.4` · TypeScript `~5.9.3` · Tailwind `^4.2.4` ·
vitest `^4.1.0` · MUI `^9.3.1`

## Preflight check

```bash
./scripts/runbook_local.sh preflight
```

It checks that `go`, `node`, `npm`, `docker`, `lsof`, and `curl` are installed
(`kind`, `kubectl`, and `helm` are optional) and prints each version. It does not enforce
minimum versions.
