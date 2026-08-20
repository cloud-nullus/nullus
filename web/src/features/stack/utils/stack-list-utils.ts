import type { TFunction } from "i18next";

export type PipelineTool = ToolSelectionView & {
  /**
   * 앞선 스테이지에 이미 나온 도구. Nexus 처럼 한 제품이 여러 역할을 겸하면
   * 각 역할 칸에 다 보여야 하지만(그게 역할 배치 정보다), 인스턴스는 한 번만
   * 센다 — 안 그러면 파드 수가 부풀려진다.
   */
  shared?: boolean;
  /** 이 도구를 실제로 돌리는 배포의 이름. gitlab-ci 의 sharedWith 는 gitlab 이다. */
  sharedWith?: string;
  /**
   * 실제로 떠 있는지. 설정(스냅샷)에는 없는 값이라 모니터링에서 겹쳐 넣는다
   * — applyToolRuntimeStatus. 모니터링을 아직 못 받았으면 undefined 다.
   */
  status?: ToolRuntimeStatus;
  /** 실제로 떠 있는 파드 수. */
  runtimeInstances?: number;
  /** 그중 준비된 파드 수. */
  readyInstances?: number;
};

export type PipelineNode = {
  category: string;
  /**
   * 스테이지에 속한 도구들. 이름과 버전을 따로 join 한 문자열 두 개로 들고
   * 있으면("a + b", "1.0 / 2.0") 읽는 쪽이 자리를 세서 짝을 맞춰야 한다.
   */
  tools: PipelineTool[];
  /** 이 스테이지의 파드 수. shared 도구는 빼고 센다. */
  instances: number;
};
// health/sync 는 여기 없다. 두 값 모두 stack.status 하나에서 파생되므로 노드마다
// 들고 있으면 같은 값이 스테이지 수만큼 복제된다. 스택 단위 상태는 화면 헤더가
// 스택에서 직접 읽는다.

export type ToolSelectionView = {
  name: string;
  version: string;
  instances: number;
};

export type ToolRuntimeStatus = "running" | "warning" | "error";

export type MonitoringToolView = {
  key: string;
  name: string;
  version: string;
  enabled: boolean;
  pod_count: number;
  status?: ToolRuntimeStatus;
  ready_pods?: number;
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

// 한 스테이지 안에서 같은 제품이 두 번 나오면 한 번만 남긴다.
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
  tools: PipelineTool[],
): PipelineNode | null {
  const distinct = dedupeByName(tools);
  if (distinct.length === 0) {
    return null;
  }
  return {
    category,
    tools: distinct,
    instances: distinct.reduce((sum, tool) => sum + tool.instances, 0),
  };
}

// 구분자를 '-' 로 통일하되 지우지는 않는다. 아래 접두사 규칙이 구분자에 기대기
// 때문이다 — 파일 아래쪽 normalizeToolName 은 구분자를 통째로 지우는 별개 함수로,
// 인증정보 조회("Argo CD" == "argocd")에 쓴다. 여기서 그걸 쓰면 argocd 가 argo 의
// 하위 제품으로 잘못 묶인다.
function toProductName(name: string): string {
  return name.trim().toLowerCase().replace(/[_\s]+/g, "-");
}

/**
 * 두 이름이 같은 배포인가.
 *
 * 스냅샷은 역할별 이름을 쓴다: gitlab / gitlab-ci / gitlab-registry /
 * gitlab-package-registry. 넷 다 GitLab 설치 하나가 맡은 역할이고, 모니터링은
 * 그걸 "gitlab" 하나로 보고한다(파드 15개가 그 아래 다 들어 있다).
 *
 * 별칭 목록을 손으로 들고 있지 않는다 — 도구가 늘 때마다 갱신해야 하고 빠뜨리면
 * 조용히 틀린다. 대신 "구분자를 사이에 둔 접두사면 같은 제품" 규칙을 쓴다.
 * grafana 와 tempo 처럼 로고만 공유하는 별개 배포는 이 규칙에 걸리지 않는다
 * (tool-logo.ts 의 로고 매핑을 이 판정에 재사용하면 안 되는 이유다).
 */
function isSameProduct(candidate: string, base: string): boolean {
  return candidate === base || candidate.startsWith(`${base}-`);
}

/**
 * 스테이지를 가로질러 같은 배포를 표시한다.
 *
 * Nexus 는 컨테이너 레지스트리이면서 패키지 저장소다. 역할별로 칸을 나눈 뒤에는
 * 두 칸에 다 나와야 배치가 보이지만, 파드는 하나다. 두 번째부터는 shared 로
 * 표시하고 그 스테이지의 인스턴스 합계에서 뺀다.
 */
