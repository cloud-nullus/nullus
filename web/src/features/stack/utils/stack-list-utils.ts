import type { TFunction } from "i18next"

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

export function buildHostsText(stackName: string, accessDomain: string, launchTools: LaunchTool[]): string {
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

	return [
		`# Nullus Stack: ${stackName}`,
		`127.0.0.1 ${hosts.join(" ")}`,
	].join("\n");
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


export function normalizeStackStatus(status: string, clusterConnectionStatus?: string): string {
	if (status === "success" || status === "running") return "healthy";
	if (status === "completed" && clusterConnectionStatus === "connected") return "healthy";
	return status;
}

export function isHealthyStatus(status: string, clusterConnectionStatus?: string): boolean {
	const normalized = normalizeStackStatus(status, clusterConnectionStatus);
	return normalized === "healthy";
}

export function matchesStackStatusFilter(status: string, filter: string, clusterConnectionStatus?: string): boolean {
	if (!filter) return true;
	const normalized = normalizeStackStatus(status, clusterConnectionStatus);
	if (filter === "healthy") return normalized === "healthy";
	if (filter === "running") return status === "running";
	if (filter === "completed") return status === "completed" && clusterConnectionStatus !== "connected";
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

function readNumber(record: Record<string, unknown>, keys: string[]): number | null {
	for (const key of keys) {
		const value = record[key];
		if (typeof value === "number" && Number.isFinite(value)) {
			return value;
		}
	}
	return null;
}

function readBool(record: Record<string, unknown>, keys: string[], defaultValue = true): boolean {
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
	const version = readString(record, ["version", "Version", "app_version", "appVersion"]) || "-";
	const instances = Math.max(1, Math.floor(readNumber(record, ["instances", "replicas", "count"]) ?? 1));

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

function pickGroup(config: Record<string, unknown>, keys: string[]): Record<string, unknown> {
	for (const key of keys) {
		const group = toRecord(config[key]);
		if (group) {
			return group;
		}
	}
	return {};
}

function parseCategorySelections(group: Record<string, unknown>, keyPairs: string[][]): ToolSelectionView[] {
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

function toPipelineNode(category: string, tools: ToolSelectionView[], color: string): PipelineNode | null {
	if (tools.length === 0) {
		return null;
	}
	return {
		category,
		oss: tools.map((tool) => tool.name).join(" + "),
		version: tools.map((tool) => tool.version).join(" / "),
		instances: tools.reduce((sum, tool) => sum + tool.instances, 0),
		color,
		health: "healthy",
		sync: "synced",
	};
}

export function buildPipelineNodesFromSnapshot(snapshot: unknown): PipelineNode[] {
	const config = resolveSnapshotConfig(snapshot);
	const artifacts = parseCategorySelections(
		pickGroup(config, ["artifacts", "Artifacts"]),
		[["package_registry", "packageRegistry"], ["source_repository", "sourceRepository"], ["container_registry", "containerRegistry"], ["storage_backend", "storageBackend"]],
	);
	const pipeline = pickGroup(config, ["pipeline", "Pipeline"]);
	const ci = parseCategorySelections(pipeline, [["ci_platform", "ciPlatform"]]);
	const cd = parseCategorySelections(pipeline, [["cd_tool", "cdTool"]]);
	const monitoring = parseCategorySelections(
		pickGroup(config, ["monitoring", "Monitoring"]),
		[["collection", "Collection"], ["visualization", "Visualization"]],
	);
	const loggingGroup = pickGroup(config, ["logging", "Logging"]);
	const logging = parseCategorySelections(loggingGroup, [["collection", "Collection"], ["search", "Search"]]);
	const trace = parseCategorySelections(loggingGroup, [["trace_layer", "traceLayer", "TraceLayer"]]);

	return [
		toPipelineNode("Artifacts", artifacts, "#6366f1"),
		toPipelineNode("CI", ci, "#0ea5e9"),
		toPipelineNode("CD", cd, "#8b5cf6"),
		toPipelineNode("Monitoring", monitoring, "#10b981"),
		toPipelineNode("Logging", logging, "#f59e0b"),
		toPipelineNode("Trace", trace, "#ef4444"),
	].filter((node): node is PipelineNode => !!node);
}

export function buildPipelineNodesFromMonitoring(tools: MonitoringToolView[] | undefined): PipelineNode[] {
	const enabledTools = (tools ?? []).filter((tool) => tool.enabled);
	const toNode = (category: string, keys: string[], color: string): PipelineNode | null => {
		const matches = enabledTools.filter((tool) => keys.includes(tool.key));
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
		toNode("Artifacts", ["source_repository", "storage_backend"], "#6366f1"),
		toNode("CD", ["cd_tool"], "#8b5cf6"),
		toNode("Monitoring", ["collection", "visualization"], "#10b981"),
		toNode("Logging", ["logging_collection", "logging_search"], "#f59e0b"),
		toNode("Trace", ["trace_layer"], "#ef4444"),
	].filter((node): node is PipelineNode => !!node);
}

export function buildInstalledToolsFromSnapshot(snapshot: unknown): ToolSelectionView[] {
	const config = resolveSnapshotConfig(snapshot);
	const artifacts = parseCategorySelections(
		pickGroup(config, ["artifacts", "Artifacts"]),
		[["package_registry", "packageRegistry"], ["source_repository", "sourceRepository"], ["container_registry", "containerRegistry"], ["storage_backend", "storageBackend"]],
	);
	const pipeline = parseCategorySelections(
		pickGroup(config, ["pipeline", "Pipeline"]),
		[["ci_platform", "ciPlatform"], ["cd_tool", "cdTool"]],
	);
	const monitoring = parseCategorySelections(
		pickGroup(config, ["monitoring", "Monitoring"]),
		[["collection", "Collection"], ["visualization", "Visualization"]],
	);
	const logging = parseCategorySelections(
		pickGroup(config, ["logging", "Logging"]),
		[["collection", "Collection"], ["search", "Search"], ["trace_layer", "traceLayer", "TraceLayer"]],
	);
	const authenticationGroup = pickGroup(config, ["authentication", "Authentication"]);
	const authProvider = readString(authenticationGroup, ["provider", "Provider", "name", "Name", "tool"]);
	const authentication: ToolSelectionView[] = authProvider
		? [{ name: authProvider, version: "shared", instances: 1 }]
		: [];

	const byName = new Map<string, ToolSelectionView>();
	for (const tool of [...authentication, ...artifacts, ...pipeline, ...monitoring, ...logging]) {
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

export function deriveGatewayName(accessDomain: string, stackName: string): string {
	const base = (accessDomain || fallbackAccessDomain(stackName))
		.replace(/\.internal$/i, "")
		.toLowerCase()
		.replace(/[^a-z0-9-]+/g, "-")
		.replace(/-+/g, "-")
		.replace(/^-+|-+$/g, "");
	return `${base || "nullus-stack"}-gateway`;
}

export function extractAccessDomain(snapshot: unknown, stackName: string): string {
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

function readStorageTarget(record: Record<string, unknown>, fallback: Partial<StorageConnectionInfo>): StorageConnectionInfo {
	return {
		mode: readString(record, ["mode", "Mode"]) || fallback.mode || "create",
		providerOrEngine: readString(record, ["provider_or_engine", "providerOrEngine", "ProviderOrEngine"]) || fallback.providerOrEngine || "-",
		endpoint: readString(record, ["endpoint", "Endpoint"]) || fallback.endpoint || "-",
		resourceName: readString(record, ["resource_name", "resourceName", "ResourceName"]) || fallback.resourceName || "-",
		authId: readString(record, ["auth_id", "authId", "AuthID", "user", "username"]) || fallback.authId || "-",
		accessSecretRef: readString(record, ["access_secret_ref", "accessSecretRef", "AccessSecretRef", "secret", "secretRef"]) || fallback.accessSecretRef || "-",
		authPasswordKey: readString(record, ["auth_password_key", "authPasswordKey", "AuthPasswordKey", "passwordKey"]) || fallback.authPasswordKey || "-",
	};
}

export function extractConnectionInfo(snapshot: unknown, namespace: string, accessDomain: string): StackConnectionInfo {
	const config = resolveSnapshotConfig(snapshot);
	const storage = pickGroup(config, ["storage", "Storage"]);
	const db = pickGroup(storage, ["database", "Database"]);
	const objectStorage = pickGroup(storage, ["object_storage", "objectStorage", "ObjectStorage"]);

	const ns = namespace.trim() || "nullus";
	const dbFallback: Partial<StorageConnectionInfo> = {
		mode: "create",
		providerOrEngine: "postgres",
		endpoint: `${ns}-postgresql:5432`,
		resourceName: "nullus",
		authId: "postgres",
		accessSecretRef: `${ns}-postgresql`,
		authPasswordKey: "postgres-password",
	};
	const objectFallback: Partial<StorageConnectionInfo> = {
		mode: "create",
		providerOrEngine: "minio",
		endpoint: `http://${ns}-minio:9000`,
		resourceName: "nullus-artifacts",
		authId: "nullus",
		accessSecretRef: `${ns}-minio`,
		authPasswordKey: "root-password",
	};

	return {
		accessDomain,
		database: readStorageTarget(db, dbFallback),
		objectStorage: readStorageTarget(objectStorage, objectFallback),
	};
}

export function buildOssLoginHint(toolName: string, conn: StackConnectionInfo, isKorean = false): string {
	const key = toolName.toLowerCase().replace(/[_-]+/g, " ").replace(/\s+/g, " ").trim();
	if (["argocd", "argo cd"].includes(key)) {
		return "ID: admin / Password: kubectl -n nullus get secret argo-cd-argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d";
	}
	if (["gitlab", "gitlab ce", "gitlab ci", "gitlab registry"].includes(key)) {
		return "ID: root / Password: kubectl -n nullus get secret gitlab-gitlab-initial-root-password -o jsonpath='{.data.password}' | base64 -d";
	}
	if (key === "grafana") {
		return isKorean ? "기본값: admin / admin (또는 values override 확인)" : "Default: admin / admin (or check values override)";
	}
	if (key === "minio") {
		return `ID: ${conn.objectStorage.authId} / SecretRef: ${conn.objectStorage.accessSecretRef} (key: ${conn.objectStorage.authPasswordKey})`;
	}
	if (key === "opensearch") {
		return isKorean ? "ID: admin / 비밀번호: NullusAdmin123! (기본값, 변경 시 values 확인)" : "ID: admin / Password: NullusAdmin123! (default value, check values if changed)";
	}
	if (key === "prometheus") {
		return isKorean ? "로그인 불필요 (기본 설정)" : "No login required (default setting)";
	}
	if (key === "openbao") {
		return isKorean ? "관리자 인증 후 OpenBao UI에서 토큰/시크릿을 조회하세요." : "After admin authentication, check tokens/secrets in OpenBao UI.";
	}
	return isKorean ? "도구별 기본 인증정보를 확인하세요." : "Check the default credentials for each tool.";
}

export function buildConnectionInfoText(stackName: string, conn: StackConnectionInfo, launchTools: LaunchTool[], isKorean = false, gatewayPFCommand?: string): string {
	const ossLines = launchTools
		.map((tool) => `- ${tool.name}: ${tool.url ?? (isKorean ? "(URL 없음)" : "(No URL)") } | ${buildOssLoginHint(tool.name, conn, isKorean)}`)
		.join("\n");
	const gatewayLines = gatewayPFCommand
		? [
			"",
			"[Gateway Port-Forward]",
			gatewayPFCommand,
		]
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

export function toolLaunchURL(toolName: string, accessDomain: string): string | null {
  if (!accessDomain) {
    return null;
  }
  const key = toolName.toLowerCase().replace(/[_-]+/g, " ").replace(/\s+/g, " ").trim();
	if (["gitlab", "gitlab ce", "gitlab ci", "gitlab registry"].includes(key)) return `http://gitlab.${accessDomain}`;
	if (["argocd", "argo cd"].includes(key)) return `http://argocd.${accessDomain}`;
	if (key === "grafana") return `http://grafana.${accessDomain}`;
	if (key === "prometheus") return `http://prometheus.${accessDomain}`;
  if (key === "harbor") return `http://harbor.${accessDomain}`;
  if (key === "minio") return `http://minio.${accessDomain}`;
	if (key === "opensearch") return `http://opensearch.${accessDomain}`;
	if (key === "elasticsearch") return `http://kibana.${accessDomain}`;
	if (key === "jaeger") return `http://jaeger.${accessDomain}`;
	if (["tempo", "loki", "opentelemetry collector"].includes(key)) return `http://grafana.${accessDomain}`;
	if (key === "openbao") return `http://openbao.${accessDomain}`;
	return null;
}
