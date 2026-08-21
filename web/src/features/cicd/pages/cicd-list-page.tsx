import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";

import { buildStageStates, type StageState } from "../utils/stage-states";
import { useTranslation } from "react-i18next";
import {
  Activity,
  Box,
  Boxes,
  ChartColumn,
  CircleCheck,
  CircleDashed,
  CircleX,
  ExternalLink,
  Eye,
  EyeOff,
  FileCode2,
  GitBranch,
  Globe,
  History,
  Info,
  LoaderCircle,
  Package,
  Plus,
  RefreshCw,
  Rocket,
  Server,
  Terminal,
  Trash2,
  Workflow,
  X,
} from "lucide-react";
import { iconProps } from "../../../components/ui/icon";
import type { ColumnDef } from "@tanstack/react-table";
import {
  AppLogPanel,
  AppUsageCharts,
} from "../../observability/components/app-runtime-panels";
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import {
  useDeletePipeline,
  useDeploymentStatus,
  useDeployPipeline,
  usePipelineDeployments,
  usePipelineResources,
  usePipelines,
  useTemplateById,
} from "../api/cicd-api";
import {
  DeletePipelineDialog,
  type DeletePipelineSelection,
} from "../components/delete-pipeline-dialog";
import type { Pipeline } from "../api/cicd-api";
import { useScopedClusters as useClusters } from "../../admin/api/admin-api";
import { useStacks } from "../../stack/api/stack-api";
import { Button } from "../../../components/ui/button";
import { Select } from "../../../components/ui/select";
import { YamlEditor } from "../../../components/shared/yaml-editor";
import { DataTable } from "../../../components/shared/data-table";
import { Tabs } from "../../../components/ui/tabs";
import { IconButton } from "../../../components/ui/icon-button";
import { formatDate, formatDateTime, resolveLocale } from "../../../lib/locale";
import {
  getPipelineStatusLabel,
  getPipelineStatusStyle,
} from "../utils/pipeline-status";
import { cn } from "../../../lib/utils";
import { PageHeader } from "../../../components/layout/page-header";
import { SearchInput } from "../../../components/ui/search-input";
import { Badge } from "../../../components/ui/badge";

// ── Execute Modal ─────────────────────────────────────────────────────────────

type ExecuteSetupTab = "cluster" | "build" | "deploy";
type ExecuteDeployMode = "template" | "custom";

const EXECUTE_DOCKERFILE_PRESETS = [
  {
    id: "dockerfile.root",
    label: "Dockerfile (root)",
    path: "./Dockerfile",
    content: [
      "FROM node:20-alpine AS builder",
      "WORKDIR /app",
      "COPY package*.json ./",
      "RUN npm ci",
      "COPY . .",
      "RUN npm run build",
      "",
      "FROM nginx:1.27-alpine",
      "COPY --from=builder /app/dist /usr/share/nginx/html",
      "EXPOSE 80",
      'CMD ["nginx", "-g", "daemon off;"]',
    ].join("\n"),
  },
  {
    id: "dockerfile.app",
    label: "Dockerfile (app/)",
    path: "./app/Dockerfile",
    content: [
      "FROM golang:1.24-alpine AS builder",
      "WORKDIR /src",
      "COPY go.mod go.sum ./",
      "RUN go mod download",
      "COPY . .",
      "RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o app ./cmd/server",
      "",
      "FROM gcr.io/distroless/static:nonroot",
      "COPY --from=builder /src/app /app",
      "USER nonroot:nonroot",
      'ENTRYPOINT ["/app"]',
    ].join("\n"),
  },
  {
    id: "dockerfile.service",
    label: "Dockerfile (services/api/)",
    path: "./services/api/Dockerfile",
    content: [
      "FROM python:3.12-slim",
      "WORKDIR /service",
      "COPY requirements.txt .",
      "RUN pip install --no-cache-dir -r requirements.txt",
      "COPY . .",
      "EXPOSE 8080",
      'CMD ["python", "-m", "uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8080"]',
    ].join("\n"),
  },
];

const EXECUTE_DEPLOY_YAML_PRESETS = [
  {
    id: "k8s-deployment",
    label: "Kubernetes Deployment",
    description: "Deployment + Service 기본 매니페스트",
    content: [
      "apiVersion: apps/v1",
      "kind: Deployment",
      "metadata:",
      "  name: app-placeholder",
      "spec:",
      "  replicas: 2",
      "  selector:",
      "    matchLabels:",
      "      app: app-placeholder",
      "  template:",
      "    metadata:",
      "      labels:",
      "        app: app-placeholder",
      "    spec:",
      "      containers:",
      "        - name: app",
      "          image: harbor.local/app-placeholder:latest",
      "          ports:",
      "            - containerPort: 8080",
      "---",
      "apiVersion: v1",
      "kind: Service",
      "metadata:",
      "  name: app-placeholder-svc",
      "spec:",
      "  selector:",
      "    app: app-placeholder",
      "  ports:",
      "    - port: 80",
      "      targetPort: 8080",
      "  type: ClusterIP",
    ].join("\n"),
  },
  {
    id: "k8s-cronjob",
    label: "Kubernetes CronJob",
    description: "배치/스케줄 작업용 CronJob 매니페스트",
    content: [
      "apiVersion: batch/v1",
      "kind: CronJob",
      "metadata:",
      "  name: batch-placeholder",
      "spec:",
      '  schedule: "*/10 * * * *"',
      "  jobTemplate:",
      "    spec:",
      "      template:",
      "        spec:",
      "          restartPolicy: OnFailure",
      "          containers:",
      "            - name: batch",
      "              image: harbor.local/batch-placeholder:latest",
    ].join("\n"),
  },
  {
    id: "kustomize",
    label: "Kustomize Base",
    description: "Kustomization 기반 배포 구성",
    content: [
      "apiVersion: kustomize.config.k8s.io/v1beta1",
      "kind: Kustomization",
      "namespace: default",
      "resources:",
      "  - deployment.yaml",
      "  - service.yaml",
    ].join("\n"),
  },
];

const EXECUTE_SETUP_TABS: {
  id: ExecuteSetupTab;
  label: string;
  icon: React.ReactNode;
}[] = [
  { id: "cluster", label: "Cluster", icon: <Server {...iconProps("xs")} /> },
  { id: "build", label: "Build", icon: <FileCode2 {...iconProps("xs")} /> },
  { id: "deploy", label: "Deploy", icon: <Boxes {...iconProps("xs")} /> },
];

