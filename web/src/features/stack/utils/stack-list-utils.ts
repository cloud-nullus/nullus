import type { TFunction } from "i18next";

export type PipelineNode = {
  category: string;
  oss: string;
  version: string;
  instances: number;
  color: string;
  health: "healthy" | "progressing" | "degraded";
  sync: "synced" | "out-of-sync";
};

export type ToolSelectionView = {
  name: string;
  version: string;
  instances: number;
};

export type MonitoringToolView = {
  key: string;
  name: string;
  version: string;
  enabled: boolean;
  pod_count: number;
};

export type LaunchTool = {
  name: string;
  version: string;
  url: string | null;
  logo: string;
};

import type {
  StackConnectionInfoResponse,
  StorageConnectionResponse,
  ToolCredentialResponse,
} from "../api/stack-api-types";

export type StorageConnectionInfo = {
  mode: string;
  providerOrEngine: string;
  endpoint: string;
  resourceName: string;
  authId: string;
  accessSecretRef: string;
  authPasswordKey: string;
};

export type StackConnectionInfo = {
  accessDomain: string;
  // 스택이 설치된 네임스페이스. 인증정보 조회 명령을 만들 때 필요하다.
  namespace: string;
  database: StorageConnectionInfo;
  objectStorage: StorageConnectionInfo;
};

function tryGetHostname(url: string | null): string | null {
  if (!url) return null;
  try {
    return new URL(url).hostname;
  } catch {
    return null;
  }
}

export function buildHostsText(
  stackName: string,
  accessDomain: string,
  launchTools: LaunchTool[],
): string {
  if (!accessDomain) {
    return "";
  }
  const hostSet = new Set<string>();
  for (const tool of launchTools) {
    const hostname = tryGetHostname(tool.url);
    if (hostname) {
      hostSet.add(hostname);
    }
  }
  hostSet.add(accessDomain);

  const hosts = Array.from(hostSet).sort();
  if (hosts.length === 0) {
    return "";
  }

  return [`# Nullus Stack: ${stackName}`, `127.0.0.1 ${hosts.join(" ")}`].join(
    "\n",
  );
}

export async function copyTextToClipboard(value: string): Promise<void> {
  if (typeof navigator !== "undefined" && navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(value);
    return;
  }

  if (typeof document !== "undefined") {
    const textArea = document.createElement("textarea");
    textArea.value = value;
    textArea.style.position = "fixed";
    textArea.style.left = "-9999px";
    document.body.appendChild(textArea);
    textArea.focus();
    textArea.select();
    document.execCommand("copy");
    document.body.removeChild(textArea);
  }
}

export function getStackStatusLabel(t: TFunction, status: string) {
  switch (status) {
    case "pending":
      return t("stackList.status.pending", "Pending");
    case "terminating":
      return t("stackList.status.terminating", "Terminating");
    case "validating":
      return t("stackList.status.validating", "Validating");
    case "installing":
      return t("stackList.status.installing", "Installing");
    case "configuring":
      return t("stackList.status.configuring", "Configuring");
    case "health_check":
      return t("stackList.status.healthCheck", "Health Check");
    case "completed":
      return t("stackList.status.completed", "Completed");
    case "failed":
      return t("stackList.status.failed", "Failed");
    case "rolling_back":
      return t("stackList.status.rollingBack", "Rolling Back");
    case "rolled_back":
      return t("stackList.status.rolledBack", "Rolled Back");
    case "running":
      return t("stackList.status.running", "Running");
    case "success":
    case "healthy":
      return t("stackList.status.healthy", "Running");
    case "cancelled":
      return t("stackList.status.cancelled", "Cancelled");
    case "deleted":
      return t("stackList.status.deleted", "Deleted");
    default:
      return status;
  }
}

export function normalizeStackStatus(
  status: string,
  clusterConnectionStatus?: string,
): string {
  if (status === "success" || status === "running") return "healthy";
  if (status === "completed" && clusterConnectionStatus === "connected")
    return "healthy";
  return status;
}

export function isHealthyStatus(
  status: string,
  clusterConnectionStatus?: string,
): boolean {
  const normalized = normalizeStackStatus(status, clusterConnectionStatus);
  return normalized === "healthy";
}

