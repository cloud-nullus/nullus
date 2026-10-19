# CLI

The `nullus` command-line tool.

## Getting it

The release assets include no CLI binary, and no make target builds it. Build it from source
yourself.

```bash
go build ./cmd/nullus
```

## Command tree (everything implemented)

```
nullus
  --server <url>        API server address
  -o, --output <fmt>    json (anything else gives a human-readable table)

  version               Print the CLI version
  stack                 View and deploy stacks
    ls                  List stacks (GET /api/v1/stacks)
  mcp                   MCP server
    serve               Start the stdio MCP server
      --allow-write     Expose the 3 write tools
```

!!! warning "There is no `nullus login` — pass the token in an environment variable"
    `login`, `auth bootstrap issue|revoke`, `cluster`, `pipeline`, and
    `stack deploy|status|logs|rm|rollback|retry|template|config` exist only in the design and
    are not registered. The OIDC login **logic** is in the library, but no command calls it.
    Provide credentials with `NULLUS_TOKEN`.

    The authentication error message the server returns tells you to run `nullus login`, but
    that command does not exist yet.

## Configuration and token

Precedence is **flag > environment variable > file**.

| Location | Content |
|------|------|
| `~/.nullus/config` | YAML, `server:` key |
| `~/.nullus/token` | Token |
| `NULLUS_CONFIG_DIR` | Moves the directory above somewhere else |

!!! note "The token file's permissions are checked"
    If they are not `0600`, the CLI refuses to read the file and recommends replacing the token.

## Exit codes

| Code | Meaning |
|------|-----|
| 0 | Success |
| 1 | Any other error |
| 2 | Usage error or 4xx |
| 3 | Authentication failure (401, 403) |
| 4 | Not found (404) |
| 5 | 5xx or transport error |

!!! note "Codes 6 and 7 from the guide do not exist"
    The table in the CLI guide lists 6 (job failed) and 7 (`--wait` timeout), but no
    classification that produces those codes is defined, because the command that would use them
    (`stack status --wait`) does not exist yet.

## The four bundled binaries

| Binary | Role | Built by `make build`? |
|----------|------|---------------------|
| `cmd/api` | Platform API server | ✅ |
| `cmd/nullus-bootstrap` | Issues Keycloak credentials for unattended and air-gapped installs (`issue` / `revoke`) | ✅ |
| `cmd/nullus` | CLI + MCP server | ❌ |
| `cmd/token-source-sync` | Backfills token sources (see [Token Management](../features/admin/tokens.md)) | ❌ |