function ExecuteModal({
  pipeline,
  onClose,
  onExecute,
  isExecuting,
}: {
  pipeline: Pipeline;
  onClose: () => void;
  onExecute: () => void;
  isExecuting: boolean;
}) {
  const { data: clustersData } = useClusters();
  const { data: executeStacksData } = useStacks();
  // 스택을 아직 못 받았거나 스택 없이 만든 파이프라인이면 아무것도 붙이지
  // 않는다. 모르는 값을 그럴듯하게 지어내지 않는다.
  const executeStackLabel = pipeline.stackId
    ? ((executeStacksData?.items ?? []).find(
        (stack) => stack.id === pipeline.stackId,
      )?.name ?? pipeline.stackId)
    : "";
  const clusterList = clustersData?.items ?? [];
  const targetClusters = clusterList.filter((c) => {
    const types = Array.isArray((c as any).types)
      ? (c as any).types
      : [(c as any).type ?? ""];
    return types
      .flatMap((t: string) => t.split(","))
      .map((t: string) => t.trim().toLowerCase())
      .includes("target");
  });
  const clusterOptions =
    targetClusters.length > 0
      ? targetClusters
          .sort((a, b) => a.name.localeCompare(b.name))
          .map((c) => ({ id: c.id, name: c.name }))
      : [
          { id: "c1", name: "prod-k8s" },
          { id: "c2", name: "dev-k8s" },
        ];

  const [activeTab, setActiveTab] = useState<ExecuteSetupTab>("cluster");
  const [clusterId, setClusterId] = useState(
    pipeline.clusterId || clusterOptions[0]?.id || "",
  );
  const [dockerfileId, setDockerfileId] = useState(
    EXECUTE_DOCKERFILE_PRESETS[0].id,
  );
  const [deployMode, setDeployMode] = useState<ExecuteDeployMode>("template");
  const [deployYamlId, setDeployYamlId] = useState(
    EXECUTE_DEPLOY_YAML_PRESETS[0].id,
  );
  const [customDeployYaml, setCustomDeployYaml] = useState(
    EXECUTE_DEPLOY_YAML_PRESETS[0].content,
  );

  const selectedDockerfile =
    EXECUTE_DOCKERFILE_PRESETS.find((p) => p.id === dockerfileId) ??
    EXECUTE_DOCKERFILE_PRESETS[0];
  const selectedDeployYaml =
    EXECUTE_DEPLOY_YAML_PRESETS.find((p) => p.id === deployYamlId) ??
    EXECUTE_DEPLOY_YAML_PRESETS[0];

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
      onClick={onClose}
    >
      <div
        className="relative flex max-h-[90vh] w-full max-w-2xl flex-col overflow-hidden rounded-2xl border border-[var(--color-border-default)] bg-[var(--color-surface-card)] shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between border-b border-[var(--color-border-default)] px-5 py-4">
          <div className="flex items-center gap-2.5">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-[color-mix(in_srgb,_var(--color-primary)_15%,_transparent)] text-[var(--color-primary)]">
              <Rocket {...iconProps("sm")} />
            </div>
            <div>
              <div className="text-[15px] font-bold text-[var(--color-text-primary)]">
                Execute Pipeline
              </div>
              <div className="text-[12px] text-[var(--color-text-secondary)]">
                {pipeline.name}
                {/* 어느 스택 위에서 도는지 배포 직전에 밝힌다 — 스택마다
                    이미지가 올라가는 레지스트리가 다르다. */}
                {executeStackLabel ? ` · ${executeStackLabel}` : ""}
              </div>
            </div>
          </div>
          <IconButton onClick={onClose} aria-label="Close">
            <X {...iconProps("sm")} />
          </IconButton>
        </div>

        {/* Tabs */}
        <Tabs
          value={activeTab}
          onChange={setActiveTab}
          items={EXECUTE_SETUP_TABS.map((tab) => ({
            id: tab.id,
            icon: tab.icon,
            label: tab.label,
          }))}
        />

        {/* Tab content */}
        <div className="flex-1 overflow-y-auto p-5">
          {activeTab === "cluster" && (
            <div className="max-w-sm">
              <Select
                label="Deploy Cluster"
                value={clusterId}
                onChange={(e) => setClusterId(e.target.value)}
                className="w-full"
              >
                {clusterOptions.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </Select>
            </div>
          )}

          {activeTab === "build" && (
            <div className="flex flex-col gap-4">
              <p className="m-0 text-[13px] text-[var(--color-text-secondary)]">
                Select the Dockerfile to use in the build stage.
              </p>
              <div className="grid grid-cols-3 gap-2.5">
                {EXECUTE_DOCKERFILE_PRESETS.map((preset) => {
                  const selected = dockerfileId === preset.id;
                  return (
                    <button
                      key={preset.id}
                      type="button"
                      onClick={() => setDockerfileId(preset.id)}
                      className={cn(
                        "cursor-pointer rounded-lg border p-3 text-left transition-all duration-150",
                        selected
                          ? "border-[color-mix(in_srgb,_var(--color-primary)_50%,_transparent)] bg-[color-mix(in_srgb,_var(--color-primary)_10%,_transparent)]"
                          : "border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)]",
                      )}
                    >
                      <div
                        className={cn(
                          "text-sm font-semibold",
                          selected
                            ? "text-[var(--color-primary)]"
                            : "text-[var(--color-text-primary)]",
                        )}
                      >
                        {preset.label}
                      </div>
                      <div className="mt-1 text-xs text-[var(--color-text-secondary)]">
                        {preset.path}
                      </div>
                    </button>
                  );
                })}
              </div>
              <YamlEditor
                value={selectedDockerfile.content}
                readOnly
                height="240px"
              />
            </div>
          )}

          {activeTab === "deploy" && (
            <div className="flex flex-col gap-4">
              <div className="flex items-center gap-2 rounded-lg border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] p-2">
                {(["template", "custom"] as ExecuteDeployMode[]).map((mode) => (
                  <button
                    key={mode}
                    type="button"
                    onClick={() => setDeployMode(mode)}
                    className={cn(
                      "cursor-pointer rounded-md px-3 py-1.5 text-xs font-semibold",
                      deployMode === mode
                        ? "bg-[color-mix(in_srgb,_var(--color-primary)_20%,_transparent)] text-[var(--color-primary)]"
                        : "text-[var(--color-text-secondary)]",
                    )}
                  >
                    {mode === "template"
                      ? "Select Template YAML"
                      : "Write Custom YAML"}
                  </button>
                ))}
              </div>

              {deployMode === "template" ? (
                <>
                  <div className="grid grid-cols-3 gap-2.5">
                    {EXECUTE_DEPLOY_YAML_PRESETS.map((preset) => {
                      const selected = deployYamlId === preset.id;
                      return (
                        <button
                          key={preset.id}
                          type="button"
                          onClick={() => setDeployYamlId(preset.id)}
                          className={cn(
                            "cursor-pointer rounded-lg border p-3 text-left transition-all duration-150",
                            selected
                              ? "border-[color-mix(in_srgb,_var(--color-primary)_50%,_transparent)] bg-[color-mix(in_srgb,_var(--color-primary)_10%,_transparent)]"
                              : "border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)]",
                          )}
                        >
                          <div
                            className={cn(
                              "text-sm font-semibold",
                              selected
                                ? "text-[var(--color-primary)]"
                                : "text-[var(--color-text-primary)]",
                            )}
                          >
                            {preset.label}
                          </div>
                          <div className="mt-1 text-xs text-[var(--color-text-secondary)]">
                            {preset.description}
                          </div>
                        </button>
                      );
                    })}
                  </div>
                  <YamlEditor
                    value={selectedDeployYaml.content}
                    readOnly
                    height="240px"
                  />
                </>
              ) : (
                <YamlEditor
                  value={customDeployYaml}
                  onChange={setCustomDeployYaml}
                  height="240px"
                />
              )}
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end gap-2 border-t border-[var(--color-border-default)] px-5 py-4">
          <Button
            variant="secondary"
            size="sm"
            type="button"
            onClick={onClose}
            disabled={isExecuting}
          >
            Cancel
          </Button>
          <Button
            variant="primary"
            size="sm"
            type="button"
            onClick={onExecute}
            loading={isExecuting}
            disabled={isExecuting}
          >
            <Rocket {...iconProps("xs")} />
            Execute
          </Button>
        </div>
      </div>
    </div>
  );
}

// ── Pipeline inner tabs ────────────────────────────────────────────────────────

type PipelineInnerTab = "info" | "monitoring" | "history";

const INNER_TABS: Array<{
  key: PipelineInnerTab;
  label: string;
  icon: React.ReactNode;
}> = [
  { key: "info", label: "Info", icon: <Info {...iconProps("xs")} /> },
  {
    key: "monitoring",
    label: "Monitoring",
    icon: <ChartColumn {...iconProps("xs")} />,
  },
  { key: "history", label: "History", icon: <History {...iconProps("xs")} /> },
];

function DetailCard({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div className="rounded-lg border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] p-4">
      <div className="mb-3 text-[12px] font-semibold uppercase tracking-[0.02em] text-[var(--color-text-secondary)]">
        {title}
      </div>
      {children}
    </div>
  );
}

function ConfigRow({
  label,
  value,
}: {
  label: string;
  value: React.ReactNode;
}) {
  return (
    <div className="flex items-center justify-between gap-3 text-[13px]">
      <span className="text-[var(--color-text-secondary)]">{label}</span>
      <span className="font-semibold text-[var(--color-text-primary)]">
        {value}
      </span>
    </div>
  );
}

type PipelineResourceNode = {
  kind: string;
  name: string;
  status: string;
  labelSelector?: string;
  serviceUrls?: string[];
};

function pickResourcesByKind(
  resources: PipelineResourceNode[],
  kinds: string[],
): PipelineResourceNode[] {
  const lowered = kinds.map((kind) => kind.toLowerCase());
  return resources.filter((resource) =>
    lowered.includes(resource.kind.toLowerCase()),
  );
}

function stageMeta(state: StageState): {
  icon: React.ReactNode;
  label: string;
  cls: string;
} {
  if (state === "completed") {
    return {
      icon: <CircleCheck {...iconProps("sm")} />,
      label: "Completed",
      cls: "border-[color-mix(in_srgb,_var(--color-success)_35%,_transparent)] bg-[color-mix(in_srgb,_var(--color-success)_8%,_transparent)] text-[var(--color-success)]",
    };
  }
  if (state === "failed") {
    return {
      icon: <CircleX {...iconProps("sm")} />,
      label: "Failed",
      cls: "border-[color-mix(in_srgb,_var(--color-error)_35%,_transparent)] bg-[color-mix(in_srgb,_var(--color-error)_8%,_transparent)] text-[var(--color-error)]",
    };
  }
  if (state === "unknown") {
    return {
      icon: <CircleDashed {...iconProps("sm")} />,
      label: "실행 정보 없음",
      cls: "border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] text-[var(--color-text-secondary)]",
    };
  }
  if (state === "in_progress") {
    return {
      icon: <LoaderCircle {...iconProps("sm")} className="animate-spin" />,
      label: "In progress",
      cls: "border-[color-mix(in_srgb,_var(--color-warning)_35%,_transparent)] bg-[color-mix(in_srgb,_var(--color-warning)_8%,_transparent)] text-[var(--color-warning)]",
    };
  }
  return {
    icon: <CircleDashed {...iconProps("sm")} />,
    label: "Queued",
    cls: "border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] text-[var(--color-text-muted)]",
  };
}

function statusClass(status: string): string {
  const normalized = status.toLowerCase();
  if (normalized === "running" || normalized === "completed")
    return "bg-[color-mix(in_srgb,_var(--color-success)_18%,_transparent)] text-[var(--color-success)]";
  if (
    normalized.includes("crash") ||
    normalized === "failed" ||
    normalized === "degraded"
  )
    return "bg-[color-mix(in_srgb,_var(--color-error)_20%,_transparent)] text-[var(--color-error)]";
  if (
    normalized === "updating" ||
    normalized === "progressing" ||
    normalized === "scheduled"
  )
    return "bg-[color-mix(in_srgb,_var(--color-warning)_20%,_transparent)] text-[var(--color-warning)]";
  return "bg-[color-mix(in_srgb,_var(--color-text-secondary)_18%,_transparent)] text-[var(--color-text-secondary)]";
}

function logLineClass(line: string): string {
  const normalized = line.toLowerCase();
  if (normalized.startsWith("$")) return "text-[var(--color-terminal-info)]";
  if (
    normalized.includes("error") ||
    normalized.includes("failed") ||
    normalized.includes("panic")
  )
    return "text-[var(--color-error)]";
  if (
    normalized.includes("created") ||
    normalized.includes("applied") ||
    normalized.includes("ready") ||
    normalized.includes("running")
  )
    return "text-[var(--color-success)]";
  if (
    normalized.includes("warning") ||
    normalized.includes("progress") ||
    normalized.includes("waiting")
  )
    return "text-[var(--color-warning)]";
  return "text-[var(--color-text-secondary)]";
}

function modeLabel(mode: Pipeline["mode"]): string {
  if (mode === "ci") return "CI";
  if (mode === "cd") return "CD";
  return "CI/CD";
}

function ResourceNode({
  title,
  resources,
  accentClass,
  emptyLabel = "-",
}: {
  title: string;
  resources: PipelineResourceNode[];
  accentClass: string;
  emptyLabel?: string;
}) {
  return (
    <div className="min-h-[84px] min-w-0 overflow-hidden rounded-lg border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] p-2.5">
      <div className="mb-1.5 text-[11px] font-semibold uppercase tracking-[0.05em] text-[var(--color-text-secondary)]">
        {title}
      </div>
      {resources.length > 0 ? (
        <div className="flex flex-col gap-1.5">
          {resources.map((resource) => (
            <div
              key={`${resource.kind}-${resource.name}`}
              className="min-w-0 overflow-hidden rounded border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] px-2 py-1.5"
            >
              <div className="flex min-w-0 flex-wrap items-center gap-1.5">
                <span
                  className={`max-w-full truncate rounded px-1.5 py-0.5 font-mono text-[11px] ${accentClass}`}
                >
                  {resource.name}
                </span>
                <span
                  className={`rounded px-1.5 py-0.5 text-[10px] font-bold uppercase ${statusClass(resource.status || "unknown")}`}
                >
                  {resource.status || "unknown"}
                </span>
              </div>
              {resource.labelSelector && (
                <div className="mt-1 break-all font-mono text-[10px] text-[var(--color-text-muted)]">
                  selector: {resource.labelSelector}
                </div>
              )}
              {resource.serviceUrls && resource.serviceUrls.length > 0 && (
                <div className="mt-1 flex min-w-0 flex-wrap gap-1">
                  {resource.serviceUrls.slice(0, 2).map((url) => (
                    <code
                      key={url}
                      className="max-w-full break-all rounded bg-[color-mix(in_srgb,_var(--color-text-primary)_7%,_transparent)] px-1.5 py-[1px] text-[10px] text-[var(--color-text-secondary)]"
                    >
                      {url}
                    </code>
                  ))}
                </div>
              )}
            </div>
          ))}
        </div>
      ) : (
        <div className="text-[12px] text-[var(--color-text-muted)]">
          {emptyLabel}
        </div>
      )}
    </div>
  );
}

const MANIFEST_ENV_KEYS = new Set([
  "NULLUS_MANIFEST_DEPLOYMENT",
  "NULLUS_MANIFEST_SERVICE",
  "NULLUS_MANIFEST_INGRESS",
]);

function PipelineInfoTab({ pipeline }: { pipeline: Pipeline }) {
  const { t, i18n } = useTranslation();
  const locale = resolveLocale(i18n.resolvedLanguage || i18n.language);
  const { data: template } = useTemplateById(pipeline.templateId);
  const { data: deploymentsData, isLoading: isDeploymentsLoading } =
    usePipelineDeployments(pipeline.id);
  const { data: resourcesData, isLoading: isResourcesLoading } =
    usePipelineResources(pipeline.id);
  const [revealedVars, setRevealedVars] = useState<Set<string>>(new Set());

  const toggleReveal = (key: string) => {
    setRevealedVars((prev) => {
      const next = new Set(prev);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  };

  const deployments = deploymentsData?.items ?? [];
  const latestDeployment = [...deployments].sort(
    (a, b) => new Date(b.startedAt).getTime() - new Date(a.startedAt).getTime(),
  )[0];
  const resources = resourcesData?.items ?? [];
  const ingressResources = pickResourcesByKind(resources, ["Ingress"]);
  const serviceResources = pickResourcesByKind(resources, ["Service"]);
  const workloadResources = pickResourcesByKind(resources, [
    "Deployment",
    "StatefulSet",
  ]);
  const podResources = pickResourcesByKind(resources, ["Pod"]);
  const jobResources = pickResourcesByKind(resources, ["Job", "CronJob"]);

  const allServiceUrls = Array.from(
    new Set([
      ...ingressResources.flatMap((r) => r.serviceUrls ?? []),
      ...serviceResources.flatMap((r) => r.serviceUrls ?? []),
    ]),
  ).filter(Boolean);

  const envEntries = Object.entries(pipeline.envVars ?? {}).filter(
    ([k]) => !MANIFEST_ENV_KEYS.has(k),
  );
  const manifestEntries = Object.entries(pipeline.envVars ?? {}).filter(([k]) =>
    MANIFEST_ENV_KEYS.has(k),
  );

  const modeLabel =
    pipeline.executionMode === "ci"
      ? "CI Only"
      : pipeline.executionMode === "cd"
        ? "CD Only"
        : "CI/CD";

  return (
    <div className="flex flex-col gap-4">
      {/* service url open buttons */}
      {allServiceUrls.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {allServiceUrls.map((url) => (
            <a
              key={url}
              href={url.startsWith("http") ? url : `http://${url}`}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1.5 rounded-lg border border-[color-mix(in_srgb,_var(--color-primary)_40%,_transparent)] bg-[color-mix(in_srgb,_var(--color-primary)_10%,_transparent)] px-3 py-1.5 text-[12px] font-semibold text-[var(--color-primary)] transition-colors hover:bg-[color-mix(in_srgb,_var(--color-primary)_20%,_transparent)]"
            >
              <ExternalLink {...iconProps("xs")} />
              {url}
            </a>
          ))}
        </div>
      )}

      {/* basic info */}
      <DetailCard title="Pipeline Info">
        <div className="flex flex-col gap-2.5">
          <ConfigRow
            label="Name"
            value={<span className="font-semibold">{pipeline.name}</span>}
          />
          <ConfigRow label="App Type" value={pipeline.appType} />
          <ConfigRow
            label="Execution Mode"
            value={
              <span className="rounded-md border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-primary)_8%,_transparent)] px-2 py-[2px] text-[11px] font-semibold text-[var(--color-primary)]">
                {modeLabel}
              </span>
            }
          />
          <ConfigRow
            label="Template"
            value={template?.name ?? (pipeline.templateId || "-")}
          />
          <ConfigRow
            label="Status"
            value={
              <span
                className="rounded-md px-[9px] py-[3px] text-xs font-semibold"
                style={{
                  backgroundColor: getPipelineStatusStyle(pipeline.status).bg,
                  color: getPipelineStatusStyle(pipeline.status).color,
                }}
              >
                {getPipelineStatusLabel(t, pipeline.status)}
              </span>
            }
          />
          <ConfigRow
            label="Created"
            value={formatDateTime(pipeline.createdAt, locale)}
          />
          <ConfigRow
            label="Last Deployed"
            value={formatDateTime(pipeline.lastDeployedAt, locale)}
          />
          {/* 배포된 앱이 어디로 열리는지. 스택에 접근 도메인이 없으면 서버가
              빈 값을 주므로 줄 자체를 그리지 않는다 — 열리지 않는 링크를
              보여 주는 것이 없는 것보다 나쁘다. */}
          {pipeline.accessUrl && (
            <ConfigRow
              label={t("cicdListPage.accessUrl", "Access URL")}
              value={
                <a
                  href={pipeline.accessUrl}
                  target="_blank"
                  rel="noreferrer"
                  className="max-w-[260px] truncate text-[12px] underline"
                  title={pipeline.accessUrl}
                >
                  {pipeline.accessUrl.replace(/^https:\/\//, "")}
                </a>
              }
            />
          )}
        </div>
      </DetailCard>

      {/* code checkout */}
      <DetailCard title="Code Checkout">
        <div className="flex flex-col gap-2.5">
          <ConfigRow
            label="Git Repository"
            value={
              pipeline.gitRepoUrl ? (
                <code
                  className="max-w-[260px] truncate rounded bg-[color-mix(in_srgb,_var(--color-text-primary)_8%,_transparent)] px-2 py-[2px] text-[12px]"
                  title={pipeline.gitRepoUrl}
                >
                  {pipeline.gitRepoUrl}
                </code>
              ) : (
                "-"
              )
            }
          />
          {pipeline.stackId && (
            <ConfigRow
              label="Stack"
              value={
                <code className="rounded bg-[color-mix(in_srgb,_var(--color-text-primary)_8%,_transparent)] px-2 py-[2px] text-[12px]">
                  {pipeline.stackId}
                </code>
              }
            />
          )}
        </div>
      </DetailCard>

      {/* build */}
      {pipeline.dockerfilePath && (
        <DetailCard title="Build">
          <div className="flex flex-col gap-2.5">
            <ConfigRow
              label="Dockerfile"
              value={
                <code
                  className="max-w-[260px] truncate rounded bg-[color-mix(in_srgb,_var(--color-text-primary)_8%,_transparent)] px-2 py-[2px] text-[12px]"
                  title={pipeline.dockerfilePath}
                >
                  {pipeline.dockerfilePath}
                </code>
              }
            />
            <ConfigRow
              label="Build Context"
              value={
                <code className="rounded bg-[color-mix(in_srgb,_var(--color-text-primary)_8%,_transparent)] px-2 py-[2px] text-[12px]">
                  {pipeline.dockerContext || "."}
                </code>
              }
            />
            {(pipeline.envVars ?? {})["IMAGE_REGISTRY_URL"] && (
              <ConfigRow
                label="Image Registry"
                value={
                  <code className="max-w-[260px] truncate rounded bg-[color-mix(in_srgb,_var(--color-text-primary)_8%,_transparent)] px-2 py-[2px] text-[12px]">
                    {(pipeline.envVars ?? {})["IMAGE_REGISTRY_URL"]}
                  </code>
                }
              />
            )}
          </div>
        </DetailCard>
      )}

      {/* deployment target */}
      <DetailCard title="Deployment Target">
        <div className="flex flex-col gap-2.5">
          <ConfigRow
            label="Cluster"
            value={pipeline.clusterName || pipeline.clusterId}
          />
          <ConfigRow
            label="Namespace"
            value={
              <code className="rounded bg-[color-mix(in_srgb,_var(--color-text-primary)_8%,_transparent)] px-2 py-[2px] text-[12px]">
                {pipeline.namespace}
              </code>
            }
          />
        </div>

        <div className="mt-4 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-surface-sunken)] p-3">
          <div className="mb-2 text-[12px] font-semibold text-[var(--color-text-primary)]">
            Deployed Resources
          </div>

          {(isDeploymentsLoading || isResourcesLoading) && (
            <div className="text-[12px] text-[var(--color-text-secondary)]">
              Loading resources...
            </div>
          )}
          {!isDeploymentsLoading &&
            !isResourcesLoading &&
            deployments.length === 0 && (
              <div className="text-[12px] text-[var(--color-text-secondary)]">
                No deployment history yet.
              </div>
            )}
          {!isDeploymentsLoading &&
            !isResourcesLoading &&
            deployments.length > 0 && (
              <div className="space-y-2">
                <div className="text-[11px] text-[var(--color-text-secondary)]">
                  Latest:{" "}
                  <strong className="text-[var(--color-text-primary)]">
                    {latestDeployment?.version ?? "-"}
                  </strong>
                </div>
                <div className="grid grid-cols-1 gap-2 md:grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)_auto_minmax(0,1fr)_auto_minmax(0,1fr)] md:items-stretch">
                  <ResourceNode
                    title="Ingress"
                    resources={ingressResources}
                    accentClass="bg-[color-mix(in_srgb,_var(--color-success)_12%,_transparent)] text-[var(--color-success)]"
                  />
                  <div className="hidden items-center justify-center text-[var(--color-text-muted)] md:flex">
                    →
                  </div>
                  <ResourceNode
                    title="Service"
                    resources={serviceResources}
                    accentClass="bg-[color-mix(in_srgb,_var(--color-info)_12%,_transparent)] text-[var(--color-info)]"
                  />
                  <div className="hidden items-center justify-center text-[var(--color-text-muted)] md:flex">
                    →
                  </div>
                  <ResourceNode
                    title="Deployment / StatefulSet"
                    resources={workloadResources}
                    accentClass="bg-[color-mix(in_srgb,_var(--color-primary)_12%,_transparent)] text-[var(--color-primary)]"
                  />
                  <div className="hidden items-center justify-center text-[var(--color-text-muted)] md:flex">
                    →
                  </div>
                  <ResourceNode
                    title="Pod"
                    resources={podResources}
                    accentClass="bg-[color-mix(in_srgb,_var(--color-warning)_14%,_transparent)] text-[var(--color-warning)]"
                    emptyLabel={
                      workloadResources.length > 0
                        ? "(managed by workload)"
                        : "-"
                    }
                  />
                </div>
                {jobResources.length > 0 && (
                  <ResourceNode
                    title="Job / CronJob"
                    resources={jobResources}
                    accentClass="bg-[color-mix(in_srgb,_var(--color-info)_16%,_transparent)] text-[var(--color-info)]"
                  />
                )}
              </div>
            )}
        </div>
      </DetailCard>

      {/* env vars */}
      {envEntries.length > 0 && (
        <DetailCard title="Environment Variables">
          <div className="flex flex-col gap-2">
            {envEntries.map(([key, value]) => {
              const isRevealed = revealedVars.has(key);
              return (
                <div
                  key={key}
                  className="grid grid-cols-[1fr_1fr_88px] items-center gap-2 rounded-md border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] px-3 py-2 text-[12px]"
                >
                  <span className="font-mono text-[var(--color-text-primary)]">
                    {key}
                  </span>
                  <span className="truncate font-mono text-[var(--color-text-secondary)]">
                    {isRevealed ? value : "••••••••"}
                  </span>
                  <button
                    type="button"
                    onClick={() => toggleReveal(key)}
                    className="inline-flex items-center justify-center gap-1 rounded border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_4%,_transparent)] px-2 py-[2px] text-[11px] text-[var(--color-text-secondary)]"
                  >
                    {isRevealed ? (
                      <EyeOff {...iconProps("xs")} />
                    ) : (
                      <Eye {...iconProps("xs")} />
                    )}
                    {isRevealed ? "Hide" : "Show"}
                  </button>
                </div>
              );
            })}
          </div>
        </DetailCard>
      )}

      {/* manifest overrides */}
      {manifestEntries.length > 0 && (
        <DetailCard title="Manifest Overrides">
          <div className="flex flex-col gap-2">
            {manifestEntries.map(([key, value]) => {
              const isRevealed = revealedVars.has(key);
              const shortKey = key.replace("NULLUS_MANIFEST_", "");
              return (
                <div
                  key={key}
                  className="rounded-md border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] p-2.5"
                >
                  <div className="mb-1.5 flex items-center justify-between">
                    <span className="text-[11px] font-semibold uppercase tracking-[0.04em] text-[var(--color-text-secondary)]">
                      {shortKey}
                    </span>
                    <button
                      type="button"
                      onClick={() => toggleReveal(key)}
                      className="inline-flex items-center gap-1 rounded border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_4%,_transparent)] px-2 py-[2px] text-[11px] text-[var(--color-text-secondary)]"
                    >
                      {isRevealed ? (
                        <EyeOff {...iconProps("xs")} />
                      ) : (
                        <Eye {...iconProps("xs")} />
                      )}
                      {isRevealed ? "Hide" : "Show"}
                    </button>
                  </div>
                  {isRevealed && (
                    <pre className="max-h-40 overflow-y-auto whitespace-pre-wrap break-all rounded bg-[var(--color-surface-base)] p-2 font-mono text-[11px] text-[var(--color-text-secondary)]">
                      {value}
                    </pre>
                  )}
                </div>
              );
            })}
          </div>
        </DetailCard>
      )}
    </div>
  );
}

function resourceStatusColor(status: string) {
  const s = status.toLowerCase();
  if (s === "running" || s === "active" || s === "healthy")
    return {
      dot: "var(--color-success)",
      bg: "color-mix(in srgb, var(--color-success) 10%, transparent)",
      text: "var(--color-success)",
    };
  if (s === "pending" || s === "progressing" || s === "updating")
    return {
      dot: "var(--color-warning)",
      bg: "color-mix(in srgb, var(--color-warning) 10%, transparent)",
      text: "var(--color-warning)",
    };
  if (s === "failed" || s === "error" || s === "degraded")
    return {
      dot: "var(--color-error)",
      bg: "color-mix(in srgb, var(--color-error) 10%, transparent)",
      text: "var(--color-error)",
    };
  return {
    dot: "var(--color-text-secondary)",
    bg: "color-mix(in srgb, var(--color-text-secondary) 8%, transparent)",
    text: "var(--color-text-secondary)",
  };
}

function kindIcon(kind: string) {
  const k = kind.toLowerCase();
  if (k === "deployment" || k === "statefulset")
    return <Box {...iconProps("xs")} />;
  if (k === "service") return <Activity {...iconProps("xs")} />;
  if (k === "ingress") return <Globe {...iconProps("xs")} />;
  if (k === "pod") return <Server {...iconProps("xs")} />;
  return <Package {...iconProps("xs")} />;
}

function PipelineMonitoringTab({ pipeline }: { pipeline: Pipeline }) {
  const { i18n } = useTranslation();
  const locale = resolveLocale(i18n.resolvedLanguage || i18n.language);
  const { data: deploymentsData, isLoading: isDeploymentsLoading } =
    usePipelineDeployments(pipeline.id);
  const {
    data: resourcesData,
    isLoading: isResourcesLoading,
    refetch: refetchResources,
  } = usePipelineResources(pipeline.id);
  const deployments = deploymentsData?.items ?? [];
  const resources = resourcesData?.items ?? [];

  const total = deployments.length;
  const successCount = deployments.filter((d) => d.status === "success").length;
  const failedCount = deployments.filter((d) => d.status === "failed").length;
  const successRate = total > 0 ? Math.round((successCount / total) * 100) : 0;
  const latestDeployment = [...deployments].sort(
    (a, b) => new Date(b.startedAt).getTime() - new Date(a.startedAt).getTime(),
  )[0];

  const deploymentsWithDuration = deployments.filter(
    (d) => d.startedAt && d.completedAt,
  );
  const avgDurationMs =
    deploymentsWithDuration.reduce((acc, d) => {
      return (
        acc +
        (new Date(d.completedAt as string).getTime() -
          new Date(d.startedAt).getTime())
      );
    }, 0) / Math.max(deploymentsWithDuration.length, 1);
  const avgDuration =
    avgDurationMs > 60000
      ? `${Math.round(avgDurationMs / 60000)}m ${Math.round((avgDurationMs % 60000) / 1000)}s`
      : `${Math.round(avgDurationMs / 1000)}s`;

  const trendMap = new Map<string, { success: number; failed: number }>();
  for (const d of deployments) {
    const date = formatDate(d.startedAt, locale, {
      month: "numeric",
      day: "numeric",
    });
    const entry = trendMap.get(date) ?? { success: 0, failed: 0 };
    if (d.status === "success") entry.success += 1;
    else if (d.status === "failed") entry.failed += 1;
    trendMap.set(date, entry);
  }
  const buildTrend = [...trendMap.entries()]
    .slice(-7)
    .map(([date, counts]) => ({ date, ...counts }));

  const workloadResources = resources.filter((r) =>
    ["deployment", "statefulset", "daemonset"].includes(r.kind.toLowerCase()),
  );
  const serviceResources = resources.filter(
    (r) => r.kind.toLowerCase() === "service",
  );
  const ingressResources = resources.filter(
    (r) => r.kind.toLowerCase() === "ingress",
  );
  const podResources = resources.filter((r) => r.kind.toLowerCase() === "pod");

  const runningPods = podResources.filter(
    (r) => r.status.toLowerCase() === "running",
  ).length;
  const totalPods = podResources.length;

  // 워크로드 이름은 파이프라인 이름에서 나온다(스캐폴딩이 그렇게 만든다).
  // 이 배열로 공용 패널을 이 파이프라인의 앱만 보게 좁힌다.
  const pipelineApps = useMemo(() => [pipeline.name], [pipeline.name]);

  const isLoading = isDeploymentsLoading || isResourcesLoading;

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-10">
        <LoaderCircle
          {...iconProps("md")}
          className="animate-spin text-[var(--color-primary)]"
        />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-5">
      {/* stat cards */}
      <div className="grid grid-cols-2 gap-3 xl:grid-cols-4">
        {[
          {
            label: "Success Rate",
            value: total > 0 ? `${successRate}%` : "-",
            sub: `${successCount}/${total} runs`,
            color: "var(--color-success)",
          },
          {
            label: "Total Runs",
            value: String(total),
            sub: latestDeployment
              ? `Last: ${formatDateTime(latestDeployment.startedAt, locale)}`
              : "No runs yet",
            color: "var(--color-primary)",
          },
          {
            label: "Avg Duration",
            value: total > 0 ? avgDuration : "-",
            sub: `${deploymentsWithDuration.length} measured`,
            color: "var(--color-warning)",
          },
          {
            label: "Failed",
            value: String(failedCount),
            sub:
              total > 0
                ? `${Math.round((failedCount / total) * 100)}% failure rate`
                : "-",
            color:
              failedCount > 0
                ? "var(--color-error)"
                : "var(--color-text-secondary)",
          },
        ].map((item) => (
          <div
            key={item.label}
            className="rounded-xl border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] p-4"
          >
            <div
              className="text-[26px] font-extrabold leading-none"
              style={{ color: item.color }}
            >
              {item.value}
            </div>
            <div className="mt-1 text-[12px] font-semibold text-[var(--color-text-secondary)]">
              {item.label}
            </div>
            <div className="mt-1 text-[11px] text-[var(--color-text-secondary)] opacity-70">
              {item.sub}
            </div>
          </div>
        ))}
      </div>

      {/* 배포된 앱의 실시간 자원 사용과 로그. 모니터링 대시보드와 같은 패널을
          쓰되, 이 파이프라인의 앱으로만 좁힌다 — 옆 앱의 선이 섞이면 어느 것이
          이 앱인지 알 수 없다. */}
      <div className="grid grid-cols-1 gap-3.5 xl:h-[544px] xl:grid-cols-2">
        <div className="grid min-h-0 grid-cols-1 gap-3.5 xl:grid-rows-2">
          <AppUsageCharts
            stackId={pipeline.stackId}
            apps={pipelineApps}
            linkedHint
          />
        </div>
        <AppLogPanel
          stackId={pipeline.stackId}
          apps={pipelineApps}
          linkedHint
        />
      </div>

      {/* live k8s resources */}
      <div className="rounded-xl border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] p-4">
        <div className="mb-3 flex items-center justify-between">
          <span className="text-[13px] font-bold text-[var(--color-text-primary)]">
            Live Resources
          </span>
          <div className="flex items-center gap-3">
            {totalPods > 0 && (
              <span className="text-[12px] text-[var(--color-text-secondary)]">
                <span className="font-semibold text-[var(--color-success)]">
                  {runningPods}
                </span>
                /{totalPods} pods running
              </span>
            )}
            <button
              type="button"
              onClick={() => void refetchResources()}
              aria-label="Refresh resources"
              className="rounded p-1 text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
            >
              <RefreshCw {...iconProps("xs")} />
            </button>
          </div>
        </div>

        {resources.length === 0 ? (
          <div className="py-6 text-center text-[13px] text-[var(--color-text-secondary)]">
            No resources found. Run a deployment first.
          </div>
        ) : (
          <div className="flex flex-col gap-2">
            {[
              { label: "Workloads", items: workloadResources },
              { label: "Services", items: serviceResources },
              { label: "Ingress", items: ingressResources },
              { label: "Pods", items: podResources },
            ]
              .filter((group) => group.items.length > 0)
              .map((group) => (
                <div key={group.label}>
                  <div className="mb-1.5 text-[11px] font-semibold uppercase tracking-[0.05em] text-[var(--color-text-secondary)]">
                    {group.label}
                  </div>
                  <div className="grid grid-cols-1 gap-1.5 sm:grid-cols-2">
                    {group.items.map((r) => {
                      const sc = resourceStatusColor(r.status);
                      return (
                        <div
                          key={`${r.kind}-${r.name}`}
                          className="flex items-center gap-2.5 rounded-lg border border-[var(--color-border-default)] px-3 py-2"
                          style={{ background: sc.bg }}
                        >
                          <span className="shrink-0" style={{ color: sc.text }}>
                            {kindIcon(r.kind)}
                          </span>
                          <div className="min-w-0 flex-1">
                            <div className="truncate text-[12px] font-semibold text-[var(--color-text-primary)]">
                              {r.name}
                            </div>
                            {r.serviceUrls && r.serviceUrls.length > 0 && (
                              <div className="truncate text-[11px] text-[var(--color-text-secondary)]">
                                {r.serviceUrls[0]}
                              </div>
                            )}
                          </div>
                          <span
                            className="shrink-0 rounded-full px-2 py-0.5 text-[10px] font-semibold capitalize"
                            style={{
                              background: sc.bg,
                              color: sc.text,
                              border: `1px solid ${sc.dot}40`,
                            }}
                          >
                            {r.status}
                          </span>
                        </div>
                      );
                    })}
                  </div>
                </div>
              ))}
          </div>
        )}
      </div>

      {/* deployment trend chart */}
      {buildTrend.length > 0 && (
        <div className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-surface-base)] p-4">
          <h4 className="m-0 mb-3 text-[13px] font-bold text-[var(--color-text-primary)]">
            Deployment Trend (last 7 days)
          </h4>
          <ResponsiveContainer width="100%" height={160}>
            <BarChart data={buildTrend} barSize={14}>
              <CartesianGrid
                stroke="color-mix(in srgb, var(--color-text-secondary) 12%, transparent)"
                strokeDasharray="3 3"
                vertical={false}
              />
              <XAxis
                dataKey="date"
                stroke="var(--color-text-muted)"
                tick={{ fill: "var(--color-text-secondary)", fontSize: 11 }}
                axisLine={false}
                tickLine={false}
              />
              <YAxis
                stroke="var(--color-text-muted)"
                tick={{ fill: "var(--color-text-secondary)", fontSize: 11 }}
                axisLine={false}
                tickLine={false}
                allowDecimals={false}
              />
              <Tooltip
                contentStyle={{
                  background: "var(--color-surface-base)",
                  border: "1px solid var(--color-border-default)",
                  borderRadius: 8,
                  color: "var(--color-text-primary)",
                  fontSize: 12,
                }}
                cursor={{
                  fill: "color-mix(in srgb, var(--color-primary) 6%, transparent)",
                }}
              />
              <Bar
                dataKey="success"
                name="Success"
                fill="var(--color-primary)"
                radius={[4, 4, 0, 0]}
              />
              <Bar
                dataKey="failed"
                name="Failed"
                fill="var(--color-error)"
                radius={[4, 4, 0, 0]}
              />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}
    </div>
  );
}

function PipelineHistoryTab({ pipeline }: { pipeline: Pipeline }) {
  const { t, i18n } = useTranslation();
  const locale = resolveLocale(i18n.resolvedLanguage || i18n.language);
  const { data: template } = useTemplateById(pipeline.templateId);
  const { data: deploymentsData, isLoading } = usePipelineDeployments(
    pipeline.id,
  );
  const [selectedDeploymentId, setSelectedDeploymentId] = useState<
    string | null
  >(null);
  const deployments = deploymentsData?.items ?? [];
  const stages = (template?.stages ?? []) as string[];

  useEffect(() => {
    if (deployments.length === 0) {
      setSelectedDeploymentId(null);
      return;
    }
    if (
      !selectedDeploymentId ||
      !deployments.some((d) => d.id === selectedDeploymentId)
    ) {
      setSelectedDeploymentId(deployments[0].id);
    }
  }, [deployments, selectedDeploymentId]);

  const selectedDeployment =
    deployments.find((d) => d.id === selectedDeploymentId) ?? null;
  const { data: deploymentStatus, isLoading: isDeploymentStatusLoading } =
    useDeploymentStatus(selectedDeploymentId);
  const stepDetails = deploymentStatus?.steps ?? [];
  const selectedStageStates = buildStageStates(stages, stepDetails);
  const logLineCount = stepDetails.reduce(
    (total, step) => total + (step.logs?.length ?? 0),
    0,
  );

  if (isLoading) {
    return (
      <div className="py-8 text-center text-sm text-[var(--color-text-secondary)]">
        Loading deployment history...
      </div>
    );
  }

  if (deployments.length === 0) {
    return (
      <div className="py-8 text-center text-sm text-[var(--color-text-secondary)]">
        {t("cicdListPage.emptyDeployments", "No deployment history.")}
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      {deployments.map((d) => {
        const st = getPipelineStatusStyle(d.status);
        const durationMs =
          d.completedAt && d.startedAt
            ? new Date(d.completedAt).getTime() -
              new Date(d.startedAt).getTime()
            : 0;
        const duration =
          durationMs > 0
            ? durationMs >= 60000
              ? `${Math.floor(durationMs / 60000)}m ${Math.round((durationMs % 60000) / 1000)}s`
              : `${Math.round(durationMs / 1000)}s`
            : d.status === "running"
              ? "running"
              : "-";
        const isSelected = d.id === selectedDeploymentId;

        return (
          <div
            key={d.id}
            className={`flex flex-wrap items-center gap-2.5 rounded-lg border px-3.5 py-3 ${
              isSelected
                ? "border-[color-mix(in_srgb,_var(--color-primary)_45%,_transparent)] bg-[color-mix(in_srgb,_var(--color-primary)_12%,_transparent)]"
                : "border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)]"
            }`}
          >
            <span
              className="rounded-full px-2.5 py-0.5 text-[11px] font-bold"
              style={{ background: st.bg, color: st.color }}
            >
              {getPipelineStatusLabel(t, d.status)}
            </span>
            <button
              type="button"
              onClick={() => setSelectedDeploymentId(d.id)}
              className="rounded px-1 py-0.5 text-[13px] font-semibold text-[var(--color-primary)] underline decoration-dotted underline-offset-2 hover:text-[var(--color-primary)]"
            >
              {d.version}
            </button>
            <span className="flex-1 text-[12px] text-[var(--color-text-secondary)]">
              {d.triggeredBy || "-"}
            </span>
            <span className="text-[12px] text-[var(--color-text-secondary)]">
              {duration}
            </span>
            <span className="text-[12px] text-[var(--color-text-secondary)]">
              {formatDateTime(d.startedAt, locale)}
            </span>
          </div>
        );
      })}

      {selectedDeployment && (
        <div className="rounded-lg border border-[color-mix(in_srgb,_var(--color-primary)_35%,_transparent)] bg-[var(--color-surface-sunken)] p-3">
          <div className="flex flex-wrap items-center gap-2 text-[12px] text-[var(--color-text-secondary)]">
            <span className="rounded bg-[color-mix(in_srgb,_var(--color-primary)_20%,_transparent)] px-1.5 py-[2px] font-mono text-[var(--color-primary)]">
              {selectedDeployment.version}
            </span>
            <span>Deployment ID:</span>
            <code className="rounded bg-[color-mix(in_srgb,_var(--color-text-primary)_8%,_transparent)] px-1.5 py-[2px]">
              {selectedDeployment.id}
            </code>
            <span>Triggered by:</span>
            <span className="text-[var(--color-text-primary)]">
              {selectedDeployment.triggeredBy || "-"}
            </span>
          </div>

          {stages.length > 0 && (
            <div className="mt-3 rounded-lg border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] p-2">
              <div className="mb-2 flex flex-wrap items-baseline gap-x-2 gap-y-1">
                <span className="text-[12px] font-semibold text-[var(--color-text-primary)]">
                  Pipeline Stages
                </span>
                {/* 스텝 정보가 없으면 템플릿에 선언된 단계일 뿐이라는 사실을
                    분명히 한다 — 실행 결과로 읽히면 안 된다. */}
                {stepDetails.length === 0 && (
                  <span className="text-[11px] text-[var(--color-text-secondary)]">
                    템플릿 정의 · 이 CI 는 단계별 결과를 보고하지 않습니다
                  </span>
                )}
              </div>
              {stages.map((stage: string, i: number) => {
                const state = selectedStageStates[i] ?? "queued";
                const meta = stageMeta(state);
                return (
                  <div
                    key={`${selectedDeployment.id}-${stage}`}
                    className="relative"
                  >
                    {i < stages.length - 1 && (
                      <div className="absolute left-[15px] top-7 h-[calc(100%-24px)] w-px bg-[color-mix(in_srgb,_var(--color-text-secondary)_25%,_transparent)]" />
                    )}
                    <div
                      className={`mb-1 grid grid-cols-[22px_1fr_auto] items-center gap-2 rounded-md border px-2 py-1.5 ${meta.cls}`}
                    >
                      <span className="flex items-center justify-center">
                        {meta.icon}
                      </span>
                      <span className="truncate text-[12px] font-medium">
                        {stage}
                      </span>
                      {/* 순서는 세로 배치가 이미 말해 준다. 상태만 남긴다. */}
                      <span className="text-[11px] opacity-80">
                        {meta.label}
                      </span>
                    </div>
                  </div>
                );
              })}
            </div>
          )}

          <div className="mt-3 overflow-hidden rounded-lg border border-[var(--color-border-default)] bg-[var(--color-surface-base)]">
            <div className="flex flex-wrap items-center gap-2 border-b border-[color-mix(in_srgb,_var(--color-text-primary)_6%,_transparent)] bg-[color-mix(in_srgb,_var(--color-text-primary)_3%,_transparent)] px-3 py-2 text-[11px] text-[color-mix(in_srgb,_var(--color-text-primary)_65%,_transparent)]">
              <span>Detailed Logs</span>
              <span>·</span>
              <span>{stepDetails.length} steps</span>
              <span>·</span>
              <span>{logLineCount} lines</span>
              {isDeploymentStatusLoading && (
                <span className="text-[var(--color-warning)]">Loading...</span>
              )}
            </div>

            <div className="max-h-[460px] space-y-3 overflow-y-auto p-3 font-mono text-[12px]">
              {!isDeploymentStatusLoading && stepDetails.length === 0 && (
                <div className="text-[12px] text-[var(--color-text-secondary)]">
                  No detailed logs available for this deployment.
                </div>
              )}

              {stepDetails.map((step, stepIndex) => (
                <div
                  key={`${selectedDeployment.id}-${step.name}-${stepIndex}`}
                  className="rounded border border-[color-mix(in_srgb,_var(--color-text-secondary)_25%,_transparent)] bg-[color-mix(in_srgb,_var(--color-surface-base)_65%,_transparent)]"
                >
                  <div className="flex flex-wrap items-center gap-2 border-b border-[color-mix(in_srgb,_var(--color-text-secondary)_25%,_transparent)] px-2.5 py-2 text-[11px] text-[var(--color-text-secondary)]">
                    <span className="font-semibold text-[var(--color-text-secondary)]">
                      {step.name}
                    </span>
                    {step.kind && (
                      <span className="rounded bg-[color-mix(in_srgb,_var(--color-text-secondary)_20%,_transparent)] px-1.5 py-[1px]">
                        {step.kind}
                      </span>
                    )}
                    {step.status && (
                      <span
                        className={`rounded px-1.5 py-[1px] uppercase ${statusClass(step.status)}`}
                      >
                        {step.status}
                      </span>
                    )}
                    {step.applied_at && (
                      <span>{formatDateTime(step.applied_at, locale)}</span>
                    )}
                  </div>
                  <div className="space-y-1 px-2.5 py-2">
                    {(step.logs ?? []).map((line, lineIndex) => (
                      <div
                        key={`${selectedDeployment.id}-${step.name}-${lineIndex}`}
                        className="grid grid-cols-[30px_minmax(0,1fr)] gap-2"
                      >
                        <span className="text-right text-[10px] text-[var(--color-text-muted)]">
                          {lineIndex + 1}
                        </span>
                        <span className={`break-all ${logLineClass(line)}`}>
                          {line}
                        </span>
                      </div>
                    ))}
                    {(step.logs ?? []).length === 0 && (
                      <div className="text-[11px] text-[var(--color-text-secondary)]">
                        {step.message || "No log lines for this step."}
                      </div>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function PipelineDetailPanel({
  pipeline,
  onExecuteClick,
  onOpenLogs,
  onDelete,
  isDeleting,
}: {
  pipeline: Pipeline;
  onExecuteClick: () => void;
  onOpenLogs: () => void;
  onDelete: () => void;
  isDeleting: boolean;
}) {
  const { t, i18n } = useTranslation();
  const locale = resolveLocale(i18n.resolvedLanguage || i18n.language);
  const [innerTab, setInnerTab] = useState<PipelineInnerTab>("info");
  const statusStyle = getPipelineStatusStyle(pipeline.status);

  return (
    <div className="overflow-hidden rounded-[var(--card-radius)] border border-[color-mix(in_srgb,_var(--color-primary)_30%,_transparent)] bg-[var(--color-surface-card)]">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--color-border-default)] px-5 py-3.5">
        <div className="flex items-center gap-3">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-[color-mix(in_srgb,_var(--color-primary)_15%,_transparent)] text-[var(--color-primary)]">
            <GitBranch {...iconProps("sm")} />
          </div>
          <h3 className="m-0 text-[15px] font-bold text-[var(--color-text-primary)]">
            {pipeline.name}
          </h3>
          <Badge
            className=""
            style={{ background: statusStyle.bg, color: statusStyle.color }}
          >
            {getPipelineStatusLabel(t, pipeline.status)}
          </Badge>
          <span className="text-[12px] text-[var(--color-text-secondary)]">
            · {pipeline.appType} · {pipeline.clusterName} ·{" "}
            {formatDateTime(pipeline.lastDeployedAt, locale)}
          </span>
        </div>

        <div className="flex items-center gap-1.5">
          <Button
            variant="secondary"
            size="sm"
            type="button"
            className="border border-[color-mix(in_srgb,_var(--color-error)_35%,_transparent)] bg-[color-mix(in_srgb,_var(--color-error)_15%,_transparent)] text-[var(--color-error)] hover:bg-[color-mix(in_srgb,_var(--color-error)_25%,_transparent)]"
            onClick={onDelete}
            disabled={isDeleting}
          >
            <Trash2 {...iconProps("xs")} />
            {isDeleting ? "Deleting..." : "Delete"}
          </Button>
          <Button
            variant="secondary"
            size="sm"
            type="button"
            onClick={onOpenLogs}
          >
            <Terminal {...iconProps("xs")} />
            Logs
          </Button>
          <Button
            variant="primary"
            size="sm"
            type="button"
            onClick={onExecuteClick}
          >
            <Rocket {...iconProps("xs")} />
            Execute
          </Button>
        </div>
      </div>

      <Tabs
        value={innerTab}
        onChange={setInnerTab}
        items={INNER_TABS.map((tab) => ({
          id: tab.key,
          icon: tab.icon,
          label: tab.label,
        }))}
      />

      <div className="p-5">
        {innerTab === "info" && <PipelineInfoTab pipeline={pipeline} />}
        {innerTab === "monitoring" && (
          <PipelineMonitoringTab pipeline={pipeline} />
        )}
        {innerTab === "history" && <PipelineHistoryTab pipeline={pipeline} />}
      </div>
    </div>
  );
}

// deletePipelineErrorMessage 는 삭제 실패를 사용자가 읽을 수 있는 한 줄로 만든다.
function deletePipelineErrorMessage(error: unknown): string {
  if (typeof error === "object" && error !== null && "message" in error) {
    const message = (error as { message?: unknown }).message;
    if (typeof message === "string" && message.trim() !== "") return message;
  }
  return "파이프라인을 지우지 못했습니다";
}

export function CicdListPage() {
  const { t, i18n } = useTranslation();
  const locale = resolveLocale(i18n.resolvedLanguage || i18n.language);
  const navigate = useNavigate();
  const [statusFilter, setStatusFilter] = useState("");
  const [clusterFilter, setClusterFilter] = useState("");
  const [search, setSearch] = useState("");
  const [expandedPipelineId, setExpandedPipelineId] = useState<string | null>(
    null,
  );
  const [pipelinePendingDelete, setPipelinePendingDelete] =
    useState<Pipeline | null>(null);
  const [deletingPipelineId, setDeletingPipelineId] = useState<string | null>(
    null,
  );
  // 삭제가 끝난 뒤 알려야 할 것들 — 지우지 못한 자원, 또는 삭제 실패 사유.
  const [deleteWarnings, setDeleteWarnings] = useState<string[]>([]);
  const [deployingPipelineId, setDeployingPipelineId] = useState<string | null>(
    null,
  );
  const [executeModalPipeline, setExecuteModalPipeline] =
    useState<Pipeline | null>(null);
  const [viewportWidth, setViewportWidth] = useState(() =>
    typeof window !== "undefined" ? window.innerWidth : 1440,
  );
  const isDesktopLayout = viewportWidth >= 1280;

  useEffect(() => {
    if (typeof window === "undefined") return;
    const onResize = () => setViewportWidth(window.innerWidth);
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);

  const { data: clustersData } = useClusters();
  const { data: stacksData } = useStacks();
  const { data: apiData } = usePipelines({
    status: statusFilter || undefined,
    search: search || undefined,
  });
  const deletePipelineMutation = useDeletePipeline();
  const deployPipelineMutation = useDeployPipeline();
  const pipelines = apiData?.items ?? [];

  const filtered = pipelines.filter((p) => {
    const q = search.toLowerCase();
    const matchesSearch =
      !search ||
      p.name.toLowerCase().includes(q) ||
      p.clusterName.toLowerCase().includes(q);
    const matchesStatus = !statusFilter || p.status === statusFilter;
    const matchesCluster = !clusterFilter || p.clusterId === clusterFilter;
    return matchesSearch && matchesStatus && matchesCluster;
  });

  const selectedPipelineId =
    expandedPipelineId &&
    filtered.some((pipeline) => pipeline.id === expandedPipelineId)
      ? expandedPipelineId
      : (filtered[0]?.id ?? null);
  const expandedPipeline = selectedPipelineId
    ? (filtered.find((pipeline) => pipeline.id === selectedPipelineId) ?? null)
    : null;

  // 파이프라인이 어느 스택 위에서 도는지 보여준다. 스택마다 레지스트리와
  // 저장소가 달라 이미지가 어디로 가는지가 스택에 따라 달라진다.
  const stackNameById = new Map(
    (stacksData?.items ?? []).map((stack) => [stack.id, stack.name]),
  );

  const columns: ColumnDef<Pipeline, unknown>[] = [
    {
      accessorKey: "name",
      header: t("cicdListPage.table.name", "Name"),
      cell: ({ row }) => (
        <div className="flex items-center gap-2">
          {selectedPipelineId === row.original.id && (
            <div className="h-1.5 w-1.5 shrink-0 rounded-full bg-[var(--color-primary)]" />
          )}
          <span className="font-semibold">{row.original.name}</span>
        </div>
      ),
    },
    {
      accessorKey: "mode",
      header: "Mode",
      cell: ({ row }) => (
        <span className="rounded-md border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-primary)_8%,_transparent)] px-[8px] py-[2px] text-[11px] font-semibold text-[var(--color-primary)]">
          {modeLabel(row.original.mode)}
        </span>
      ),
    },
    {
      accessorKey: "appType",
      header: t("cicdListPage.table.appType", "App Type"),
      cell: ({ row }) => (
        <span className="text-[var(--color-text-secondary)]">
          {row.original.appType}
        </span>
      ),
    },
    {
      id: "accessUrl",
      header: t("cicdListPage.table.accessUrl", "Access"),
      cell: ({ row }) => {
        const url = row.original.accessUrl?.trim();
        // 스택에 접근 도메인이 없으면 앱은 클러스터 안에서만 닿는다. 열리지
        // 않는 링크를 보여 주는 것이 없다고 밝히는 것보다 나쁘다.
        if (!url) {
          return <span className="text-[var(--color-text-muted)]">—</span>;
        }
        return (
          <a
            href={url}
            target="_blank"
            rel="noreferrer"
            title={url}
            // 행 클릭은 상세 패널을 여는 동작이다. 링크까지 그것을 타면
            // 새 탭이 열리면서 패널도 바뀐다.
            onClick={(event) => event.stopPropagation()}
            className="block max-w-[180px] truncate underline"
          >
            {url.replace(/^https:\/\//, "")}
          </a>
        );
      },
    },
    {
      accessorKey: "clusterName",
      header: t("cicdListPage.table.cluster", "Cluster"),
      cell: ({ row }) => (
        <span className="text-[var(--color-text-secondary)]">
          {row.original.clusterName}
        </span>
      ),
    },
    {
      id: "stack",
      header: t("cicdListPage.table.stack", "Stack"),
      cell: ({ row }) => {
        const stackId = row.original.stackId;
        // 스택 없이 만든 파이프라인도 있다. 이름을 지어내지 않고 없음을 밝힌다.
        if (!stackId) {
          return (
            <span className="text-[var(--color-text-muted)]">
              {t("cicdListPage.table.noStack", "—")}
            </span>
          );
        }
        // 스택이 지워져도 파이프라인 행은 남는다. 그때 id 를 이름 자리에 넣으면
        // stk_c073c556ed8c 가 스택 "이름" 으로 읽혀, 가리키는 스택이 없다는
        // 사실을 놓친다. id 는 툴팁으로만 남긴다.
        const name = stackNameById.get(stackId);
        if (!name) {
          return (
            <span className="text-[var(--color-text-muted)]" title={stackId}>
              {t("cicdListPage.table.deletedStack", "삭제됨")}
            </span>
          );
        }
        return (
          <span className="text-[var(--color-text-secondary)]">{name}</span>
        );
      },
    },
    {
      accessorKey: "status",
      header: t("cicdListPage.table.status", "Status"),
      cell: ({ row }) => {
        const st = getPipelineStatusStyle(row.original.status);
        return (
          <span
            className="rounded-md px-[9px] py-[3px] text-xs font-semibold"
            style={{ backgroundColor: st.bg, color: st.color }}
          >
            {getPipelineStatusLabel(t, row.original.status)}
          </span>
        );
      },
    },
    {
      accessorKey: "lastDeployedAt",
      header: t("cicdListPage.table.lastDeployed", "Last Deployed"),
      cell: ({ row }) => (
        <span className="text-[13px] text-[var(--color-text-secondary)]">
          {formatDateTime(row.original.lastDeployedAt, locale)}
        </span>
      ),
    },
  ];

  // 삭제는 확인 대화상자를 거친다. 저장소·이미지 삭제는 되돌릴 수 없고
  // 클러스터 리소스를 지우면 돌던 앱이 멈추므로, 무엇을 지울지 사용자가 고른다.
  const handleDeletePipeline = (pipeline: Pipeline) => {
    setPipelinePendingDelete(pipeline);
  };

  const confirmDeletePipeline = async (selection: DeletePipelineSelection) => {
    const pipeline = pipelinePendingDelete;
    if (!pipeline) return;

    try {
      setDeletingPipelineId(pipeline.id);
      setDeleteWarnings([]);
      const result = await deletePipelineMutation.mutateAsync({
        id: pipeline.id,
        ...selection,
      });
      // 지우지 못한 것이 있으면 말해 준다. 레지스트리가 이미지 삭제를 지원하지
      // 않는 경우가 그렇다 — 조용히 넘기면 레지스트리에 남은 것을 영영 모른다.
      setDeleteWarnings(result?.warnings ?? []);
      setPipelinePendingDelete(null);
      if (selectedPipelineId === pipeline.id) {
        setExpandedPipelineId(null);
      }
    } catch (error) {
      // 예전에는 이 거부를 아무도 받지 않아 콘솔에만 남았다. 사용자 화면은
      // 아무 일도 없었던 것처럼 보였다.
      setDeleteWarnings([deletePipelineErrorMessage(error)]);
    } finally {
      setDeletingPipelineId(null);
    }
  };

  const handleDeployPipeline = async (pipeline: Pipeline) => {
    try {
      setDeployingPipelineId(pipeline.id);
      const result = await deployPipelineMutation.mutateAsync({
        pipelineId: pipeline.id,
      });
      setExecuteModalPipeline(null);
      navigate(
        `/cicd/pipelines/${pipeline.id}/logs?deploymentId=${result.deploymentId}`,
      );
    } finally {
      setDeployingPipelineId(null);
    }
  };

  return (
    <div>
      <PageHeader
        breadcrumb={[{ label: t("sidebar.cicdList", "CI/CD List") }]}
        icon={<Workflow {...iconProps("sm")} />}
        tone="primary"
        title={t("cicdListPage.title", "CI/CD List")}
        subtitle={t("cicdListPage.description", "CI/CD Pipeline List")}
        actions={
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="md"
              onClick={() => navigate("/cicd/developer-deploy")}
              type="button"
            >
              <Plus {...iconProps("sm")} />
              {t("cicd.addPhase", "Add Phase")}
            </Button>
            <Button
              variant="primary"
              size="md"
              onClick={() => navigate("/cicd/templates")}
              type="button"
            >
              <Plus {...iconProps("sm")} />
              {t("cicd.newPipeline", "New Pipeline")}
            </Button>
          </div>
        }
      />

      {deleteWarnings.length > 0 && (
        <div
          className="mb-4 rounded-md border border-[var(--color-warning)] p-3 text-sm text-[var(--color-text-secondary)]"
          data-testid="delete-warnings"
        >
          <div className="mb-2 flex items-start justify-between gap-3">
            <p className="m-0 font-medium text-[var(--color-warning)]">
              {t(
                "cicdListPage.deleteWarnings.title",
                "Deleted, but some resources remain",
              )}
            </p>
            <button
              type="button"
              className="text-xs underline"
              onClick={() => setDeleteWarnings([])}
            >
              {t("common.dismiss", "Dismiss")}
            </button>
          </div>
          <ul className="m-0 list-disc pl-4">
            {deleteWarnings.map((warning) => (
              <li key={warning}>{warning}</li>
            ))}
          </ul>
        </div>
      )}

      <div className="grid gap-4 xl:grid-cols-[minmax(300px,38%)_minmax(0,62%)]">
        <div className="min-w-0" data-tour="pipeline-list">
          <DataTable
            columns={columns}
            data={filtered}
            getRowKey={(row) => row.id}
            onRowClick={(row) => setExpandedPipelineId(row.id)}
            emptyMessage={t(
              "cicdListPage.emptyPipelines",
              "No pipelines found.",
            )}
            toolbar={
              <>
                <Select
                  value={statusFilter}
                  onChange={(e) => setStatusFilter(e.target.value)}
                >
                  <option value="">
                    {t("cicdListPage.filters.allStatus", "All Status")}
                  </option>
                  <option value="success">
                    {t("cicd.status.success", "Success")}
                  </option>
                  <option value="running">
                    {t("cicd.status.running", "Running")}
                  </option>
                  <option value="pending">
                    {t("cicd.status.pending", "Pending")}
                  </option>
                  <option value="failed">
                    {t("cicd.status.failed", "Failed")}
                  </option>
                  <option value="cancelled">
                    {t("cicd.status.cancelled", "Cancelled")}
                  </option>
                </Select>
                <Select
                  value={clusterFilter}
                  onChange={(e) => setClusterFilter(e.target.value)}
                  className="w-auto"
                >
                  <option value="">
                    {t("cicdListPage.filters.allClusters", "All Clusters")}
                  </option>
                  {(clustersData?.items ?? []).map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.name}
                    </option>
                  ))}
                </Select>
                <SearchInput
                  wrapperClassName="ml-auto w-[220px]"
                  placeholder={t(
                    "cicdListPage.searchPlaceholder",
                    "Search pipelines...",
                  )}
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
              </>
            }
          />
          <div className="mt-2 hidden text-[12px] text-[var(--color-text-secondary)] xl:block">
            {t(
              "cicdListPage.listHint",
              "Selecting a pipeline from the list updates the detail panel immediately.",
            )}
          </div>
        </div>

        {isDesktopLayout && (
          <div>
            {expandedPipeline ? (
              <div className="h-full pr-1">
                <PipelineDetailPanel
                  key={expandedPipeline.id}
                  pipeline={expandedPipeline}
                  onDelete={() => void handleDeletePipeline(expandedPipeline)}
                  isDeleting={deletingPipelineId === expandedPipeline.id}
                  onExecuteClick={() =>
                    setExecuteModalPipeline(expandedPipeline)
                  }
                  onOpenLogs={() =>
                    navigate(`/cicd/pipelines/${expandedPipeline.id}/logs`)
                  }
                />
              </div>
            ) : (
              <div className="rounded-[var(--card-radius)] border border-dashed border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] p-8 text-center text-[13px] text-[var(--color-text-secondary)]">
                {t(
                  "cicdListPage.emptyDetail",
                  "Select a pipeline from the list to view details here.",
                )}
              </div>
            )}
          </div>
        )}
      </div>

      {!isDesktopLayout && expandedPipeline && (
        // 세로로 이어 붙는 배치라 목록과 간격이 필요하다. 2단 배치에서는
        // 같은 여백이 상세 카드를 목록보다 낮게 그려 상단이 어긋난다.
        <div className="mt-2.5">
          <PipelineDetailPanel
            key={`${expandedPipeline.id}-mobile`}
            pipeline={expandedPipeline}
            onDelete={() => void handleDeletePipeline(expandedPipeline)}
            isDeleting={deletingPipelineId === expandedPipeline.id}
            onExecuteClick={() => setExecuteModalPipeline(expandedPipeline)}
            onOpenLogs={() =>
              navigate(`/cicd/pipelines/${expandedPipeline.id}/logs`)
            }
          />
        </div>
      )}

      {executeModalPipeline && (
        <ExecuteModal
          pipeline={executeModalPipeline}
          onClose={() => setExecuteModalPipeline(null)}
          onExecute={() => void handleDeployPipeline(executeModalPipeline)}
          isExecuting={deployingPipelineId === executeModalPipeline.id}
        />
      )}

      <DeletePipelineDialog
        open={pipelinePendingDelete !== null}
        pipelineName={pipelinePendingDelete?.name ?? ""}
        repositoryPath={repositoryPathOf(pipelinePendingDelete)}
        imageRepository={
          pipelinePendingDelete?.envVars?.IMAGE_REGISTRY_URL || undefined
        }
        busy={deletingPipelineId === pipelinePendingDelete?.id}
        onCancel={() => setPipelinePendingDelete(null)}
        onConfirm={(selection) => void confirmDeletePipeline(selection)}
      />
    </div>
  );
}

/**
 * git_repo_url 에서 owner/repo 를 뽑는다.
 *
 * 대화상자가 확인용으로 보여 주고 사용자가 그대로 입력해야 하는 값이므로,
 * 스킴과 .git 접미사를 걷어 백엔드가 쓰는 형태와 맞춘다.
 */
function repositoryPathOf(pipeline: Pipeline | null): string | undefined {
  const url = pipeline?.gitRepoUrl?.trim();
  if (!url) return undefined;
  const withoutScheme = url.replace(/^[a-z]+:\/\//i, "").replace(/\.git$/i, "");
  const segments = withoutScheme.split("/").filter(Boolean);
  // host/owner/repo 이하로 잘리면 확인 문자열을 만들 수 없다.
  if (segments.length < 3) return undefined;
  return segments.slice(-2).join("/");
}