function markSharedAcrossStages(nodes: PipelineNode[]): PipelineNode[] {
  const seen: string[] = [];
  return nodes.map((node) => {
    const tools = node.tools.map((tool) => {
      const key = toProductName(tool.name);
      const base = seen.find((name) => isSameProduct(key, name));
      if (base) {
        return { ...tool, shared: true, sharedWith: base };
      }
      seen.push(key);
      return tool;
    });
    return {
      ...node,
      tools,
      instances: tools.reduce(
        (sum, tool) => (tool.shared ? sum : sum + tool.instances),
        0,
      ),
    };
  });
}

export function buildPipelineNodesFromSnapshot(
  snapshot: unknown,
): PipelineNode[] {
  const config = resolveSnapshotConfig(snapshot);
  // Artifacts 를 한 칸에 몰지 않는다. 소스 저장소·컨테이너 레지스트리·패키지
  // 저장소·스토리지는 파이프라인에서 서는 자리가 각각 다르다. 한 칸에 합치면
  // "gitlab + gitlab-registry + minio" 처럼 역할이 사라진 이름 나열이 된다.
  const artifactsGroup = pickGroup(config, ["artifacts", "Artifacts"]);
  const source = parseCategorySelections(artifactsGroup, [
    ["source_repository", "sourceRepository"],
  ]);
  const containerRegistry = parseCategorySelections(artifactsGroup, [
    ["container_registry", "containerRegistry"],
  ]);
  const packageRegistry = parseCategorySelections(artifactsGroup, [
    ["package_registry", "packageRegistry"],
  ]);
  const storage = parseCategorySelections(artifactsGroup, [
    ["storage_backend", "storageBackend"],
  ]);
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
    // 수집기는 추적 저장소와 별개 워크로드다. 여기 없으면 파드가 떠 있어도
    // 스택 상세의 관측 단계에 나오지 않는다.
    ["trace_exporter", "traceExporter", "TraceExporter"],
  ]);

  // 순서는 코드가 흐르는 순서다: 소스 → 빌드 → 저장 → 배포 → 관측.
  return markSharedAcrossStages(
    [
      toPipelineNode("Source", source),
      toPipelineNode("CI", ci),
      toPipelineNode("Container Registry", containerRegistry),
      toPipelineNode("Package Registry", packageRegistry),
      toPipelineNode("Storage", storage),
      toPipelineNode("CD", cd),
      toPipelineNode("Monitoring", monitoring),
      toPipelineNode("Logging", logging),
      toPipelineNode("Trace", trace),
    ].filter((node): node is PipelineNode => !!node),
  );
}

/**
 * 설정에서 만든 스테이지에 "실제로 떠 있는지" 를 겹쳐 넣는다.
 *
 * 스냅샷(설치할 때 고른 것)에는 런타임이 없고, 모니터링에는 역할 배치가 없다.
 * 둘을 도구 이름으로 맞춘다 — 파이프라인 그림 안에서 동작 여부까지 보이게 하려면
 * 이 합류가 필요하다. 개편 전에는 이 정보가 모니터링 대시보드의 별도 카드에만
 * 있어서, 스택 상세를 보던 사람이 화면을 옮겨야 했다.
 */
export function applyToolRuntimeStatus(
  nodes: PipelineNode[],
  statuses: MonitoringToolView[] | undefined,
): PipelineNode[] {
  if (!statuses || statuses.length === 0) {
    return nodes;
  }
  // 이름이 정확히 같은 것을 먼저 찾고, 없으면 상위 배포를 찾는다. 스냅샷의
  // gitlab-ci 는 모니터링에 없다 — 그 역할을 gitlab 설치 하나가 맡고 있어서
  // 모니터링은 "gitlab" 하나로만 보고한다.
  const runtimeOf = (name: string): MonitoringToolView | undefined => {
    const key = toProductName(name);
    return (
      statuses.find((tool) => toProductName(tool.name) === key) ??
      statuses.find((tool) => isSameProduct(key, toProductName(tool.name)))
    );
  };
  return nodes.map((node) => ({
    ...node,
    tools: node.tools.map((tool) => {
      const runtime = runtimeOf(tool.name);
      if (!runtime) {
        return tool;
      }
      return {
        ...tool,
        status: runtime.status,
        // 분모는 설정값(instances)이 아니라 실제 파드 수다. 둘은 단위가 다르다 —
        // GitLab 은 설정상 1 인데 Helm 차트가 파드 15개를 띄운다. 섞으면 "15/1"
        // 같은 값이 나온다.
        runtimeInstances: runtime.pod_count,
        readyInstances: runtime.ready_pods,
      };
    }),
  }));
}

