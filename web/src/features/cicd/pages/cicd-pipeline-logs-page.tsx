import { useEffect, useMemo, useRef, useState } from "react";
import {
  CheckCircle2,
  CircleDashed,
  Clock,
  Loader2,
  Terminal,
  XCircle,
} from "lucide-react";
import { useTranslation } from "react-i18next";
import { useParams, useSearchParams } from "react-router-dom";
import { Breadcrumb } from "../../../components/shared/breadcrumb";
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
      <Breadcrumb
        items={[
          { label: "CI/CD List", path: "/cicd/list" },
          { label: `${breadcrumbName} Logs` },
        ]}
      />

      <div className="mb-5 flex items-start justify-between gap-3">
        <div className="flex items-center gap-2.5">
          <div className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(99,102,241,0.15)] text-[#818cf8]">
            <Terminal size={18} />
          </div>
          <div>
            <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
              Pipeline Logs
            </h1>
            <p className="m-0 mt-0.5 text-[13px] text-[var(--color-text-secondary)]">
              {pipeline?.name ?? pipelineId} · {pipeline?.clusterName ?? "-"} ·{" "}
              {pipeline?.namespace ?? "-"}
            </p>
          </div>
        </div>
      </div>

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
              <div className="rounded-md border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-3 py-2 text-sm text-[var(--color-text-secondary)]">
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
                      ? "border-[rgba(99,102,241,0.5)] bg-[rgba(99,102,241,0.12)]"
                      : "border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] hover:bg-[rgba(255,255,255,0.05)]",
                  )}
                >
                  <div className="mb-1 flex items-center justify-between gap-2">
                    <span className="font-mono text-[12px] font-semibold text-[#a5b4fc]">
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

        <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[#0d1117]">
          <div className="flex items-center gap-2 border-b border-[rgba(255,255,255,0.06)] bg-[rgba(255,255,255,0.02)] px-4 py-2.5">
            <div className="flex gap-1.5">
              <span className="h-3 w-3 rounded-full bg-[#ef4444]" />
              <span className="h-3 w-3 rounded-full bg-[#fbbf24]" />
              <span className="h-3 w-3 rounded-full bg-[#34d399]" />
            </div>
            <span className="ml-2 text-[11px] text-[rgba(255,255,255,0.4)]">
              deployment/{selectedDeploymentId ?? "-"}
            </span>
            {deploymentStatus?.status === "running" && (
              <span className="ml-auto flex items-center gap-1 text-[11px] text-[#fbbf24]">
                <Loader2 size={11} className="animate-spin" />
                Streaming...
              </span>
            )}
            {deploymentStatus?.status === "success" && (
              <span className="ml-auto flex items-center gap-1 text-[11px] text-[#34d399]">
                <CheckCircle2 size={11} />
                Completed
              </span>
            )}
            {deploymentStatus?.status === "failed" && (
              <span className="ml-auto flex items-center gap-1 text-[11px] text-[#f87171]">
                <XCircle size={11} />
                Failed
              </span>
            )}
          </div>

          <div
            ref={terminalRef}
            className="h-[520px] overflow-y-auto p-4 font-mono text-[13px] leading-[1.7]"
          >
            {selectedDeploymentId && lines.length === 0 && isDeploying && (
              <p className="text-[#8b949e]">Waiting for deployment output...</p>
            )}
            {selectedDeploymentId &&
              lines.length === 0 &&
              deploymentState &&
              !isDeploying && (
                <p className="text-[#8b949e]">
                  No output is available for this deployment.
                </p>
              )}
            {!selectedDeploymentId && (
              <p className="text-[#8b949e]">
                Select a deployment to view logs.
              </p>
            )}

            {steps.map((step, stepIdx) => (
              <div key={`${step.name}-${stepIdx}`} className="mb-3">
                <div className="mb-1 flex items-center gap-2 text-[11px]">
                  {step.status === "success" ? (
                    <CheckCircle2 size={11} className="text-[#3fb950]" />
                  ) : step.status === "failed" ? (
                    <XCircle size={11} className="text-[#f85149]" />
                  ) : step.status === "running" ? (
                    <Loader2
                      size={11}
                      className="animate-spin text-[#fbbf24]"
                    />
                  ) : (
                    <CircleDashed
                      size={11}
                      className="text-[rgba(255,255,255,0.3)]"
                    />
                  )}
                  <span
                    className={cn(
                      "font-semibold",
                      step.status === "success"
                        ? "text-[#3fb950]"
                        : step.status === "failed"
                          ? "text-[#f85149]"
                          : step.status === "running"
                            ? "text-[#fbbf24]"
                            : "text-[rgba(255,255,255,0.4)]",
                    )}
                  >
                    {step.name}
                  </span>
                  {step.status && (
                    <span className="text-[rgba(255,255,255,0.25)]">
                      [{step.status}]
                    </span>
                  )}
                  {step.applied_at && (
                    <span className="ml-auto text-[rgba(255,255,255,0.2)]">
                      <Clock size={10} className="mr-0.5 inline" />
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
                        ? "text-[#58a6ff]"
                        : line.includes("created")
                          ? "text-[#3fb950]"
                          : line.includes("configured")
                            ? "text-[#d29922]"
                            : line.includes("error") || line.includes("failed")
                              ? "text-[#f85149]"
                              : "text-[#c9d1d9]",
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
                        ? "text-[#f85149]"
                        : "text-[#c9d1d9]",
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
