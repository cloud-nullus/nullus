import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { FolderOpen } from "lucide-react";
import { iconProps } from "../../../components/ui/icon";
import { Badge } from "../../../components/ui/badge";
import { Button } from "../../../components/ui/button";
import { Checkbox } from "../../../components/ui/checkbox";
import { Modal } from "../../../components/ui/modal";
import { Skeleton } from "../../../components/ui/skeleton";
import { thClass } from "../../../components/shared/table-chrome";
import { cn } from "../../../lib/utils";
import {
  useChangeUpgradeBundleStatus,
  useUpgradeBundles,
} from "../../stack/api/stack-api";
import type {
  UpgradeBundle,
  UpgradeBundleStatus,
} from "../../stack/api/stack-api-types";
import {
  UPGRADE_TOOL_PRESETS,
  UpgradeBundleRegisterModal,
  apiErrorMessage,
} from "./upgrade-bundle-register-modal";

// 업그레이드 번들 목록(관리자). 스택의 "버전 업그레이드" 탭은 여기서 verified 인 번들만 후보로 보여 준다.
// 상태 변경은 도메인 규칙을 따른다 — draft · deprecated → verified(리허설 확인 필요), verified → deprecated,
// revoked 는 끝이다.

const STATUS_BADGE: Record<UpgradeBundleStatus, string> = {
  draft:
    "bg-[color-mix(in_srgb,_var(--color-text-secondary)_15%,_transparent)] text-[var(--color-text-secondary)]",
  verified:
    "bg-[color-mix(in_srgb,_var(--color-success)_15%,_transparent)] text-[var(--color-success)]",
  deprecated:
    "bg-[color-mix(in_srgb,_var(--color-warning)_15%,_transparent)] text-[var(--color-warning)]",
  revoked:
    "bg-[color-mix(in_srgb,_var(--color-error)_15%,_transparent)] text-[var(--color-error)]",
};

const RISK_BADGE: Record<UpgradeBundle["risk_level"], string> = {
  low: "bg-[color-mix(in_srgb,_var(--color-success)_15%,_transparent)] text-[var(--color-success)]",
  medium:
    "bg-[color-mix(in_srgb,_var(--color-warning)_15%,_transparent)] text-[var(--color-warning)]",
  high: "bg-[color-mix(in_srgb,_var(--color-error)_15%,_transparent)] text-[var(--color-error)]",
};

/** 상태에서 갈 수 있는 다음 상태. 도메인(UpgradeBundle.ChangeStatus)과 같은 표다. */
export function nextBundleStatuses(
  status: UpgradeBundleStatus | undefined,
): UpgradeBundleStatus[] {
  switch (status) {
    case "draft":
      return ["verified", "revoked"];
    case "verified":
      return ["deprecated", "revoked"];
    case "deprecated":
      return ["verified", "revoked"];
    default:
      return [];
  }
}

function versionLabel(chart: string, app?: string) {
  return app ? `${chart} (${app})` : chart;
}

interface Props {
  registerOpen: boolean;
  onRegisterClose: () => void;
}