export function buildPipelineNodesFromMonitoring(
  tools: MonitoringToolView[] | undefined,
): PipelineNode[] {
  const enabledTools = (tools ?? []).filter((tool) => tool.enabled);
  const toNode = (category: string, keys: string[]): PipelineNode | null =>
    toPipelineNode(
      category,
      enabledTools
        .filter((tool) => keys.includes(tool.key))
        .map((tool) => ({
          name: tool.name,
          version: tool.version,
          instances: tool.pod_count,
          status: tool.status,
          readyInstances: tool.ready_pods,
        })),
    );

  // 스테이지 구성은 스냅샷 경로와 같다 — 두 경로가 다르면 데이터 출처에 따라
  // 같은 스택이 다른 모양으로 보인다.
  return markSharedAcrossStages(
    [
      toNode("Source", ["source_repository"]),
      toNode("CI", ["ci_platform"]),
      toNode("Container Registry", ["container_registry"]),
      toNode("Package Registry", ["package_registry"]),
      toNode("Storage", ["storage_backend"]),
      toNode("CD", ["cd_tool"]),
      toNode("Monitoring", ["collection", "visualization"]),
      toNode("Logging", ["logging_collection", "logging_search"]),
      toNode("Trace", ["trace_layer", "trace_exporter"]),
    ].filter((node): node is PipelineNode => !!node),
  );
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
      ["trace_exporter", "traceExporter", "TraceExporter"],
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

/**
 * 접속 도메인에서 도구의 웹 주소를 만든다.
 *
 * 스킴은 항상 https 다 — 접속 도메인은 게이트웨이 TLS 리스너 뒤에 서고, 서버의
 * 단일 출처(internal/stack/domain/tool_access_url.go)도 같은 규칙으로 내려준다.
 * 여기만 http 로 두면 화면마다 다른 주소를 안내하게 된다.
 *
 * 이 함수는 서버가 아직 주소를 주지 못할 때의 대비책이다. 서버 응답이 있으면
 * resolveToolLaunchURL 을 거쳐 그쪽을 먼저 쓴다.
 */
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
    return `https://gitlab.${accessDomain}`;
  if (key.startsWith("gitea")) return `https://gitea.${accessDomain}`;
  if (key.startsWith("jenkins")) return `https://jenkins.${accessDomain}`;
  if (key === "nexus") return `https://nexus.${accessDomain}`;
  if (["argocd", "argo cd"].includes(key))
    return `https://argocd.${accessDomain}`;
  if (key === "grafana") return `https://grafana.${accessDomain}`;
  if (key === "prometheus") return `https://prometheus.${accessDomain}`;
  if (key === "harbor") return `https://harbor.${accessDomain}`;
  if (key === "minio") return `https://minio.${accessDomain}`;
  if (key === "opensearch") return `https://opensearch.${accessDomain}`;
  if (key === "elasticsearch") return `https://kibana.${accessDomain}`;
  if (key === "jaeger") return `https://jaeger.${accessDomain}`;
  if (["tempo", "loki", "opentelemetry collector"].includes(key))
    return `https://grafana.${accessDomain}`;
  if (key === "openbao") return `https://openbao.${accessDomain}`;
  return null;
}

/**
 * 도구의 접속 주소를 정한다. 서버가 준 값이 먼저다.
 *
 * 주소의 단일 출처는 서버(스택 모니터링 응답의 oss_statuses[].url)다. 화면이
 * 접속 도메인으로 주소를 다시 조립하면 설치 규칙이 바뀌는 순간 조용히 갈라진다 —
 * 그래서 서버 값이 있으면 그것을 쓰고, 아직 없을 때만 규칙으로 만든다.
 */
export function resolveToolLaunchURL(
  toolName: string,
  serverTools: Array<{ name: string; url?: string }> | undefined,
  accessDomain: string,
): string | null {
  const wanted = toolName.trim().toLowerCase();
  const fromServer = (serverTools ?? []).find(
    (tool) => tool.name.trim().toLowerCase() === wanted,
  )?.url;
  if (fromServer && fromServer.trim()) {
    return fromServer.trim();
  }
  return toolLaunchURL(toolName, accessDomain);
}
