# Backup & Restore

<span class="route">/admin/backup</span>
<span class="role role-admin">admin</span>

Back up the platform and stack state to S3-compatible storage, and restore it when you need to.
In the screen's own words: "Run a backup and review past runs."

!!! warning "Restore has no screen"
    You run backups from the screen, but **restore has no UI yet.** There is no restore
    screen, hook, or API client anywhere under `web/src/`; you have to call
    `POST /api/v1/admin/restores` directly. The procedure is [below](#restore).

## How to back up

1. Click **Run backup** in the top right.
2. In the **Select backup targets** dialog, choose a scope.
3. Check the items to include (the six below).
4. If you include volumes, you must type a confirmation phrase.
5. Click **Start backup**.

Backups are **synchronous.** By the time the request returns 201, the work is already
done. The screen re-reads the list every 5 seconds, but only while a run is in progress.

### Three backup scopes

| Choice | Mode value | Contents |
|------|---------|---------|
| **Everything — platform + all stacks** | backup set | 1 run for the platform + 1 per stack |
| A single stack (shown with its namespace) | `full` | The platform + that stack |
| **Platform only (no stack)** | `platform_only` | Platform DB and Keycloak DB |

When you back up a single stack, the first stack is selected by default and all six items
are checked. If you pick **Platform only**, the four stack-only items are disabled. If you
pick **Everything**, item selection itself is left to the server.

### Six items you can include

| Item | What goes in |
|------|----------------|
| `platform_db` | The Nullus platform PostgreSQL (`pg_dump --format=custom`) |
| `keycloak_db` | Keycloak's own database |
| `openbao_kv` | A logical, recursive export of the **per-stack** OpenBao KV (not a raft snapshot) |
| `ns_resources` | Namespace resources — Deployment, StatefulSet, Service, ConfigMap, **Secret**, Ingress, Gateway/HTTPRoute, ESO, Argo CD, and Prometheus operator objects, plus the cluster-scoped objects owned by Helm releases in that namespace (CRD, ClusterRole, ClusterRoleBinding), plus Argo CD destination namespaces and registry pull Secrets. **PVCs are deliberately excluded** (the volume item covers them) |
| `shared_resources` | Cluster-wide shared resources that are reclaimed together with the last stack — Gateway API CRDs and GatewayClass, the internal CA (ClusterIssuer, Certificate, Secret) |
| `volume` | A `tar` of **every PVC** in the namespace, taken through a helper pod |

!!! note "Helm release state is not captured separately"
    Release Secrets (`sh.helm.release.v1.*`) are included in `ns_resources`, and the
    chart name, version, and revision are recorded in the manifest. The cert-manager and
    metrics-server releases themselves are not backed up; they are **reinstalled during restore**.

### Including volumes stops the service

When the volume item is included, every Deployment and StatefulSet in that namespace is
**scaled down to 0 replicas**. Once the pods are gone, the volumes are archived, and then
the workloads are scaled back up. They are scaled back up on failure too.

That is why the screen shows an orange warning and asks for a confirmation phrase.

| Scope | Phrase to type |
|------|------------|
| A single stack | Its **namespace name** |
| Everything | `all` |

The server checks the same rule again. If the phrase is wrong, it rejects the request with
`400 BACKUP_CONFIRM_REQUIRED` before creating a run. The actual time spent stopped is
recorded in the table's **Downtime** column.

## History table

| Column | Content |
|------|------|
| Started | When the run was created |
| Target | The stack name, or `Platform` for the platform. Runs that belong to a set get a `Set <first 8 chars>` badge |
| Mode | `full` · `platform_only` · `stack_only` |
| Size | `total_bytes` in human-readable units |
| Downtime | Seconds stopped because of volumes. `—` if none |
| Status | `pending`·`running` (blue) · `succeeded` (green) · `partial` (**orange**) · `failed` (red) |

`partial` means only some items succeeded. Not coloring it green is intentional — it has to
stand apart from a backup that captured everything. The failure reason appears in small
text next to the status.

## Integrity check (verify)

```
POST /api/v1/admin/backups/{id}/verify
```

The response is `{backup_run_id, ok, checked_count, schema_check, missing_objects[], checksum_fails[]}`.

!!! warning "verify does not decrypt anything"
    For each artifact, it checks that **the object exists** in storage, that the sha256
    written to the object metadata at upload time **matches the DB record**, and that the
    schema version is restorable. It does **not** download, re-hash, or decrypt. So it
    cannot catch bytes that were later corrupted or truncated in storage. Those surface
    during an actual restore, when the data is decrypted (each chunk's "final" flag is bound
    in as AAD, so truncated ciphertext will not open).

## Backup sets

A single backup run holds **either the platform or one stack** — never two stacks in one
run, because each stack has its own cluster and its own vault.

Choosing **Everything** creates several runs that share a `set_id`.

- Order: the platform (`platform_only`) first, then one per completed stack (`stack_only` + that stack's vault)
- They run one after another, synchronously. If one fails, the rest continue
- The set status is not stored; it is computed on demand — `running` if any run is in progress, `failed` if all failed, `succeeded` if all succeeded, `partial` otherwise

## Restore

Restore is API-only.

```bash
curl -X POST "$API/api/v1/admin/restores" \
  -H 'Content-Type: application/json' \
  -d '{"backup_run_id":"<backup ID>","confirm":"<same backup ID>","namespace":"<stack namespace>"}'
```

- Put **the backup ID itself** in `confirm`. If it differs, you get `400 RESTORE_CONFIRM_REQUIRED`.
- If the backup has a `stack_id` and its mode is not `platform_only`, `namespace` is required.
- Track progress with `GET /api/v1/admin/restores/{id}`.

### What restore does (actual order)

1. **Preconditions** — `ENCRYPTION_KEY` (32 bytes), the backup sealing key (32 bytes), destination credentials, and a destination reachability check. If any of them is missing, it stops without touching anything
2. Checks that the backup is `succeeded` or `partial`
3. **Artifact integrity pre-check** (same method as verify)
4. **Schema check** — aborts if the current DB is dirty, or if the backup's version is newer than the current one
5. Chart version differences are only logged as warnings; they do not block the restore
6. Stack side — ensure the namespace → apply **the internal CA Secret first** → reinstall cert-manager and metrics-server → apply shared resources, **only the ones that are missing** → recreate PVCs with the size and StorageClass from the manifest → restore the volume tars → apply namespace resources
7. Platform side — `pg_restore --clean --if-exists`. **The backup history tables are not restored**. Then the Keycloak DB (a failure here is not fatal)
8. **Scales** workloads back to the original replica counts in the manifest
9. Imports the **stack vault KV after that** (retries every 10 seconds for up to 2 minutes by default; the restore still succeeds if this fails)
10. Referential integrity check → `integrity_report`
11. **Re-pins the gateway address** — rewrites the `hostAliases` of the runners and Argo CD to the current gateway address

!!! danger "Restore does not delete what is already there"
    It does **not delete** namespaces, releases, or PVCs. It checks that the namespace
    exists and then overwrites on top of it. So resources that did not exist at backup time
    are still there after the restore — to return to a clean state, delete the namespace first.

### What `dangling` means

`integrity_report.dangling[]` in the restore result lists **the secret paths that the
restored database points to but that do not actually exist in the restored vault**
(`{token_source_id, path}`).

The restore asks the vault about each of the organization's token source paths, one by one,
and collects the missing ones. `checked_paths` is the number of paths checked. **It is only
a warning** — the restore is reported as successful, and you must register the affected
token sources again.

### Set restore

The platform is restored first. If the platform fails, the whole set stops with
`플랫폼 복구 실패 — 스택은 되돌리지 않았습니다` ("Platform restore failed — stacks were not
restored"). The stacks then proceed independently of each other.

## Storage and encryption

| Item | Value |
|------|-----|
| Location | A single S3-compatible bucket **outside the cluster** |
| Object key | `[<prefix>/]backup-<run ID>/<name>` |
| Encryption | Streaming **AES-256-GCM**, 1 MiB chunks, header `NBK1` |
| Compression | **None** — volumes are raw `tar -cpf -` output (only `pg_dump`'s internal compression applies) |
| Local disk | Not used (streamed straight to the upload through a pipe) |
| Key | Only one (`backup.seal_key`, 32 bytes). It **must differ** from `ENCRYPTION_KEY` and from the DB password; if it matches either, the server does not start |

The destination pre-check runs **before** any workloads are stopped — it does not just
check that the bucket exists, it actually PUTs and DELETEs a probe object.

!!! note "There is no manifest file in the bucket"
    The manifest lives only in a database column; you retrieve it with
    `GET /api/v1/admin/backups/{id}`. You cannot take just the bucket and restore somewhere
    else. Artifact names carry no `.enc` or `.zst` suffix either — nothing is compressed,
    and the name does not reveal whether the artifact is encrypted.

## Scheduled backups

Implemented, but **off by default**.

| Item | Default |
|------|--------|
| `backup.schedule.enabled` | `false` |
| Interval | 24 hours |
| Time limit per run | 4 hours |
| Retention | 7 daily · 4 weekly · 3 monthly |

If the previous backup is still running, that tick is skipped. The retention policy
**deletes only the objects and keeps the history rows**, and it never deletes the most
recent successful backup. Retention runs only for scheduled backups; it does not apply to
manual backups.

## Further reading

- [Backup and restore architecture (Korean)](../../guides/backup-architecture.html) — design drawings and diagrams

## What's missing today

| Missing | Notes |
|---------|------|
| Backup delete API | There is no `DELETE /backups/:id`. Only the retention policy deletes backups |
| Audit log | Backups and restores are not written to `audit_logs` — you cannot trace who restored what, or when |
| Restore screen | API only |
| Post-restore smoke test | None. Even when a restore is `succeeded`, check sign-in, git clone, image pull, and pipeline re-runs yourself |
| Key escrow and rotation | Not in the code. The sealing interface is exposed only as a BYOK replacement point |