export function UpgradeBundlesPanel({ registerOpen, onRegisterClose }: Props) {
  const { t } = useTranslation();
  const bundles = useUpgradeBundles();
  const change = useChangeUpgradeBundleStatus();
  const [pending, setPending] = useState<{
    bundle: UpgradeBundle;
    to: UpgradeBundleStatus;
  } | null>(null);
  const [confirmed, setConfirmed] = useState(false);
  const [changeError, setChangeError] = useState("");

  const items = useMemo(
    () =>
      (bundles.data?.items ?? [])
        .slice()
        .sort(
          (a, b) =>
            a.tool.localeCompare(b.tool) ||
            a.source_chart_version.localeCompare(b.source_chart_version) ||
            a.id.localeCompare(b.id),
        ),
    [bundles.data],
  );

  const b = (key: string, options?: Record<string, unknown>) =>
    t(`stackVersionsAdmin.bundles.${key}`, options);

  const openChange = (bundle: UpgradeBundle, to: UpgradeBundleStatus) => {
    setPending({ bundle, to });
    setConfirmed(false);
    setChangeError("");
  };

  const submitChange = async () => {
    if (!pending) return;
    try {
      await change.mutateAsync({
        id: pending.bundle.id,
        status: pending.to,
        rehearsalConfirmed: confirmed,
      });
      toast.success(b("change.done"));
      setPending(null);
    } catch (error) {
      setChangeError(apiErrorMessage(error) || b("change.failed"));
    }
  };

  const needsConfirmation = pending?.to === "verified";

  return (
    <div
      className="flex flex-col gap-4 p-6"
      data-testid="upgrade-bundles-panel"
    >
      <p className="m-0 max-w-3xl text-[13px] text-[var(--color-text-secondary)]">
        {b("subtitle")}
      </p>

      {bundles.isLoading && (
        <div
          className="flex flex-col gap-2"
          data-testid="upgrade-bundles-loading"
        >
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
        </div>
      )}
      {bundles.isError && (
        <div className="text-xs text-[var(--color-error)]">
          {b("loadError")}
        </div>
      )}
      {!bundles.isLoading && !bundles.isError && items.length === 0 && (
        <div
          className="flex flex-col items-center gap-2 rounded-md border border-[var(--color-border-default)] py-12 text-center text-[var(--color-text-secondary)]"
          data-testid="upgrade-bundles-empty"
        >
          <FolderOpen {...iconProps("lg")} className="opacity-40" />
          <p className="m-0 text-sm">{b("empty.title")}</p>
          <p className="m-0 text-[11px]">{b("empty.hint")}</p>
        </div>
      )}

      {items.length > 0 && (
        <div className="overflow-x-auto rounded-md border border-[var(--color-border-default)]">
          <table className="min-w-full text-left text-xs">
            <thead className="bg-[color-mix(in_srgb,_var(--color-text-primary)_3%,_transparent)] text-[var(--color-text-secondary)]">
              <tr>
                <th className={thClass}>{b("col.tool")}</th>
                <th className={thClass}>{b("col.path")}</th>
                <th className={thClass}>{b("col.risk")}</th>
                <th className={thClass}>{b("col.backup")}</th>
                <th className={thClass}>{b("col.cves")}</th>
                <th className={thClass}>{b("col.status")}</th>
                <th className={thClass}>{b("col.createdAt")}</th>
                <th className="px-3 py-2 text-right">{b("col.actions")}</th>
              </tr>
            </thead>
            <tbody>
              {items.map((bundle) => {
                const status = bundle.status ?? "verified";
                return (
                  <tr
                    key={bundle.id}
                    className="border-t border-[var(--color-border-default)] align-top"
                    data-testid={`upgrade-bundle-${bundle.id}`}
                  >
                    <td className="px-3 py-2">
                      <div className="text-[var(--color-text-primary)]">
                        {UPGRADE_TOOL_PRESETS[bundle.tool]?.label ??
                          bundle.tool}
                      </div>
                      <div className="font-mono text-[11px] text-[var(--color-text-secondary)]">
                        {bundle.id}
                      </div>
                    </td>
                    <td className="px-3 py-2 font-mono text-[11px] text-[var(--color-text-primary)]">
                      {versionLabel(
                        bundle.source_chart_version,
                        bundle.source_app_version,
                      )}
                      {" → "}
                      <strong>
                        {versionLabel(
                          bundle.target_chart_version,
                          bundle.target_app_version,
                        )}
                      </strong>
                      {bundle.expected_disruption && (
                        <div className="font-sans text-[11px] text-[var(--color-text-secondary)]">
                          {bundle.expected_disruption}
                        </div>
                      )}
                    </td>
                    <td className="px-3 py-2">
                      <Badge
                        pill
                        className={cn(
                          "rounded-full px-2 py-0.5 text-[10px]",
                          RISK_BADGE[bundle.risk_level],
                        )}
                      >
                        {t(`stackList.upgrade.risk.${bundle.risk_level}`)}
                      </Badge>
                    </td>
                    <td className="px-3 py-2 text-[var(--color-text-primary)]">
                      {bundle.requires_backup
                        ? b("backupRequired")
                        : b("backupOptional")}
                    </td>
                    <td className="px-3 py-2 font-mono text-[11px] text-[var(--color-text-primary)]">
                      {bundle.fixes_cves && bundle.fixes_cves.length > 0
                        ? bundle.fixes_cves.join(", ")
                        : "—"}
                    </td>
                    <td className="px-3 py-2">
                      <Badge
                        pill
                        className={cn(
                          "rounded-full px-2 py-0.5 text-[10px] uppercase",
                          STATUS_BADGE[status],
                        )}
                        aria-label={`status ${status}`}
                      >
                        {b(`status.${status}`)}
                      </Badge>
                      <div className="mt-1 max-w-[220px] text-[11px] text-[var(--color-text-secondary)]">
                        {b(`statusHint.${status}`)}
                      </div>
                    </td>
                    <td className="px-3 py-2 text-[var(--color-text-secondary)]">
                      {bundle.created_at
                        ? new Date(bundle.created_at).toLocaleDateString()
                        : "—"}
                    </td>
                    <td className="px-3 py-2 text-right">
                      <div className="flex flex-wrap justify-end gap-1.5">
                        {nextBundleStatuses(status).map((to) => (
                          <Button
                            key={to}
                            type="button"
                            variant="outline"
                            size="sm"
                            onClick={() => openChange(bundle, to)}
                          >
                            {b(`action.${to}`)}
                          </Button>
                        ))}
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      <UpgradeBundleRegisterModal
        open={registerOpen}
        onClose={onRegisterClose}
      />

      {pending && (
        <Modal
          open
          onClose={() => setPending(null)}
          title={b("change.title")}
          footer={
            <>
              <Button variant="outline" onClick={() => setPending(null)}>
                {t("common.cancel", "Cancel")}
              </Button>
              <Button
                variant="primary"
                disabled={needsConfirmation && !confirmed}
                loading={change.isPending}
                onClick={() => void submitChange()}
              >
                {b("change.confirm")}
              </Button>
            </>
          }
        >
          <div className="flex flex-col gap-3 text-[13px]">
            <p className="m-0 text-[var(--color-text-primary)]">
              {b("change.description", {
                id: pending.bundle.id,
                from: b(`status.${pending.bundle.status ?? "verified"}`),
                to: b(`status.${pending.to}`),
              })}
            </p>
            <p className="m-0 text-[var(--color-text-secondary)]">
              {b(`statusHint.${pending.to}`)}
            </p>
            {pending.to === "revoked" && (
              <p className="m-0 text-[var(--color-warning)]">
                {b("change.revokeWarning")}
              </p>
            )}
            {needsConfirmation && (
              <label className="flex items-start gap-2">
                <Checkbox
                  align="start"
                  checked={confirmed}
                  onChange={(e) => setConfirmed(e.target.checked)}
                  aria-label={b("rehearsal.label")}
                />
                <span>
                  <span className="text-[var(--color-text-primary)]">
                    {b("rehearsal.label")}
                  </span>
                  <span className="block text-[12px] text-[var(--color-text-secondary)]">
                    {b("rehearsal.hint")}
                  </span>
                </span>
              </label>
            )}
            {changeError && (
              <div className="text-[var(--color-error)]" role="alert">
                {changeError}
              </div>
            )}
          </div>
        </Modal>
      )}
    </div>
  );
}