export function matchesStackStatusFilter(
  status: string,
  filter: string,
  clusterConnectionStatus?: string,
): boolean {
  if (!filter) return true;
  const normalized = normalizeStackStatus(status, clusterConnectionStatus);
  if (filter === "healthy") return normalized === "healthy";
  if (filter === "running") return status === "running";
  if (filter === "completed")
    return status === "completed" && clusterConnectionStatus !== "connected";
  return normalized === filter;
}

export function formatDate(iso: string) {
  if (!iso) {
    return "-";
  }
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  return date.toLocaleDateString("ko-KR", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  });
}

export function toShellSingleQuoted(value: string): string {
  return `'${value.replace(/'/g, `'"'"'`)}'`;
}

function toRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }
  return value as Record<string, unknown>;
}

function readString(record: Record<string, unknown>, keys: string[]): string {
  for (const key of keys) {
    const value = record[key];
    if (typeof value === "string" && value.trim() !== "") {
      return value;
    }
  }
  return "";
}

function readNumber(
  record: Record<string, unknown>,
  keys: string[],
): number | null {
  for (const key of keys) {
    const value = record[key];
    if (typeof value === "number" && Number.isFinite(value)) {
      return value;
    }
  }
  return null;
}

function readBool(
  record: Record<string, unknown>,
  keys: string[],
  defaultValue = true,
): boolean {
  for (const key of keys) {
    const value = record[key];
    if (typeof value === "boolean") {
      return value;
    }
  }
  return defaultValue;
}

function parseToolSelection(raw: unknown): ToolSelectionView | null {
  const record = toRecord(raw);
  if (!record) {
    return null;
  }

  if (!readBool(record, ["enabled", "Enabled"], true)) {
    return null;
  }

  const name = readString(record, ["tool", "name", "Name", "id"]);
  if (!name) {
    return null;
  }
  const version =
    readString(record, ["version", "Version", "app_version", "appVersion"]) ||
    "-";
  const instances = Math.max(
    1,
    Math.floor(readNumber(record, ["instances", "replicas", "count"]) ?? 1),
  );

  return { name, version, instances };
}

function resolveSnapshotConfig(snapshot: unknown): Record<string, unknown> {
  const root = toRecord(snapshot);
  if (!root) {
    return {};
  }
  const nested = toRecord(root.config) ?? toRecord(root.Config);
  return nested ?? root;
}

function pickGroup(
  config: Record<string, unknown>,
  keys: string[],
): Record<string, unknown> {
  for (const key of keys) {
    const group = toRecord(config[key]);
    if (group) {
      return group;
    }
  }
  return {};
}

function parseCategorySelections(
  group: Record<string, unknown>,
  keyPairs: string[][],
): ToolSelectionView[] {
  const tools: ToolSelectionView[] = [];
  for (const pair of keyPairs) {
    let selection: ToolSelectionView | null = null;
    for (const key of pair) {
      selection = parseToolSelection(group[key]);
      if (selection) break;
    }
    if (selection) {
      tools.push(selection);
    }
  }
  return tools;
}

