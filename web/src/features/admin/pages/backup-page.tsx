import { useState } from "react";
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
import { BackupTargetDialog } from "../components/backup-target-dialog";
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
  const { data: runs } = useBackupRuns();
  const createBackup = useCreateBackup();

  // 확인 문구로 입력할 대상. 서버 기본값과 같아야 하므로 화면에서 정하지 않고
  // 가장 최근 실행이 쓴 값을 따른다 — 없으면 플랫폼 기본 네임스페이스다.
  const namespace = "nullus-app";

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
                  colSpan={5}
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

      <BackupTargetDialog
        open={dialogOpen}
        onClose={() => setDialogOpen(false)}
        namespace={namespace}
        submitting={createBackup.isPending}
        onSubmit={({ scope, confirm }) => {
          createBackup.mutate(
            { mode: "full", namespace, scope, confirm },
            { onSuccess: () => setDialogOpen(false) },
          );
        }}
      />
    </div>
  );
}
