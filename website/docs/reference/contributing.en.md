# Contributing

The full rules are in the repository's
[CONTRIBUTING.md](https://github.com/cloud-nullus/nullus/blob/main/CONTRIBUTING.md)
and [CLAUDE.md](https://github.com/cloud-nullus/nullus/blob/main/CLAUDE.md). This page
collects only the ones people most often trip over.

## Branches

```
feat/<module>/<description>     New feature
fix/<module>/<description>      Bug fix
chore/<module>/<description>    Everything else (docs, tests, refactoring, CI, config)
```

These **three are the only** branch types. `refactor`, `test`, `docs`, `perf`, and `ci` are used
**only as commit types**.

`<module>` is one of `stack`, `cicd`, `admin`, `auth`, `o11y`, `product`, `ui`,
`shared`, `infra`, and `cli`.

!!! note "There is no `csp` module"
    For cloud and deployment infrastructure, use **`infra`**.

## Commits

```
<type>(<module>): <description>

feat(stack): implement the stack install workflow
fix(cicd): preserve PVCs on pipeline deployment rollback
docs(infra): correct the local E2E guide
```

## PRs

The **PR Convention Check** in CI enforces the title and body.

- Title: `<type>(<module>): <description>` — the description is 1–72 characters
- Required in the body: the `## Summary`, `## Changes`, and `## Testing` sections, and a
  `Scope:` line with content

!!! warning "`gh pr create --body` does not apply the template"
    Even when you write the body yourself, you must include all four elements above.

!!! note "If you only changed the title or body, close and reopen the PR to re-run the check"
    The workflow triggers only on `opened` and `reopened`.
    ```bash
    gh pr close <number> && gh pr reopen <number>
    ```

## CHANGELOG

The **CHANGELOG Check** requires an entry that corresponds to your change. The exempt paths are
`docs/`, `e2e/`, `_test.go`, `.test.tsx`, `.spec.tsx`,
`.github/ISSUE_TEMPLATE/`, `CHANGELOG.md`, `.gitignore`, `LICENSE`, and
`README.md`.

!!! note "`website/` is not exempt"
    Changes to this docs site need a CHANGELOG entry.

## Tests

| What you want to do | Command |
|-----------|------|
| Go unit tests | `make test` (untagged tests only) |
| DB integration tests | `make test-integration` — testcontainers starts Postgres itself, so **you do not need `make dev`** |
| kind E2E | `make test-golden-path` (60 minutes) |
| Go lint | `make lint` |
| Frontend unit tests | `cd web && npx vitest run` |
| Frontend lint | `cd web && npm run lint` |

!!! warning "Without a DB, E2E tests are skipped, not failed"
    The DB-backed tests in `go test ./e2e/` have no build tag, so they run along with the regular
    tests, and when they cannot connect to `localhost:5433` they drop out with **`t.Skip`**. The
    result is `ok`, so it looks like a pass. Bring up the infrastructure with `make dev`, or, to
    run without infrastructure, use `make test-integration`, which uses testcontainers.

## Coverage

!!! note "The gate and the target are different"
    The threshold CI enforces is **49%** (`scripts/check-coverage.sh`, which counts only
    `./internal/...`); 70% is the v1 GA target.

## Architecture rules

- **Modular Monolith** — modules communicate only through public interfaces. Do not import
  another module's internal packages directly
- **Clean Architecture** — dependencies always point inward (toward the domain).
  `domain` does not import frameworks
- **DDD** — a module is a bounded context. Modules also own their tables and do not query
  another module's tables directly
- **TDD** — new features start with a test; bug fixes start with a test that reproduces the bug

## Editing this site

```bash
pip install -r website/requirements.txt
./website/stage.sh                  # Pulls in the standalone HTML guides from docs/
python3 website/check_i18n.py       # Checks that Korean and English pages pair up
cd website && mkdocs serve          # http://127.0.0.1:8000 (English under /en/)
```

**Every page exists in Korean and English.** The Korean `X.md` is the source, and the English
`X.en.md` sits next to it. When you change the Korean page, update the English page in the same PR.

- In English pages, quote the real English UI text (`web/src/i18n/en.json`)
- Link to pages by their Korean file name (`list.md`) — in the English site the plugin points the
  link at the English page. Heading anchors follow the English heading
- When you add a title to nav, add its English title to `nav_translations` in `website/mkdocs.yml`
- Run `npm run shots:docs` in `web` to capture screenshots: it writes `*.png` (Korean) and
  `*.en.png` (English) together. Both versions of a page keep the image path as `*.png`

Once a change lands on `main`, GitHub Actions deploys the site automatically. PRs only build it —
with `mkdocs build --strict`, so **CI fails if a page listed in nav is missing or an internal
link or anchor is broken.** `check_i18n.py` also fails a page whose English version is missing,
or that gained a section or image in only one language.
