import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { DatabaseBackup } from "lucide-react";
import { PageHeader } from "../../../components/layout/page-header";
import { Button } from "../../../components/ui/button";
import { Badge } from "../../../components/ui/badge";
import { iconProps } from "../../../components/ui/icon";
import {
  tableHeadRowClass,
  thClass,
} from "../../../components/shared/table-chrome";
import {
  BackupTargetDialog,
  type BackupStackOption,
} from "../components/backup-target-dialog";
import { useStacks } from "../../stack/api/stack-api";
import {
  useBackupRuns,
  useCreateBackup,
  type BackupRun,
  type BackupStatus,
} from "../api/backup-api";

const STATUS_BADGE: Record<BackupStatus, string> = {
  pending:
    "bg-[color-mix(in_srgb,_var(--color-info)_15%,_transparent)] text-[var(--color-info)]",
  running:
    "bg-[color-mix(in_srgb,_var(--color-info)_15%,_transparent)] text-[var(--color-info)]",
  succeeded:
    "bg-[color-mix(in_srgb,_var(--color-success)_15%,_transparent)] text-[var(--color-success)]",
  // partial 은 성공이 아니다. 초록으로 칠하면 빠진 산출물을 아무도 보지 않는다.
  partial:
    "bg-[color-mix(in_srgb,_var(--color-warning)_15%,_transparent)] text-[var(--color-warning)]",
  failed:
    "bg-[color-mix(in_srgb,_var(--color-error)_15%,_transparent)] text-[var(--color-error)]",
};

function formatBytes(bytes: number): string {
  if (!bytes) return "—";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`;
}

export function BackupPage() {
  const { t } = useTranslation();
  const [dialogOpen, setDialogOpen] = useState(false);
  const { data: runs = [] } = useBackupRuns();
  const createBackup = useCreateBackup();
  const { data: stackList } = useStacks();

  // 백업할 수 있는 스택. 스택 클러스터의 리소스·볼륨·금고는 스택 ID 가 있어야 서버가
  // 그 클러스터에 닿는다 — 예전 화면은 네임스페이스를 nullus-app 으로 고정하고 스택을
  // 보내지 않아, 스택 백업이 되지 않았다.
  const stacks: BackupStackOption[] = useMemo(
    () =>
      (stackList?.items ?? [])
        .filter((s) => s.status !== "deleted" && s.namespace)
        .map((s) => ({ id: s.id, name: s.name, namespace: s.namespace ?? "" })),
    [stackList],
  );
  const stackNames = useMemo(
    () => new Map((stackList?.items ?? []).map((s) => [s.id, s.name])),
    [stackList],
  );

  return (
    <div>
      <PageHeader
        breadcrumb={[{ label: t("backupPage.breadcrumb.current", "Backup") }]}
        icon={<DatabaseBackup {...iconProps("sm")} />}
        title={t("backupPage.title", "Backup")}
        subtitle={t(
          "backupPage.description",
          "Run a backup and review past runs.",
        )}
        actions={
          <Button onClick={() => setDialogOpen(true)}>
            {t("backupPage.run", "Run backup")}
          </Button>
        }
      />

      <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)]">
        <table className="w-full text-sm">
          <thead>
            <tr className={tableHeadRowClass}>
              <th className={thClass}>
                {t("backupPage.table.startedAt", "Started")}
              </th>
              <th className={thClass}>
                {t("backupPage.table.target", "Target")}
              </th>
              <th className={thClass}>{t("backupPage.table.mode", "Mode")}</th>
              <th className={thClass}>{t("backupPage.table.size", "Size")}</th>
              <th className={thClass}>
                {t("backupPage.table.quiesce", "Downtime")}
              </th>
              <th className={thClass}>
                {t("backupPage.table.status", "Status")}
              </th>
            </tr>
          </thead>
          <tbody>
            {runs.length === 0 && (
              <tr>
                <td
                  colSpan={6}
                  className="p-6 text-center text-[var(--color-text-secondary)]"
                >
                  {t("backupPage.empty", "No backups yet.")}
                </td>
              </tr>
            )}
            {runs.map((run: BackupRun) => (
              <tr
                key={run.id}
                className="border-t border-[var(--color-border-default)]"
              >
                <td className="p-3">
                  {new Date(run.created_at).toLocaleString()}
                </td>
                <td className="p-3">
                  {run.stack_id
                    ? (stackNames.get(run.stack_id) ?? run.stack_id)
                    : t("backupPage.table.platform", "Platform")}
                </td>
                <td className="p-3">{run.mode}</td>
                <td className="p-3">{formatBytes(run.total_bytes)}</td>
                <td className="p-3">
                  {run.quiesce_seconds
                    ? `${run.quiesce_seconds.toFixed(1)}s`
                    : "—"}
                </td>
                <td className="p-3">
                  <Badge className={STATUS_BADGE[run.status]}>
                    {run.status}
                  </Badge>
                  {run.error && (
                    <span className="ml-2 text-xs text-[var(--color-text-secondary)]">
                      {run.error}
                    </span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* 열 때마다 새로 만든다 — 앞서 고른 대상·확인 문구가 남으면 다른 스택으로 착각한다. */}
      {dialogOpen && (
        <BackupTargetDialog
          open
          onClose={() => setDialogOpen(false)}
          stacks={stacks}
          submitting={createBackup.isPending}
          onSubmit={({ stackId, namespace, scope, confirm }) => {
            createBackup.mutate(
              {
                mode: stackId ? "full" : "platform_only",
                stack_id: stackId,
                namespace,
                scope,
                confirm,
              },
              { onSuccess: () => setDialogOpen(false) },
            );
          }}
        />
      )}
    </div>
  );
}
