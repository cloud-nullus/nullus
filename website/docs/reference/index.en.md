# Reference

<div class="nullus-cards" markdown>

[**Permissions**<span>What each of the three roles can access. A cross-check across three layers: sidebar, route, and server.</span>](rbac.md)

[**Environment variables**<span>The variables the server actually reads, and their defaults.</span>](config.md)

[**CLI**<span>The `nullus` command tree and exit codes.</span>](cli.md)

[**MCP**<span>9 tools for AI assistants.</span>](mcp.md)

[**API**<span>How the endpoint groups are organized, and their permissions.</span>](api.md)

[**Contributing**<span>Branch, commit, and PR rules, and the testing strategy.</span>](contributing.md)

</div>

## Where this content comes from

The content of this site is taken from **the code in the repository**.

| What | Source |
|------|------|
| Screen list and permissions | `web/src/app/routes.tsx` |
| Sidebar | `web/src/components/layout/nav-model.tsx` |
| Screen names | `web/src/i18n/en.json` |
| First screen after login | `web/src/features/auth/role-landing.ts` |
| API routes and permissions | Go router registrations (`cmd/api/main.go` and each module's handler) |
| Environment variables | `internal/shared/config/config.go`, `configs/config.yaml` |
| Default data | `db/migrations/` |
| Versions | `go.mod`, `web/package.json`, `.github/workflows/`, `Dockerfile` |

If you find a mistake, [let us know in an issue](https://github.com/cloud-nullus/nullus/issues).
