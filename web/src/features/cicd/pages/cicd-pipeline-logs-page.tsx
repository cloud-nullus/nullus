import { useEffect, useMemo, useRef, useState } from "react";
import { CircleCheck, CircleDashed, CircleX, Clock, LoaderCircle, Terminal } from 'lucide-react';
import { iconProps } from '../../../components/ui/icon'
import { useTranslation } from "react-i18next";
import { useParams, useSearchParams } from "react-router-dom";
import { cn } from "../../../lib/utils";
import {
  useDeploymentStatus,
  usePipelineDeployments,
  usePipelines,
} from "../api/cicd-api";
import { formatDateTime, resolveLocale } from "../../../lib/locale";
import {
  getPipelineStatusLabel,
  getPipelineStatusStyle,
} from "../utils/pipeline-status";
import { PageHeader } from '../../../components/layout/page-header'

export function CicdPipelineLogsPage() {
  const { t, i18n } = useTranslation();
  const locale = resolveLocale(i18n.resolvedLanguage || i18n.language);
  const { id: pipelineId = "" } = useParams<{ id: string }>();
  const [searchParams] = useSearchParams();
  const initialDeploymentId = searchParams.get("deploymentId");
  const terminalRef = useRef<HTMLDivElement>(null);
  const [selectedDeploymentId, setSelectedDeploymentId] = useState<
    string | null
  >(initialDeploymentId);

  const { data: pipelinesData } = usePipelines();
  const pipeline = (pipelinesData?.items ?? []).find(
    (p) => p.id === pipelineId,
  );
  const { data: deploymentsData } = usePipelineDeployments(pipelineId);

  const deployments = useMemo(
    () =>
      [...(deploymentsData?.items ?? [])].sort(
        (a, b) =>
          new Date(b.startedAt).getTime() - new Date(a.startedAt).getTime(),
      ),
    [deploymentsData?.items],
  );

  useEffect(() => {
    if (!selectedDeploymentId && deployments[0]?.id) {
      setSelectedDeploymentId(deployments[0].id);
    }
  }, [deployments, selectedDeploymentId]);

  const { data: deploymentStatus } = useDeploymentStatus(selectedDeploymentId);
  const steps = deploymentStatus?.steps ?? [];
  const lines = steps.flatMap((step) => step.logs ?? []);
  const deploymentState = deploymentStatus?.status ?? "";
  const isDeploying =
    deploymentState === "running" || deploymentState === "pending";

  useEffect(() => {
    const el = terminalRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  });

  const breadcrumbName = pipeline?.name ?? pipelineId;
  const currentStatus = getPipelineStatusStyle(pipeline?.status ?? "pending");
  const currentStatusLabel = getPipelineStatusLabel(
    t,
    pipeline?.status ?? "pending",
  );

  return (
    <div>
      <PageHeader
        breadcrumb={
          [
            { label: "CI/CD List", path: "/cicd/list" },
            { label: `${breadcrumbName} Logs` },
          ]
        }
        icon={<Terminal {...iconProps('sm')} />}
        tone="primary"
        title="Pipeline Logs"
        subtitle={
          <>
            {pipeline?.name ?? pipelineId} · {pipeline?.clusterName ?? "-"} ·{" "}
            {pipeline?.namespace ?? "-"}
          </>
        }
      />

      <div className="mb-4 flex flex-wrap items-center gap-3">
        <span
          className="rounded-md px-2.5 py-1 text-xs font-semibold"
          style={{
            backgroundColor: currentStatus.bg,
            color: currentStatus.color,
          }}
        >
          {currentStatusLabel}
        </span>
        <span className="text-xs text-[var(--color-text-secondary)]">
          Cluster: {pipeline?.clusterName ?? "-"}
        </span>
        <span className="text-xs text-[var(--color-text-secondary)]">
          Namespace: {pipeline?.namespace ?? "-"}
        </span>
      </div>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-[280px_1fr]">
        <div className="rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-3.5">
          <p className="mb-2 mt-0 text-[12px] font-semibold uppercase tracking-[0.04em] text-[var(--color-text-secondary)]">
            Recent Deployments
          </p>
          <div className="flex max-h-[520px] flex-col gap-2 overflow-y-auto pr-1">
            {deployments.length === 0 && (
              <div className="rounded-md border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] px-3 py-2 text-sm text-[var(--color-text-secondary)]">
                {t(
                  "cicdPipelineLogsPage.emptyDeployments",
                  "No deployment history.",
                )}
              </div>
            )}
            {deployments.map((deployment) => {
              const st = getPipelineStatusStyle(deployment.status);
              const selected = deployment.id === selectedDeploymentId;
              return (
                <button
                  key={deployment.id}
                  type="button"
                  onClick={() => setSelectedDeploymentId(deployment.id)}
                  className={cn(
                    "cursor-pointer rounded-md border px-3 py-2 text-left transition-colors",
                    selected
                      ? "border-[color-mix(in_srgb,_var(--color-primary)_50%,_transparent)] bg-[color-mix(in_srgb,_var(--color-primary)_12%,_transparent)]"
                      : "border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] hover:bg-[color-mix(in_srgb,_var(--color-text-primary)_5%,_transparent)]",
                  )}
                >
                  <div className="mb-1 flex items-center justify-between gap-2">
                    <span className="font-mono text-[12px] font-semibold text-[var(--color-primary)]">
                      {deployment.version}
                    </span>
                    <span
                      className="rounded px-1.5 py-[2px] text-[10px] font-semibold"
                      style={{ backgroundColor: st.bg, color: st.color }}
                    >
                      {getPipelineStatusLabel(t, deployment.status)}
                    </span>
                  </div>
                  <div className="text-[12px] text-[var(--color-text-secondary)]">
                    {formatDateTime(deployment.startedAt, locale)}
                  </div>
                </button>
              );
            })}
          </div>
        </div>

        <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-terminal-bg)]">
          <div className="flex items-center gap-2 border-b border-[color-mix(in_srgb,_var(--color-text-primary)_6%,_transparent)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] px-4 py-2.5">
            <div className="flex gap-1.5">
              <span className="h-3 w-3 rounded-full bg-[var(--color-error)]" />
              <span className="h-3 w-3 rounded-full bg-[var(--color-warning)]" />
              <span className="h-3 w-3 rounded-full bg-[var(--color-success)]" />
            </div>
            <span className="ml-2 text-[11px] text-[color-mix(in_srgb,_var(--color-text-primary)_40%,_transparent)]">
              deployment/{selectedDeploymentId ?? "-"}
            </span>
            {deploymentStatus?.status === "running" && (
              <span className="ml-auto flex items-center gap-1 text-[11px] text-[var(--color-warning)]">
                <LoaderCircle {...iconProps('xs')} className="animate-spin" />
                Streaming...
              </span>
            )}
            {deploymentStatus?.status === "success" && (
              <span className="ml-auto flex items-center gap-1 text-[11px] text-[var(--color-success)]">
                <CircleCheck {...iconProps('xs')} />
                Completed
              </span>
            )}
            {deploymentStatus?.status === "failed" && (
              <span className="ml-auto flex items-center gap-1 text-[11px] text-[var(--color-error)]">
                <CircleX {...iconProps('xs')} />
                Failed
              </span>
            )}
          </div>

          <div
            ref={terminalRef}
            className="h-[520px] overflow-y-auto p-4 font-mono text-[13px] leading-[1.7]"
          >
            {selectedDeploymentId && lines.length === 0 && isDeploying && (
              <p className="text-[var(--color-terminal-muted)]">Waiting for deployment output...</p>
            )}
            {selectedDeploymentId &&
              lines.length === 0 &&
              deploymentState &&
              !isDeploying && (
                <p className="text-[var(--color-terminal-muted)]">
                  No output is available for this deployment.
                </p>
              )}
            {!selectedDeploymentId && (
              <p className="text-[var(--color-terminal-muted)]">
                Select a deployment to view logs.
              </p>
            )}

            {steps.map((step, stepIdx) => (
              <div key={`${step.name}-${stepIdx}`} className="mb-3">
                <div className="mb-1 flex items-center gap-2 text-[11px]">
                  {step.status === "success" ? (
                    <CircleCheck {...iconProps('xs')} className="text-[var(--color-terminal-success)]" />
                  ) : step.status === "failed" ? (
                    <CircleX {...iconProps('xs')} className="text-[var(--color-terminal-error)]" />
                  ) : step.status === "running" ? (
                    <LoaderCircle
                      {...iconProps('xs')}
                      className="animate-spin text-[var(--color-warning)]"
                    />
                  ) : (
                    <CircleDashed
                      {...iconProps('xs')}
                      className="text-[color-mix(in_srgb,_var(--color-text-primary)_30%,_transparent)]"
                    />
                  )}
                  <span
                    className={cn(
                      "font-semibold",
                      step.status === "success"
                        ? "text-[var(--color-terminal-success)]"
                        : step.status === "failed"
                          ? "text-[var(--color-terminal-error)]"
                          : step.status === "running"
                            ? "text-[var(--color-warning)]"
                            : "text-[color-mix(in_srgb,_var(--color-text-primary)_40%,_transparent)]",
                    )}
                  >
                    {step.name}
                  </span>
                  {step.status && (
                    <span className="text-[color-mix(in_srgb,_var(--color-text-primary)_25%,_transparent)]">
                      [{step.status}]
                    </span>
                  )}
                  {step.applied_at && (
                    <span className="ml-auto text-[color-mix(in_srgb,_var(--color-text-primary)_20%,_transparent)]">
                      <Clock {...iconProps('xs')} className="mr-0.5 inline" />
                      {step.applied_at}
                    </span>
                  )}
                </div>
                {(step.logs ?? []).map((line, lineIdx) => (
                  <div
                    key={`${step.name}-${lineIdx}`}
                    className={cn(
                      "pl-4",
                      line.startsWith("$")
                        ? "text-[var(--color-terminal-info)]"
                        : line.includes("created")
                          ? "text-[var(--color-terminal-success)]"
                          : line.includes("configured")
                            ? "text-[var(--color-terminal-warning)]"
                            : line.includes("error") || line.includes("failed")
                              ? "text-[var(--color-terminal-error)]"
                              : "text-[var(--color-terminal-text)]",
                    )}
                  >
                    {line}
                  </div>
                ))}
                {(step.logs ?? []).length === 0 && step.message && (
                  <div
                    className={cn(
                      "pl-4 text-[12px]",
                      step.status === "failed"
                        ? "text-[var(--color-terminal-error)]"
                        : "text-[var(--color-terminal-text)]",
                    )}
                  >
                    {step.message}
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