// 하나의 제품이 여러 역할을 겸할 수 있다 — Nexus 는 컨테이너 레지스트리이면서
// 패키지 저장소다. 역할 수만큼 세면 "Nexus + GitLab CE + Nexus + MinIO" 처럼 보이고
// 인스턴스도 이중 계상되므로, 같은 이름은 한 번만 남긴다.
function dedupeByName<T extends { name: string }>(tools: T[]): T[] {
  const seen = new Set<string>();
  return tools.filter((tool) => {
    const key = tool.name.trim().toLowerCase();
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

function toPipelineNode(
  category: string,
  tools: ToolSelectionView[],
  color: string,
): PipelineNode | null {
  const distinct = dedupeByName(tools);
  if (distinct.length === 0) {
    return null;
  }
  return {
    category,
    oss: distinct.map((tool) => tool.name).join(" + "),
    version: distinct.map((tool) => tool.version).join(" / "),
    instances: distinct.reduce((sum, tool) => sum + tool.instances, 0),
    color,
    health: "healthy",
    sync: "synced",
  };
}

export function buildPipelineNodesFromSnapshot(
  snapshot: unknown,
): PipelineNode[] {
  const config = resolveSnapshotConfig(snapshot);
  const artifacts = parseCategorySelections(
    pickGroup(config, ["artifacts", "Artifacts"]),
    [
      ["package_registry", "packageRegistry"],
      ["source_repository", "sourceRepository"],
      ["container_registry", "containerRegistry"],
      ["storage_backend", "storageBackend"],
    ],
  );
  const pipeline = pickGroup(config, ["pipeline", "Pipeline"]);
  const ci = parseCategorySelections(pipeline, [["ci_platform", "ciPlatform"]]);
  const cd = parseCategorySelections(pipeline, [["cd_tool", "cdTool"]]);
  const monitoring = parseCategorySelections(
    pickGroup(config, ["monitoring", "Monitoring"]),
    [
      ["collection", "Collection"],
      ["visualization", "Visualization"],
    ],
  );
  const loggingGroup = pickGroup(config, ["logging", "Logging"]);
  const logging = parseCategorySelections(loggingGroup, [
    ["collection", "Collection"],
    ["search", "Search"],
  ]);
  const trace = parseCategorySelections(loggingGroup, [
    ["trace_layer", "traceLayer", "TraceLayer"],
  ]);

  return [
    toPipelineNode("Artifacts", artifacts, "var(--color-primary)"),
    toPipelineNode("CI", ci, "var(--color-info)"),
    toPipelineNode("CD", cd, "var(--color-accent-alt)"),
    toPipelineNode("Monitoring", monitoring, "var(--color-success)"),
    toPipelineNode("Logging", logging, "var(--color-warning)"),
    toPipelineNode("Trace", trace, "var(--color-error)"),
  ].filter((node): node is PipelineNode => !!node);
}

export function buildPipelineNodesFromMonitoring(
  tools: MonitoringToolView[] | undefined,
): PipelineNode[] {
  const enabledTools = (tools ?? []).filter((tool) => tool.enabled);
  const toNode = (
    category: string,
    keys: string[],
    color: string,
  ): PipelineNode | null => {
    const matches = dedupeByName(
      enabledTools.filter((tool) => keys.includes(tool.key)),
    );
    if (matches.length === 0) {
      return null;
    }
    return {
      category,
      oss: matches.map((tool) => tool.name).join(" + "),
      version: matches.map((tool) => tool.version).join(" / "),
      instances: matches.reduce((sum, tool) => sum + tool.pod_count, 0),
      color,
      health: "healthy",
      sync: "synced",
    };
  };

  return [
    toNode(
      "Artifacts",
      [
        "source_repository",
        "container_registry",
        "package_registry",
        "storage_backend",
      ],
      "var(--color-primary)",
    ),
    toNode("CD", ["cd_tool"], "var(--color-accent-alt)"),
    toNode("Monitoring", ["collection", "visualization"], "var(--color-success)"),
    toNode("Logging", ["logging_collection", "logging_search"], "var(--color-warning)"),
    toNode("Trace", ["trace_layer"], "var(--color-error)"),
  ].filter((node): node is PipelineNode => !!node);
}

export function buildInstalledToolsFromSnapshot(
  snapshot: unknown,
): ToolSelectionView[] {
  const config = resolveSnapshotConfig(snapshot);
  const artifacts = parseCategorySelections(
    pickGroup(config, ["artifacts", "Artifacts"]),
    [
      ["package_registry", "packageRegistry"],
      ["source_repository", "sourceRepository"],
      ["container_registry", "containerRegistry"],
      ["storage_backend", "storageBackend"],
    ],
  );
  const pipeline = parseCategorySelections(
    pickGroup(config, ["pipeline", "Pipeline"]),
    [
      ["ci_platform", "ciPlatform"],
      ["cd_tool", "cdTool"],
    ],
  );
  const monitoring = parseCategorySelections(
    pickGroup(config, ["monitoring", "Monitoring"]),
    [
      ["collection", "Collection"],
      ["visualization", "Visualization"],
    ],
  );
  const logging = parseCategorySelections(
    pickGroup(config, ["logging", "Logging"]),
    [
      ["collection", "Collection"],
      ["search", "Search"],
      ["trace_layer", "traceLayer", "TraceLayer"],
    ],
  );
  const authenticationGroup = pickGroup(config, [
    "authentication",
    "Authentication",
  ]);
  const authProvider = readString(authenticationGroup, [
    "provider",
    "Provider",
    "name",
    "Name",
    "tool",
  ]);
  const authentication: ToolSelectionView[] = authProvider
    ? [{ name: authProvider, version: "shared", instances: 1 }]
    : [];

  const byName = new Map<string, ToolSelectionView>();
  for (const tool of [
    ...authentication,
    ...artifacts,
    ...pipeline,
    ...monitoring,
    ...logging,
  ]) {
    const key = tool.name.toLowerCase();
    if (!byName.has(key)) {
      byName.set(key, tool);
    }
  }

  return Array.from(byName.values());
}

function sanitizeAccessDomain(value: string): string {
  const trimmed = value.trim().toLowerCase();
  if (!trimmed) return "";
  const noScheme = trimmed.replace(/^https?:\/\//, "");
  const hostOnly = noScheme.split("/")[0]?.split(":")[0] ?? "";
  const noWildcard = hostOnly.replace(/^\*\./, "");
  return noWildcard;
}

function fallbackAccessDomain(stackName: string): string {
  const slug = stackName
    .toLowerCase()
    .replace(/[^a-z0-9-\s]/g, "")
    .trim()
    .replace(/\s+/g, "-")
    .replace(/-+/g, "-");
  if (!slug) {
    return "";
  }
  return `${slug}.internal`;
}

export function deriveGatewayName(
  accessDomain: string,
  stackName: string,
): string {
  const base = (accessDomain || fallbackAccessDomain(stackName))
    .replace(/\.internal$/i, "")
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, "-")
    .replace(/-+/g, "-")
    .replace(/^-+|-+$/g, "");
  return `${base || "nullus-stack"}-gateway`;
}

export function extractAccessDomain(
  snapshot: unknown,
  stackName: string,
): string {
  const config = resolveSnapshotConfig(snapshot);
  const value = config.access_domain ?? config.accessDomain;
  if (typeof value === "string") {
    const normalized = sanitizeAccessDomain(value);
    if (normalized) {
      return normalized;
    }
  }
  return fallbackAccessDomain(stackName);
}


/**
 * 서버 응답을 화면 표시용 모양으로 옮긴다.
 *
 * 값을 만들지 않고 옮기기만 한다 — 리소스 이름의 단일 출처는 서버
 * (internal/stack/domain/connection.go)다. 아직 응답이 없으면 빈 값을 주어
 * "-" 로 보이게 하고, 그럴듯한 이름을 지어내지 않는다.
 */
export function toConnectionInfoView(
  server: StackConnectionInfoResponse | undefined,
  fallbackAccessDomain = "",
): StackConnectionInfo {
  const storage = (s?: StorageConnectionResponse): StorageConnectionInfo => ({
    mode: s?.mode ?? "",
    providerOrEngine: s?.engine ?? "",
    endpoint: s?.endpoint ?? "",
    resourceName: s?.resource_name ?? "",
    authId: s?.auth_id ?? "",
    accessSecretRef: s?.secret_ref ?? "",
    authPasswordKey: s?.secret_key ?? "",
  });

  return {
    accessDomain: server?.access_domain || fallbackAccessDomain,
    namespace: server?.namespace ?? "",
    database: storage(server?.database),
    objectStorage: storage(server?.object_storage),
  };
}

/**
 * 실행 도구 이름으로 서버가 준 자격증명을 찾는다.
 *
 * 카탈로그 표기가 "Argo CD" / "argocd" 로 흔들리므로 정규화해서 맞춘다.
 */
export function findToolCredential(
  tools: ToolCredentialResponse[] | undefined,
  name: string,
): ToolCredentialResponse | undefined {
  const key = normalizeToolName(name);
  return tools?.find((tool) => normalizeToolName(tool.name) === key);
}

// 표기 흔들림을 흡수한다 — "Argo CD" / "argo-cd" / "ArgoCD" 는 같은 도구다.
// 구분자를 남겨 두면 서버가 "Argo CD" 로 준 항목을 "ArgoCD" 로 못 찾는다.
function normalizeToolName(name: string): string {
  return name.trim().toLowerCase().replace(/[\s_-]+/g, "");
}

export function buildOssLoginHint(
  tool: ToolCredentialResponse | undefined,
  namespace: string,
  isKorean = false,
): string {
  if (!tool) {
    return isKorean
      ? "접속 정보를 불러오는 중입니다."
      : "Loading connection info...";
  }
  if (!tool.secret_ref) {
    return (
      tool.note ??
      (isKorean
        ? "도구별 기본 인증정보를 확인하세요."
        : "Check the default credentials for each tool.")
    );
  }

  const ns = namespace.trim() || "<namespace>";
  const key = tool.secret_key || "password";
  const cmd = `kubectl -n ${ns} get secret ${tool.secret_ref} -o jsonpath='{.data.${key}}' | base64 -d`;
  return tool.username ? `ID: ${tool.username} / Password: ${cmd}` : cmd;
}

export function buildConnectionInfoText(
  stackName: string,
  conn: StackConnectionInfo,
  launchTools: LaunchTool[],
  toolCredentials: ToolCredentialResponse[] | undefined,
  isKorean = false,
  gatewayPFCommand?: string,
): string {
  const ossLines = launchTools
    .map(
      (tool) =>
        `- ${tool.name}: ${tool.url ?? (isKorean ? "(URL 없음)" : "(No URL)")} | ${buildOssLoginHint(findToolCredential(toolCredentials, tool.name), conn.namespace, isKorean)}`,
    )
    .join("\n");
  const gatewayLines = gatewayPFCommand
    ? ["", "[Gateway Port-Forward]", gatewayPFCommand]
    : [];

  return [
    `[Stack] ${stackName}`,
    `[Access Domain] ${conn.accessDomain || "-"}`,
    ...(conn.accessDomain
      ? [
          `[Primary URLs] https://gitlab.${conn.accessDomain} | https://argocd.${conn.accessDomain} | https://minio.${conn.accessDomain} | https://openbao.${conn.accessDomain}`,
        ]
      : []),
    "",
    "[OSS Login]",
    ossLines,
    "",
    "[Database]",
    `- mode=${conn.database.mode}`,
    `- engine=${conn.database.providerOrEngine}`,
    `- endpoint=${conn.database.endpoint}`,
    `- db=${conn.database.resourceName}`,
    `- user=${conn.database.authId}`,
    `- secret=${conn.database.accessSecretRef} (key=${conn.database.authPasswordKey})`,
    "",
    "[Object Storage]",
    `- mode=${conn.objectStorage.mode}`,
    `- provider=${conn.objectStorage.providerOrEngine}`,
    `- endpoint=${conn.objectStorage.endpoint}`,
    `- bucket=${conn.objectStorage.resourceName}`,
    `- accessKey=${conn.objectStorage.authId}`,
    `- secret=${conn.objectStorage.accessSecretRef} (key=${conn.objectStorage.authPasswordKey})`,
    ...gatewayLines,
  ].join("\n");
}

export function toolLaunchURL(
  toolName: string,
  accessDomain: string,
): string | null {
  if (!accessDomain) {
    return null;
  }
  const key = toolName
    .toLowerCase()
    .replace(/[_-]+/g, " ")
    .replace(/\s+/g, " ")
    .trim();
  if (["gitlab", "gitlab ce", "gitlab ci", "gitlab registry"].includes(key))
    return `http://gitlab.${accessDomain}`;
  if (["argocd", "argo cd"].includes(key))
    return `http://argocd.${accessDomain}`;
  if (key === "grafana") return `http://grafana.${accessDomain}`;
  if (key === "prometheus") return `http://prometheus.${accessDomain}`;
  if (key === "harbor") return `http://harbor.${accessDomain}`;
  if (key === "minio") return `http://minio.${accessDomain}`;
  if (key === "opensearch") return `http://opensearch.${accessDomain}`;
  if (key === "elasticsearch") return `http://kibana.${accessDomain}`;
  if (key === "jaeger") return `http://jaeger.${accessDomain}`;
  if (["tempo", "loki", "opentelemetry collector"].includes(key))
    return `http://grafana.${accessDomain}`;
  if (key === "openbao") return `http://openbao.${accessDomain}`;
  return null;
}
