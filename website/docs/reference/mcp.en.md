# MCP

A Model Context Protocol server that connects Nullus to AI assistants.

```bash
nullus mcp serve
nullus mcp serve --allow-write   # also expose the mutating tools
```

The transport is **stdio only**.

!!! note "The CLI guide lists this feature as "not implemented""
    It is actually implemented — nine tools are registered. The documentation
    underreports the feature.

!!! warning "Resources and prompts are not registered"
    Only tools are exposed.

!!! note "The in-app AI Assistant uses these tools too"
    The [AI Assistant](../features/assistant.md) runs this server in memory inside the API and uses only the
    **six read tools** below (the change tools are not in its list). It is not a transport exposed beyond stdio.

## Six read tools (always available)

| Tool | Input | API called |
|------|------|-------------|
| `stack_list` | None | `GET /api/v1/stacks` |
| `stack_status` | `stack_id` | `GET /api/v1/stacks/{id}/status` |
| `cluster_list` | None | `GET /api/v1/admin/clusters` |
| `template_list` | None | `GET /api/v1/stacks/templates` |
| `compat_check` | `stack_id`, `tools?` | `POST /api/v1/stacks/{id}/validate` |
| `stack_logs_tail` | `stack_id`, `lines?` (default 100) | `GET /api/v1/stacks/{id}/deploy/logs/tail` |

In the `cluster_list` response, sensitive keys are **removed entirely, not just masked** —
`kubeconfig`, `kubeconfig_encrypted`, `token`, `password`, `secret`.

## Three mutating tools (only with `--allow-write`)

| Tool | Input | Behavior |
|------|------|------|
| `stack_deploy` | `stack` (stack.yaml v1alpha1), `acknowledge_warnings?` | Creates and deploys a stack |
| `stack_rollback` | `stack_id`, `version_id`, `reason?` | Requests a rollback |
| `pipeline_deploy` | `pipeline_id` | Runs a pipeline |

Instead of `--allow-write`, you can also set `NULLUS_MCP_ALLOW_WRITE=true`. An explicit
flag takes precedence over the environment variable (you can turn it off with
`--allow-write=false`).

!!! note "SCM tokens are not accepted as tool arguments"
    `stack_deploy` strips secret-like fields from its input. It reads the SCM PAT from
    `NULLUS_SCM_TOKEN` in the server process and adds it only to the deploy request.

!!! warning "`stack_rollback` fails today"
    For the same reason described in [Stack History](../features/stack/history.md#rollback),
    the server's rollback path is not wired up yet.

## Startup requirements

Without a token, it does not start the server and exits with **exit code 3**.

!!! warning "The commands it points you to do not exist"
    The error message suggests `nullus login` and `nullus auth bootstrap issue`, but
    [neither command is implemented](cli.md#command-tree-everything-implemented). The
    actual fix is the `NULLUS_TOKEN` environment variable.

At startup it checks once for a version mismatch; if the versions differ, it only prints a
warning to standard error and keeps going. The startup banner and warnings all go to
standard error.
