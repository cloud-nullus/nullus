import type { ColumnDef } from "@tanstack/react-table";
import {
	Archive,
	ArrowUpCircle,
	BarChart2,
	Boxes,
	Check,
	ClipboardList,
	FileText,
	GitBranch,
	History,
	Info,
	Layers,
	List,
	Monitor,
	ExternalLink,
	Plus,
	Search,
	Server,
	Terminal,
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { Breadcrumb } from "../../../components/shared/breadcrumb";
import { ConfirmDialog } from "../../../components/shared/confirm-dialog";
import { DataTable } from "../../../components/shared/data-table";
import { Button } from "../../../components/ui/button";
import { Modal } from "../../../components/ui/modal";
import { NativeSelect } from "../../../components/ui/native-select";
import { cn } from "../../../lib/utils";
import { StackMonitoringOverview } from "../../observability/components/stack-monitoring-overview";
import type { Stack } from "../api/stack-api";
import { useDeleteStack, useStackHistory, useStacks } from "../api/stack-api";
import { RetryStackButton } from "../components/retry-stack-button";
import type { StackStatus as RetryStackStatus } from "../utils/retry-policy";
import { useScopedClusters } from "../../admin/api/admin-api";

type InnerTab = "info" | "monitoring" | "history" | "version-upgrade";

type PipelineNode = {
	category: string;
	oss: string;
	version: string;
	instances: number;
	color: string;
	health: "healthy" | "progressing" | "degraded";
	sync: "synced" | "out-of-sync";
};

type ToolSelectionView = {
	name: string;
	version: string;
	instances: number;
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

function buildHostsText(stackName: string, accessDomain: string, launchTools: LaunchTool[]): string {
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

async function copyTextToClipboard(value: string): Promise<void> {
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

import { STATUS_STYLES } from "../utils/status-style";
import { useKeyboardShortcut } from "../../../hooks/use-keyboard-shortcut";

function getStackStatusLabel(t: TFunction, status: string) {
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


function normalizeStackStatus(status: string, clusterConnectionStatus?: string): string {
	if (status === "success" || status === "running") return "healthy";
	if (status === "completed" && clusterConnectionStatus === "connected") return "healthy";
	return status;
}

function isHealthyStatus(status: string, clusterConnectionStatus?: string): boolean {
	const normalized = normalizeStackStatus(status, clusterConnectionStatus);
	return normalized === "healthy";
}

function matchesStackStatusFilter(status: string, filter: string, clusterConnectionStatus?: string): boolean {
	if (!filter) return true;
	const normalized = normalizeStackStatus(status, clusterConnectionStatus);
	if (filter === "healthy") return normalized === "healthy";
	if (filter === "running") return status === "running";
	if (filter === "completed") return status === "completed" && clusterConnectionStatus !== "connected";
	return normalized === filter;
}

function formatDate(iso: string) {
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

function toShellSingleQuoted(value: string): string {
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

function buildPipelineNodesFromSnapshot(snapshot: unknown): PipelineNode[] {
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

function buildInstalledToolsFromSnapshot(snapshot: unknown): ToolSelectionView[] {
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

function deriveGatewayName(accessDomain: string, stackName: string): string {
	const base = (accessDomain || fallbackAccessDomain(stackName))
		.replace(/\.internal$/i, "")
		.toLowerCase()
		.replace(/[^a-z0-9-]+/g, "-")
		.replace(/-+/g, "-")
		.replace(/^-+|-+$/g, "");
	return `${base || "nullus-stack"}-gateway`;
}

function extractAccessDomain(snapshot: unknown, stackName: string): string {
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

function toolLogoURL(toolName: string): string {
	const key = toolName.toLowerCase().replace(/[_-]+/g, " ").replace(/\s+/g, " ").trim();
	const map: Record<string, string> = {
		gitlab: "gitlab",
		"gitlab ce": "gitlab",
		"gitlab ci": "gitlab",
		"gitlab registry": "gitlab",
		github: "github",
		"github actions": "githubactions",
		nexus: "sonatype",
		"nexus repository": "sonatype",
		"nexus repository manager": "sonatype",
		argocd: "argo",
		"argo cd": "argo",
		flux: "flux",
		"flux cd": "flux",
		fluxcd: "flux",
		grafana: "grafana",
		prometheus: "prometheus",
		thanos: "thanos",
		loki: "grafana",
		opensearch: "opensearch",
		elasticsearch: "elasticsearch",
		"opentelemetry collector": "opentelemetry",
		tempo: "grafana",
		jaeger: "jaeger",
		harbor: "harbor",
		"harbor registry": "harbor",
		minio: "minio",
		openbao: "vault",
	};
	const slug = map[key] ?? "kubernetes";
	return `https://cdn.simpleicons.org/${slug}`;
}

function toolLaunchURL(toolName: string, accessDomain: string): string | null {
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


function ConfigCard({
	title,
	icon,
	children,
}: {
	title: string;
	icon: React.ReactNode;
	children: React.ReactNode;
}) {
	return (
		<div className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)]">
			<div className="flex items-center gap-2 border-b border-[var(--color-border-default)] px-4 py-3">
				<span className="text-[var(--color-text-secondary)]">{icon}</span>
				<h4 className="m-0 text-[13px] font-semibold text-[var(--color-text-primary)]">
					{title}
				</h4>
			</div>
			<div className="p-4">{children}</div>
		</div>
	);
}

function ToolOption({
	checked,
	title,
	desc,
	version,
	versions,
	instances = 1,
}: {
	checked: boolean;
	title: string;
	desc: string;
	version?: string;
	versions?: string[];
	instances?: number;
}) {
	return (
		<div
			className={cn(
				"flex flex-col gap-2 rounded-md border p-2.5",
				checked
					? "border-[rgba(99,102,241,0.35)]"
					: "border-[var(--color-border-default)] opacity-60",
			)}
		>
			<div className="flex items-start gap-2">
				<div
					className={cn(
						"mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded border",
						checked
							? "border-[#6366f1] bg-[#6366f1]"
							: "border-[var(--color-border-default)]",
					)}
				>
					{checked && <Check size={10} className="text-white" />}
				</div>
				<div>
					<div className="text-[13px] font-semibold text-[var(--color-text-primary)]">
						{title}
					</div>
					<div className="text-[11px] text-[var(--color-text-secondary)]">
						{desc}
					</div>
				</div>
			</div>
			{checked && version && (
				<div className="ml-6 flex flex-wrap items-center gap-3">
					<NativeSelect
						defaultValue={version}
						className="cursor-pointer rounded border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-2 py-1 text-[12px] text-[var(--color-text-primary)] [&>option]:bg-[var(--color-surface-base)] [&>option]:text-[var(--color-text-primary)]"
					>
						{(versions ?? [version]).map((v) => (
							<option key={v} className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">{v}</option>
						))}
					</NativeSelect>
					<div className="flex items-center gap-1.5">
						<span className="text-[11px] text-[#6366f1]">Instances:</span>
						<div className="flex items-center">
							<button
								type="button"
								className="flex h-6 w-6 items-center justify-center rounded-l border border-[var(--color-border-default)] text-[var(--color-text-secondary)] text-xs"
							>
								-
							</button>
							<input
								type="number"
								defaultValue={instances}
								min={1}
								max={3}
								className="h-6 w-9 border-y border-[var(--color-border-default)] bg-transparent text-center text-[12px] text-[var(--color-text-primary)] [appearance:textfield]"
							/>
							<button
								type="button"
								className="flex h-6 w-6 items-center justify-center rounded-r border border-[var(--color-border-default)] text-[var(--color-text-secondary)] text-xs"
							>
								+
							</button>
						</div>
					</div>
				</div>
			)}
		</div>
	);
}

function PanelHeader({ title, desc }: { title: string; desc: string }) {
	return (
		<div className="mb-5">
			<h3 className="m-0 mb-1 text-[15px] font-bold text-[var(--color-text-primary)]">
				{title}
			</h3>
			<p className="m-0 text-[12px] text-[var(--color-text-secondary)]">
				{desc}
			</p>
		</div>
	);
}

function ArtifactsPanel() {
	return (
		<div>
			<PanelHeader
				title="Artifact Configuration"
				desc="현재 스택에 구성된 아티팩트 저장소"
			/>
			<div className="grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-4">
				<ConfigCard title="Package Registry" icon={<Archive size={14} />}>
					<div className="flex flex-col gap-2">
						<ToolOption
							checked
							title="GitLab Package Registry"
							desc="Default integrated solution"
							version="v16.7 (Latest)"
							versions={["v16.7 (Latest)", "v16.6", "v15.11 (LTS)"]}
							instances={1}
						/>
						<ToolOption
							checked={false}
							title="Nexus Repository"
							desc="Enterprise artifact management"
						/>
					</div>
				</ConfigCard>
				<ConfigCard
					title="Source Code Repository"
					icon={<GitBranch size={14} />}
				>
					<div className="flex flex-col gap-2">
						<ToolOption
							checked
							title="GitLab"
							desc="Complete DevOps platform"
							version="v16.7"
							versions={["v16.7", "v16.6"]}
							instances={1}
						/>
						<ToolOption
							checked={false}
							title="GitHub"
							desc="Cloud-based repository"
						/>
					</div>
				</ConfigCard>
				<ConfigCard title="Container Registry" icon={<Boxes size={14} />}>
					<div className="flex flex-col gap-2">
						<ToolOption
							checked
							title="Harbor"
							desc="Enterprise container registry"
							version="v2.8.2"
							versions={["v2.8.2", "v2.7.4"]}
							instances={1}
						/>
						<ToolOption
							checked={false}
							title="AWS ECR"
							desc="Amazon Elastic Container Registry"
						/>
					</div>
				</ConfigCard>
			</div>
		</div>
	);
}

function PipelineToolsPanel() {
	return (
		<div>
			<PanelHeader
				title="Pipeline Tools"
				desc="현재 스택의 CI/CD 파이프라인 도구 구성"
			/>
			<div className="grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-4">
				<ConfigCard title="CI/CD Platform" icon={<GitBranch size={14} />}>
					<div className="flex flex-col gap-2">
						<ToolOption
							checked
							title="GitLab CI/CD"
							desc="Integrated with GitLab SCM"
							version="v16.7"
							instances={2}
						/>
						<ToolOption
							checked={false}
							title="Jenkins"
							desc="Open-source automation server"
						/>
					</div>
				</ConfigCard>
				<ConfigCard
					title="Continuous Deployment"
					icon={<ArrowUpCircle size={14} />}
				>
					<div className="flex flex-col gap-2">
						<ToolOption
							checked
							title="Argo CD"
							desc="GitOps CD for Kubernetes"
							version="v2.9.3"
							versions={["v2.9.3", "v2.8.4"]}
							instances={1}
						/>
						<ToolOption
							checked={false}
							title="Flux"
							desc="GitOps toolkit for Kubernetes"
						/>
					</div>
				</ConfigCard>
			</div>
		</div>
	);
}

function MonitoringToolsPanel() {
	return (
		<div>
			<PanelHeader
				title="Monitoring Tools"
				desc="현재 스택의 모니터링 도구 구성"
			/>
			<div className="grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-4">
				<ConfigCard title="Metrics Collection" icon={<BarChart2 size={14} />}>
					<div className="flex flex-col gap-2">
						<ToolOption
							checked
							title="Prometheus"
							desc="Time-series metrics collection"
							version="v2.48.1"
							instances={1}
						/>
						<ToolOption
							checked={false}
							title="Thanos"
							desc="Long-term metrics storage"
						/>
					</div>
				</ConfigCard>
				<ConfigCard title="Visualization" icon={<Monitor size={14} />}>
					<div className="flex flex-col gap-2">
						<ToolOption
							checked
							title="Grafana"
							desc="Dashboard & visualization"
							version="v10.3"
							versions={["v10.3", "v10.2"]}
							instances={1}
						/>
						<ToolOption
							checked={false}
							title="Datadog"
							desc="Cloud monitoring platform"
						/>
					</div>
				</ConfigCard>
			</div>
		</div>
	);
}

function LoggingToolsPanel() {
	return (
		<div>
			<PanelHeader title="Logging Tools" desc="현재 스택의 로깅 도구 구성" />
			<div className="grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-4">
				<ConfigCard title="Log Collection" icon={<FileText size={14} />}>
					<div className="flex flex-col gap-2">
						<ToolOption
							checked
							title="Loki"
							desc="Log aggregation system"
							version="v2.9.3"
							instances={1}
						/>
						<ToolOption
							checked={false}
							title="OpenSearch"
							desc="Search and analytics engine"
						/>
					</div>
				</ConfigCard>
			</div>
		</div>
	);
}

function ResourcesPanel() {
	return (
		<div>
			<PanelHeader title="Resources" desc="현재 스택의 리소스 할당 현황" />
			<div className="mb-6 grid grid-cols-3 gap-4">
				{[
					{ label: "CPU", value: "8", unit: "cores", color: "#6366f1" },
					{ label: "Memory", value: "32", unit: "Gi", color: "#10b981" },
					{ label: "Storage", value: "500", unit: "Gi", color: "#f59e0b" },
				].map((item) => (
					<div
						key={item.label}
						className="flex flex-col items-center rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-5"
					>
						<h4 className="m-0 mb-2 text-[13px] font-semibold text-[var(--color-text-primary)]">
							{item.label}
						</h4>
						<div
							className="text-[28px] font-bold"
							style={{ color: item.color }}
						>
							{item.value}{" "}
							<span className="text-[14px] text-[var(--color-text-secondary)]">
								{item.unit}
							</span>
						</div>
						<div className="mt-1 text-[12px] text-[var(--color-text-secondary)]">
							할당된 {item.label}
						</div>
					</div>
				))}
			</div>
			<div className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-4">
				<h4 className="mb-3 flex items-center gap-1.5 text-[13px] font-semibold text-[var(--color-text-primary)]">
					<Server size={14} /> Cluster 정보
				</h4>
				<div className="flex flex-wrap gap-6 text-[13px] text-[var(--color-text-secondary)]">
					<span>
						<strong className="text-[var(--color-text-primary)]">
							Cluster:
						</strong>{" "}
						prod-k8s
					</span>
					<span>
						<strong className="text-[var(--color-text-primary)]">
							Namespace:
						</strong>{" "}
						devops
					</span>
					<span>
						<strong className="text-[var(--color-text-primary)]">
							Region:
						</strong>{" "}
						ap-northeast-2
					</span>
					<span>
						<strong className="text-[var(--color-text-primary)]">
							Node Count:
						</strong>{" "}
						6
					</span>
				</div>
			</div>
		</div>
	);
}

function StackInfoTab({
	stack,
	displayStatus,
	isDeleting,
	onAddTools,
	onDelete,
	onBackToList,
}: {
	stack: Stack;
	displayStatus: string;
	isDeleting: boolean;
	onAddTools: () => void;
	onDelete: () => void;
	onBackToList: () => void;
}) {
	const { t, i18n } = useTranslation();
	const isKorean = (i18n.resolvedLanguage ?? i18n.language ?? "").toLowerCase().startsWith("ko");
	const [hostsCopyState, setHostsCopyState] = useState<"idle" | "copied" | "failed">("idle");
	const [gatewayCopyState, setGatewayCopyState] = useState<"idle" | "copied" | "failed">("idle");
	const [connOpen, setConnOpen] = useState(false);
	const [connCopyState, setConnCopyState] = useState<"idle" | "copied" | "failed">("idle");
	const { data: historyData } = useStackHistory(stack.id);
	const latestSnapshot = Array.isArray(historyData) && historyData.length > 0
		? historyData[historyData.length - 1].snapshot
		: null;
	const derivedNodes = buildPipelineNodesFromSnapshot(latestSnapshot);
	const pipelineNodes: PipelineNode[] =
		derivedNodes.length > 0
			? derivedNodes
			: [{ category: "Stack", oss: stack.templateName, version: "-", instances: 1, color: "#6366f1", health: "progressing", sync: "out-of-sync" }];

	const degradedState = ["failed", "rolling_back", "rolled_back", "cancelled"].includes(stack.status);
	const progressingState = ["pending", "terminating", "validating", "installing", "configuring", "health_check"].includes(stack.status);
	const runtimeNodes = pipelineNodes.map((node) => ({
		...node,
		health: degradedState ? "degraded" : progressingState ? "progressing" : "healthy",
		sync: degradedState ? "out-of-sync" : "synced",
	}));
	const installedTools = buildInstalledToolsFromSnapshot(latestSnapshot);
	const accessDomain = extractAccessDomain(latestSnapshot, stack.name);
	const launchTools: LaunchTool[] = installedTools.map((tool) => ({
		name: tool.name,
		version: tool.version,
		url: toolLaunchURL(tool.name, accessDomain),
		logo: toolLogoURL(tool.name),
	}));
	const hostsText = buildHostsText(stack.name, accessDomain, launchTools);
	const connectionInfo = extractConnectionInfo(latestSnapshot, stack.namespace?.trim() || "nullus", accessDomain);
	const stackNamespace = stack.namespace?.trim() || "nullus";
	const stackNamespaceArg = toShellSingleQuoted(stackNamespace);
	const gatewayNameArg = toShellSingleQuoted(deriveGatewayName(accessDomain, stack.name));
	const accessHostArg = toShellSingleQuoted(accessDomain || `${stack.name}.internal`);
	const gatewayPFCommand = [
		isKorean ? "# 80/443 동시 포트포워드 (Gateway 서비스 자동 선택)" : "# Port-forward both 80/443 (auto-select Gateway service)",
		`KUBE_CONTEXT=kind-nullus-platform STACK_NAMESPACE=${stackNamespaceArg} GATEWAY_NAME=${gatewayNameArg} ACCESS_HOST=${accessHostArg} sudo -E ./scripts/port-forward-gateway.sh`,
	].join("\n");
	const connectionInfoWithGatewayText = buildConnectionInfoText(stack.name, connectionInfo, launchTools, isKorean, gatewayPFCommand);

	const handleCopyHosts = async () => {
		if (!hostsText) return;
		try {
			await copyTextToClipboard(hostsText);
			setHostsCopyState("copied");
		} catch {
			setHostsCopyState("failed");
		}
		setTimeout(() => setHostsCopyState("idle"), 2200);
	};

	const handleCopyConnectionInfo = async () => {
		try {
			await copyTextToClipboard(connectionInfoWithGatewayText);
			setConnCopyState("copied");
		} catch {
			setConnCopyState("failed");
		}
		setTimeout(() => setConnCopyState("idle"), 2200);
	};

	const handleCopyGatewayPF = async () => {
		try {
			await copyTextToClipboard(gatewayPFCommand);
			setGatewayCopyState("copied");
		} catch {
			setGatewayCopyState("failed");
		}
		setTimeout(() => setGatewayCopyState("idle"), 2200);
	};

	const observabilitySummary = ["Monitoring", "Logging", "Trace"]
		.filter((category) => pipelineNodes.some((node) => node.category === category))
		.join(" + ") || "Not configured";

	return (
		<div className="flex flex-col gap-6">
			<div className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-4">
				<div className="mb-3 flex flex-col gap-3">
					<div>
						<div className="flex items-center gap-2">
							<div className="text-[14px] font-bold text-[var(--color-text-primary)]">Installed Stack Summary</div>
						</div>
						<div className="text-[12px] text-[var(--color-text-secondary)]">{t("stackList.connection.summary", "Deployed configuration summary and key actions")}</div>
					</div>
					<div className="flex flex-wrap items-center gap-2">
						<Button
							variant="outline"
							size="sm"
							type="button"
							onClick={handleCopyGatewayPF}
							title={t("stackList.connection.gatewayCopyTitle", "Copy gateway port-forward command")}
						>
							<ClipboardList size={13} />
							{gatewayCopyState === "copied"
								? "Copied"
								: gatewayCopyState === "failed"
									? "Copy Failed"
									: "Gateway PF Copy"}
						</Button>
						<Button
							variant="outline"
							size="sm"
							type="button"
							onClick={handleCopyHosts}
							disabled={!hostsText}
							title={hostsText
								? t("stackList.connection.hostsCopyTitle", "Copy /etc/hosts mappings")
								: t("stackList.connection.hostsCopyUnavailable", "Cannot build hosts mappings because access domain is missing")}
						>
							<ClipboardList size={13} />
							{hostsCopyState === "copied"
								? "Copied"
								: hostsCopyState === "failed"
									? "Copy Failed"
									: "/etc/hosts Copy"}
						</Button>
						<Button variant="outline" size="sm" type="button" onClick={onAddTools}>
							<Plus size={13} /> Add Tools
						</Button>
						<RetryStackButton
							stackId={stack.id}
							status={stack.status as RetryStackStatus}
						/>
						{displayStatus === "terminating" && (
							<Button variant="outline" size="sm" type="button" onClick={onBackToList}>
								{t("stackList.actions.backToList", "Stack List로 돌아가기")}
							</Button>
						)}
						<Button variant="danger" size="sm" type="button" onClick={onDelete} disabled={isDeleting || displayStatus === "terminating"} loading={isDeleting && displayStatus !== "terminating"}>
							Delete
						</Button>
					</div>
				</div>
				<div className="grid grid-cols-2 gap-3 text-[12px] text-[var(--color-text-secondary)] lg:grid-cols-4">
					<div className="rounded-md border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.03)] px-3 py-2">
						<div className="text-[11px] uppercase tracking-[0.04em]">Stack Name</div>
						<div className="mt-1 truncate font-semibold text-[var(--color-text-primary)]" title={stack.name}>{stack.name}</div>
					</div>
					<div className="rounded-md border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.03)] px-3 py-2">
						<div className="text-[11px] uppercase tracking-[0.04em]">Runtime</div>
						<div className="mt-1 font-semibold text-[var(--color-text-primary)]">Kubernetes / Helm</div>
					</div>
					<div className="rounded-md border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.03)] px-3 py-2">
						<div className="text-[11px] uppercase tracking-[0.04em]">Observability</div>
						<div className="mt-1 font-semibold text-[var(--color-text-primary)]">{observabilitySummary}</div>
					</div>
					<div className="rounded-md border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.03)] px-3 py-2">
						<div className="text-[11px] uppercase tracking-[0.04em]">Update Mode</div>
						<div className="mt-1 font-semibold text-[var(--color-text-primary)]">{getStackStatusLabel(t, displayStatus)}</div>
					</div>
				</div>
				<div className="mt-3 border-t border-[var(--color-border-default)] pt-3">
					<div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.05em] text-[var(--color-text-secondary)]">
						Open Source Consoles
					</div>
					<div className="flex flex-wrap gap-2">
						{launchTools.map((tool) => (
							<a
								key={tool.name}
									href={tool.url ?? undefined}
									target="_blank"
									rel="noreferrer"
									onClick={(event) => {
										if (!tool.url) {
											event.preventDefault();
										}
									}}
									className={cn(
										"inline-flex items-center gap-2 rounded-md border px-2.5 py-1.5 text-[12px] transition-colors",
										tool.url
											? "border-[var(--color-border-default)] bg-[rgba(255,255,255,0.03)] text-[var(--color-text-primary)] hover:border-[rgba(99,102,241,0.45)] hover:bg-[rgba(99,102,241,0.1)]"
											: "cursor-not-allowed border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] text-[var(--color-text-muted)]",
									)}
									title={tool.url ? `${tool.name} 콘솔 열기` : `${tool.name}: 경로 미설정`}
							>
									<span className="relative flex h-5 w-5 items-center justify-center overflow-hidden rounded-sm bg-[rgba(255,255,255,0.08)] text-[10px] font-bold uppercase text-[var(--color-text-secondary)]">
										<img
											src={tool.logo}
											alt={`${tool.name} logo`}
											className="absolute inset-0 h-full w-full object-contain p-0.5"
											onError={(event) => {
												event.currentTarget.style.display = "none";
											}}
										/>
									</span>
									<span className="font-medium">{tool.name}</span>
									<span className="text-[10px] text-[var(--color-text-secondary)]">{tool.version}</span>
									<ExternalLink size={12} />
							</a>
						))}
					</div>
					<div className="mt-2 text-[11px] text-[var(--color-text-secondary)]">
						{t("stackList.connection.gatewayGuide", "After single-entry gateway port forwarding, access each OSS domain based on hosts mappings.")}
					</div>
					<div className="mt-3 flex justify-end">
						<Button size="sm" variant="outline" type="button" onClick={() => setConnOpen(true)}>
							{t("stackList.connection.open", "Connection Info")}
						</Button>
					</div>
				</div>
			</div>

			<Modal
				open={connOpen}
				onClose={() => setConnOpen(false)}
				title={t("stackList.connection.open", "Connection Info")}
				wide
				footer={(
					<>
						<Button variant="ghost" size="sm" type="button" onClick={handleCopyConnectionInfo}>
							{connCopyState === "copied"
								? t("stackList.connection.copied", "Copied")
								: connCopyState === "failed"
									? t("stackList.connection.copyFailed", "Copy failed")
									: t("stackList.connection.copyAll", "Copy all")}
						</Button>
						<Button variant="secondary" size="sm" type="button" onClick={() => setConnOpen(false)}>
							{t("common.cancel", "Close")}
						</Button>
					</>
				)}
			>
				<div className="space-y-4 text-[13px]">
					<div className="rounded-md border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-3 py-2 text-[var(--color-text-secondary)]">
						<span className="font-semibold text-[var(--color-text-primary)]">Access Domain:</span> {connectionInfo.accessDomain || "-"}
					</div>

					<div>
						<div className="mb-2 text-[12px] font-semibold uppercase tracking-[0.05em] text-[var(--color-text-secondary)]">OSS Login</div>
						<div className="space-y-2">
							{launchTools.map((tool) => (
								<div key={`conn-${tool.name}`} className="rounded-md border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-3">
									<div className="flex flex-wrap items-center justify-between gap-2">
										<div className="font-semibold text-[var(--color-text-primary)]">{tool.name}</div>
										<a
											href={tool.url ?? undefined}
											target="_blank"
											rel="noreferrer"
											onClick={(event) => {
												if (!tool.url) event.preventDefault();
											}}
											className={cn("text-[12px] underline", tool.url ? "text-[#93c5fd]" : "pointer-events-none text-[var(--color-text-muted)]")}
										>
									{tool.url || (isKorean ? "URL 없음" : "No URL")}
								</a>
							</div>
									<div className="mt-1 text-[12px] text-[var(--color-text-secondary)]">{buildOssLoginHint(tool.name, connectionInfo, isKorean)}</div>
								</div>
							))}
						</div>
					</div>

					<div className="grid gap-3 md:grid-cols-2">
						<div className="rounded-md border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-3">
							<div className="mb-2 text-[12px] font-semibold uppercase tracking-[0.05em] text-[var(--color-text-secondary)]">Database</div>
							<div className="space-y-1 text-[12px] text-[var(--color-text-secondary)]">
								<div><strong className="text-[var(--color-text-primary)]">Mode:</strong> {connectionInfo.database.mode}</div>
								<div><strong className="text-[var(--color-text-primary)]">Engine:</strong> {connectionInfo.database.providerOrEngine}</div>
								<div><strong className="text-[var(--color-text-primary)]">Endpoint:</strong> {connectionInfo.database.endpoint}</div>
								<div><strong className="text-[var(--color-text-primary)]">DB:</strong> {connectionInfo.database.resourceName}</div>
								<div><strong className="text-[var(--color-text-primary)]">User:</strong> {connectionInfo.database.authId}</div>
								<div><strong className="text-[var(--color-text-primary)]">Secret:</strong> {connectionInfo.database.accessSecretRef} ({connectionInfo.database.authPasswordKey})</div>
							</div>
						</div>

						<div className="rounded-md border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-3">
							<div className="mb-2 text-[12px] font-semibold uppercase tracking-[0.05em] text-[var(--color-text-secondary)]">Object Storage</div>
							<div className="space-y-1 text-[12px] text-[var(--color-text-secondary)]">
								<div><strong className="text-[var(--color-text-primary)]">Mode:</strong> {connectionInfo.objectStorage.mode}</div>
								<div><strong className="text-[var(--color-text-primary)]">Provider:</strong> {connectionInfo.objectStorage.providerOrEngine}</div>
								<div><strong className="text-[var(--color-text-primary)]">Endpoint:</strong> {connectionInfo.objectStorage.endpoint}</div>
								<div><strong className="text-[var(--color-text-primary)]">Bucket:</strong> {connectionInfo.objectStorage.resourceName}</div>
								<div><strong className="text-[var(--color-text-primary)]">Access Key:</strong> {connectionInfo.objectStorage.authId}</div>
								<div><strong className="text-[var(--color-text-primary)]">Secret:</strong> {connectionInfo.objectStorage.accessSecretRef} ({connectionInfo.objectStorage.authPasswordKey})</div>
							</div>
						</div>
					</div>
				</div>
			</Modal>

			<div className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-4">
				<div className="mb-3 flex items-center gap-2">
					<GitBranch size={14} className="text-[#818cf8]" />
					<div className="text-[14px] font-bold text-[var(--color-text-primary)]">Pipeline Topology</div>
				</div>
				<div className="mb-3 flex flex-wrap items-center gap-2 text-[11px]">
					<span className={cn(
						"inline-flex items-center gap-1 rounded-full px-2 py-0.5 font-semibold",
						degradedState
							? "bg-[rgba(239,68,68,0.15)] text-[#fca5a5]"
							: progressingState
								? "bg-[rgba(59,130,246,0.15)] text-[#93c5fd]"
								: "bg-[rgba(34,197,94,0.15)] text-[#86efac]",
					)}>
						● Health {degradedState ? "Degraded" : progressingState ? "Progressing" : "Healthy"}
					</span>
					<span className={cn(
						"inline-flex items-center gap-1 rounded-full px-2 py-0.5 font-semibold",
						degradedState
							? "bg-[rgba(245,158,11,0.15)] text-[#fcd34d]"
							: "bg-[rgba(16,185,129,0.15)] text-[#6ee7b7]",
					)}>
						◉ Sync {degradedState ? "OutOfSync" : "Synced"}
					</span>
				</div>
				<div className="relative overflow-x-auto pb-1">
					<div className="relative z-10 grid min-w-max grid-flow-col auto-cols-auto gap-3">
						{runtimeNodes.map((node, idx) => (
							<div key={node.category} className="relative rounded-md border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.03)] px-3 py-3 shadow-[inset_0_0_0_1px_rgba(255,255,255,0.02)]">
								{idx < runtimeNodes.length - 1 && (
									<div className="pointer-events-none absolute right-[-16px] top-3 h-[2px] w-8 bg-gradient-to-r from-[rgba(148,163,184,0.25)] to-[rgba(148,163,184,0.62)]" aria-hidden="true">
										<div className="absolute right-0 top-1/2 h-[7px] w-[7px] -translate-y-1/2 rotate-45 border-r-2 border-t-2 border-[rgba(148,163,184,0.72)]" />
									</div>
								)}
								<div className="mb-2 flex items-center gap-2">
									<div className="flex items-center gap-2">
										<div className="h-6 w-6 rounded-full ring-2 ring-white/10" style={{ backgroundColor: node.color }} />
										<div className="text-[12px] font-semibold text-[var(--color-text-primary)]">{node.category}</div>
									</div>
									<span className={cn(
										"rounded-full px-2 py-0.5 text-[10px] font-semibold",
										node.health === "degraded"
											? "bg-[rgba(239,68,68,0.15)] text-[#fca5a5]"
											: node.health === "progressing"
												? "bg-[rgba(59,130,246,0.15)] text-[#93c5fd]"
												: "bg-[rgba(34,197,94,0.15)] text-[#86efac]",
									)}>
										{node.health}
									</span>
								</div>
								<div className="mb-2 flex items-center gap-1.5">
									<span className={cn(
										"rounded-full px-2 py-0.5 text-[10px] font-semibold",
										node.sync === "synced"
											? "bg-[rgba(16,185,129,0.15)] text-[#6ee7b7]"
											: "bg-[rgba(245,158,11,0.15)] text-[#fcd34d]",
									)}>
										{node.sync}
									</span>
								</div>
								<div className="text-[11px] text-[var(--color-text-secondary)]">OSS</div>
								<div className="mb-1 text-[12px] font-medium text-[var(--color-text-primary)]">{node.oss}</div>
								<div className="text-[11px] text-[var(--color-text-secondary)]">Version</div>
								<div className="mb-1 text-[12px] font-medium text-[var(--color-text-primary)]">{node.version}</div>
								<div className="text-[11px] text-[var(--color-text-secondary)]">Instances</div>
								<div className="text-[12px] font-medium text-[var(--color-text-primary)]">{node.instances}</div>
							</div>
						))}
					</div>
				</div>
			</div>

			<div className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-4 py-3 text-[12px] text-[var(--color-text-secondary)]">
				{t("stackList.hiddenInstallCardsNotice", "Detailed install cards are hidden. Check detailed tool status in the Monitoring / History tabs.")}
			</div>
			<div className="hidden" aria-hidden="true">
				<ArtifactsPanel />
				<PipelineToolsPanel />
				<MonitoringToolsPanel />
				<LoggingToolsPanel />
				<ResourcesPanel />
			</div>
		</div>
	);
}

function StackMonitoringTab({ stackId }: { stackId: string }) {
	return <StackMonitoringOverview stackId={stackId} />;
}

function StackHistoryTab({ stack }: { stack: Stack }) {
	const navigate = useNavigate();
	const { data: historyData, isLoading } = useStackHistory(stack.id);
	const entries = Array.isArray(historyData) ? historyData : [];
	const latestEntryID = entries[entries.length - 1]?.id;

	return (
		<div className="flex h-full flex-col">
			<div className="mb-4 flex items-center justify-between gap-3">
				<div className="flex items-center gap-3">
					<div className="h-5 w-1 rounded-full bg-[linear-gradient(135deg,#10b981,#059669)]" />
					<h3 className="m-0 text-[14px] font-bold text-[var(--color-text-primary)]">{stack.name} History</h3>
				</div>
				<div className="flex items-center gap-2">
					<Button variant="outline" size="sm" onClick={() => navigate(`/stack/logs/${stack.id}`)} type="button">
						<Terminal size={13} /> Open Logs
					</Button>
					<Button variant="outline" size="sm" onClick={() => navigate(`/stack/history/${stack.id}`)} type="button">
						Open Full History
					</Button>
				</div>
			</div>
			{isLoading && (
				<div className="mb-3 rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-4 py-3 text-[13px] text-[var(--color-text-secondary)]">
					Loading history...
				</div>
			)}
			{!isLoading && entries.length === 0 && (
				<div className="mb-3 rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] px-4 py-3 text-[13px] text-[var(--color-text-secondary)]">
					No history found for this stack yet.
				</div>
			)}
			<div className="flex flex-1 flex-col gap-3 overflow-y-auto pr-1">
				{entries.map((entry) => {
					const isCurrent = entry.id === latestEntryID;
					return (
					<div
						key={entry.id}
						className="overflow-hidden rounded-lg border"
						style={{ borderColor: isCurrent ? "#bbf7d0" : "var(--color-border-default)" }}
					>
						<div className="flex flex-wrap items-center justify-between gap-3 bg-[rgba(255,255,255,0.04)] px-5 py-3">
							<div className="flex flex-wrap items-center gap-2.5">
								<span
									className="rounded-full px-2.5 py-0.5 text-[12px] font-bold text-white"
									style={{
										background: isCurrent
											? "#059669"
											: "#6366f1",
									}}
								>
									v{entry.version}{isCurrent ? " · Current" : ""}
								</span>
								<span
									className="rounded-[8px] px-2 py-0.5 text-[11px] font-semibold"
									style={{
										background: isCurrent ? "rgba(245,158,11,0.15)" : "rgba(99,102,241,0.15)",
										color: isCurrent ? "#fcd34d" : "#a5b4fc",
									}}
								>
									{isCurrent ? "Current Config" : "Version Snapshot"}
								</span>
							</div>
							<div className="text-[12px] text-[var(--color-text-secondary)]">
								👤 {entry.changedBy} &nbsp;🕐 {formatDate(entry.changedAt)}
							</div>
						</div>
						<div className="flex flex-wrap items-center justify-between gap-3 px-5 py-3">
							<div className="flex flex-wrap gap-5 text-[13px]">
								<span>
									<strong className="text-[var(--color-text-primary)]">
										Reason:
									</strong>{" "}
									<span className="text-[var(--color-text-secondary)]">
										{entry.reason || "N/A"}
									</span>
								</span>
							</div>
						</div>
					</div>
					);
				})}
			</div>
		</div>
	);
}

const UPGRADE_ITEMS = [
	{
		name: "GitLab",
		iconBg: "linear-gradient(135deg,#fc6d26,#e24329)",
		current: "v16.7",
		latest: "v16.9",
		tag: "Minor Update",
		tagBg: "rgba(245,158,11,0.15)",
		tagColor: "#fcd34d",
		upToDate: false,
	},
	{
		name: "Prometheus",
		iconBg: "linear-gradient(135deg,#e6522c,#cc3918)",
		current: "v2.48.1",
		latest: "v2.50.1",
		tag: "Patch Update",
		tagBg: "rgba(16,185,129,0.15)",
		tagColor: "#6ee7b7",
		upToDate: false,
	},
	{
		name: "Grafana",
		iconBg: "linear-gradient(135deg,#f46800,#d45a00)",
		current: "v10.3",
		latest: "v10.4",
		tag: "Minor Update",
		tagBg: "rgba(245,158,11,0.15)",
		tagColor: "#fcd34d",
		upToDate: false,
	},
	{
		name: "Argo CD",
		iconBg: "linear-gradient(135deg,#326ce5,#1e4db8)",
		current: "v2.9.3",
		latest: null,
		tag: null,
		tagBg: "",
		tagColor: "",
		upToDate: true,
	},
];

function StackVersionUpgradeTab() {
	const { t } = useTranslation();
	const handleUpgradeClick = () => {
		toast.info(t("stackList.toast.upgradeInProgress", "개발중인 기능입니다."));
	};

	return (
		<div>
			<div className="mb-6 flex flex-wrap items-center gap-3">
				<div className="h-5 w-1 rounded-full bg-[linear-gradient(135deg,#6366f1,#8b5cf6)]" />
				<h3 className="m-0 text-[14px] font-bold text-[var(--color-text-primary)]">
					Available Version Upgrades
				</h3>
				<span className="rounded-full bg-[rgba(99,102,241,0.15)] px-2.5 py-0.5 text-[12px] font-semibold text-[#a5b4fc]">
					3 updates available
				</span>
			</div>
			<div className="flex flex-col gap-3">
				{UPGRADE_ITEMS.map((item) => (
					<div
						key={item.name}
						className={cn(
							"flex flex-wrap items-center justify-between gap-3 rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-4",
							item.upToDate && "opacity-60",
						)}
					>
						<div className="flex items-center gap-3">
							<div
								className="flex h-9 w-9 items-center justify-center rounded-lg"
								style={{ background: item.iconBg }}
							>
								<GitBranch size={16} className="text-white" />
							</div>
							<div>
								<div className="font-semibold text-[var(--color-text-primary)]">
									{item.name}
								</div>
								{item.upToDate ? (
									<div className="text-[12px] text-[var(--color-text-secondary)]">
										Current: {item.current} →{" "}
										<strong className="text-[#6ee7b7]">Up to date</strong>
									</div>
								) : (
									<div className="text-[12px] text-[var(--color-text-secondary)]">
										Current: {item.current} → Latest:{" "}
										<strong className="text-[#6ee7b7]">{item.latest}</strong>
									</div>
								)}
							</div>
						</div>
						<div className="flex items-center gap-2.5">
							{item.upToDate ? (
								<span className="rounded-full bg-[rgba(16,185,129,0.15)] px-2.5 py-0.5 text-[11px] font-semibold text-[#6ee7b7]">
									✓ Up to date
								</span>
							) : (
								<>
									<span
										className="rounded-full px-2.5 py-0.5 text-[11px] font-semibold"
										style={{ background: item.tagBg, color: item.tagColor }}
									>
										{item.tag}
									</span>
									<button
										type="button"
										className="flex items-center gap-1.5 rounded-md border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-2.5 py-1.5 text-[12px] text-[var(--color-text-primary)]"
									>
										<ClipboardList size={12} /> Changelog
									</button>
									<button
										type="button"
										onClick={handleUpgradeClick}
										className="flex items-center gap-1.5 rounded-md bg-[linear-gradient(135deg,#6366f1,#8b5cf6)] px-2.5 py-1.5 text-[12px] font-semibold text-white"
									>
										<ArrowUpCircle size={12} /> Upgrade
									</button>
								</>
							)}
						</div>
					</div>
				))}
			</div>
		</div>
	);
}

const BASE_INNER_TABS: { key: InnerTab; label: string; icon: React.ReactNode }[] = [
	{ key: "info", label: "Info", icon: <Info size={13} /> },
	{ key: "history", label: "History", icon: <History size={13} /> },
	{
		key: "version-upgrade",
		label: "Version Upgrade",
		icon: <ArrowUpCircle size={13} />,
	},
];

function StackDetailPanel({
	stack,
	clusterConnectionStatus,
	isDeleting,
	onAddTools,
	onDelete,
	onBackToList,
	className,
}: {
	stack: Stack;
	clusterConnectionStatus?: string;
	isDeleting: boolean;
	onAddTools: () => void;
	onDelete: () => void;
	onBackToList: () => void;
	className?: string;
}) {
	const { t } = useTranslation();
	const [innerTab, setInnerTab] = useState<InnerTab>("info");
	const normalizedStatus = normalizeStackStatus(stack.status, clusterConnectionStatus);
	const canShowMonitoring = isHealthyStatus(stack.status, clusterConnectionStatus);
	const innerTabs = canShowMonitoring
		? [BASE_INNER_TABS[0], { key: "monitoring" as const, label: "Monitoring", icon: <BarChart2 size={13} /> }, ...BASE_INNER_TABS.slice(1)]
		: BASE_INNER_TABS;
	const statusStyle = STATUS_STYLES[normalizedStatus] ?? STATUS_STYLES.pending;

	useEffect(() => {
		if (!canShowMonitoring && innerTab === "monitoring") {
			setInnerTab("info");
		}
	}, [canShowMonitoring, innerTab]);

	return (
		<div className={cn("flex h-full flex-col overflow-hidden rounded-[var(--card-radius)] border border-[rgba(99,102,241,0.3)] bg-[var(--color-surface-card)]", className)}>
			<div className="flex items-center gap-3 border-b border-[var(--color-border-default)] px-5 py-3.5">
				<div className="flex h-8 w-8 items-center justify-center rounded-lg bg-[rgba(99,102,241,0.15)] text-[#818cf8]">
					<Layers size={16} />
				</div>
				<h3 className="m-0 text-[15px] font-bold text-[var(--color-text-primary)]">
					{stack.name}
				</h3>
				<span
					className="rounded-[10px] px-[9px] py-[3px] text-[11px] font-bold"
					style={{ background: statusStyle.bg, color: statusStyle.color }}
				>
					{getStackStatusLabel(t, normalizedStatus)}
				</span>
				<span className="text-[12px] text-[var(--color-text-secondary)]">
					· {stack.templateName} · {stack.clusterName}
				</span>
			</div>

			<div className="flex border-b border-[var(--color-border-default)]">
				{innerTabs.map((tab) => (
					<button
						key={tab.key}
						type="button"
						onClick={() => setInnerTab(tab.key)}
						className={cn(
							"flex items-center gap-1.5 border-b-2 px-4 py-2.5 text-[13px] font-medium transition-all duration-150",
							innerTab === tab.key
								? "border-b-[#6366f1] bg-[rgba(30,41,59,0.6)] text-[var(--color-text-primary)]"
								: "border-b-transparent text-[var(--color-text-secondary)] hover:bg-[rgba(99,102,241,0.08)] hover:text-[var(--color-text-primary)]",
						)}
					>
						{tab.icon} {t(`stackList.tabs.${tab.key}`, tab.label)}
					</button>
				))}
			</div>

			<div className="flex-1 overflow-auto p-5">
				{innerTab === "info" && (
					<StackInfoTab
						stack={stack}
						displayStatus={normalizedStatus}
						isDeleting={isDeleting}
						onAddTools={onAddTools}
						onDelete={onDelete}
						onBackToList={onBackToList}
					/>
				)}
				{innerTab === "monitoring" && canShowMonitoring && <StackMonitoringTab stackId={stack.id} />}
				{innerTab === "history" && <StackHistoryTab stack={stack} />}
				{innerTab === "version-upgrade" && <StackVersionUpgradeTab />}
			</div>
		</div>
	);
}

export function StackListPage() {
	const { t } = useTranslation();
	const navigate = useNavigate();
	// F8-UIUX-KeyboardHints — jump straight to the install wizard.
	useKeyboardShortcut("n", () => navigate("/stack/install"));
	const [search, setSearch] = useState("");
	const [statusFilter, setStatusFilter] = useState("");
	const [clusterFilter, setClusterFilter] = useState("");
	const [expandedStackId, setExpandedStackId] = useState<string | null>(null);
	const [deleteStackId, setDeleteStackId] = useState<string | null>(null);
	const [terminatingStatusByID, setTerminatingStatusByID] = useState<Record<string, true>>({});
	const [viewportHeight, setViewportHeight] = useState(() =>
		typeof window !== "undefined" ? window.innerHeight : 960,
	);
	const [viewportWidth, setViewportWidth] = useState(() =>
		typeof window !== "undefined" ? window.innerWidth : 1440,
	);
	const deleteStack = useDeleteStack();
	const tablePageSize = Math.max(6, Math.min(14, Math.floor((viewportHeight - 340) / 52)));
	const isDesktopLayout = viewportWidth >= 1280;

	useEffect(() => {
		if (typeof window === "undefined") {
			return;
		}
		const onResize = () => {
			setViewportHeight(window.innerHeight);
			setViewportWidth(window.innerWidth);
		};
		window.addEventListener("resize", onResize);
		return () => window.removeEventListener("resize", onResize);
	}, []);

	const normalizedStatusFilter = statusFilter === "healthy" ? "success" : statusFilter;
	const { data: clustersData } = useScopedClusters();
	const clusters = clustersData?.items ?? [];
	const shouldPollTerminating = Object.keys(terminatingStatusByID).length > 0;
	const { data: apiData, isLoading } = useStacks({
		search,
		status: normalizedStatusFilter || undefined,
		include_deleted: true,
	}, {
		refetchIntervalMs: shouldPollTerminating ? 3000 : 0,
	});
	const clusterNameByID = useMemo(
		() => new Map(clusters.map((cluster) => [cluster.id, cluster.name])),
		[clusters],
	);
	const clusterConnectionByID = useMemo(
		() => new Map(clusters.map((cluster) => [cluster.id, cluster.status])),
		[clusters],
	);
	const stacks = useMemo(
		() => (apiData?.items ?? []).map((item) => {
			const resolvedClusterName = clusterNameByID.get(item.clusterId);
			return {
				...item,
				status: terminatingStatusByID[item.id] ? "terminating" : item.status,
				clusterName: resolvedClusterName || item.clusterName || item.clusterId || "-",
			};
		}),
		[apiData?.items, clusterNameByID, terminatingStatusByID],
	);
	const clusterOptions = useMemo(() => Array.from(new Set(stacks.map((item) => item.clusterName).filter((name) => !!name))).sort(), [stacks]);

	useEffect(() => {
		if (Object.keys(terminatingStatusByID).length === 0) return;
		const visibleIDs = new Set(stacks.map((s) => s.id));
		setTerminatingStatusByID((prev) => {
			let changed = false;
			const next: Record<string, true> = {};
			for (const id of Object.keys(prev)) {
				if (visibleIDs.has(id)) {
					next[id] = true;
				} else {
					changed = true;
				}
			}
			return changed ? next : prev;
		});
	}, [stacks, terminatingStatusByID]);

	const filtered = stacks.filter((s) => {
		const q = search.toLowerCase();
		const matchesSearch =
			!search ||
			s.name.toLowerCase().includes(q) ||
			s.templateName.toLowerCase().includes(q) ||
			s.clusterName.toLowerCase().includes(q);
		const matchesStatus = matchesStackStatusFilter(s.status, statusFilter, clusterConnectionByID.get(s.clusterId));
		const matchesCluster = !clusterFilter || s.clusterName === clusterFilter;
		return matchesSearch && matchesStatus && matchesCluster;
	});
	const selectedStackId = expandedStackId && filtered.some((stack) => stack.id === expandedStackId)
		? expandedStackId
		: (filtered[0]?.id ?? null);
	const expandedStack = selectedStackId
		? filtered.find((s) => s.id === selectedStackId) ?? null
		: null;

	const handleDeleteStack = () => {
		if (!deleteStackId) return;
		const targetID = deleteStackId;
		setDeleteStackId(null);
		setTerminatingStatusByID((prev) => ({ ...prev, [targetID]: true }));
		setExpandedStackId((prev) => (prev === targetID ? null : prev));
		deleteStack.mutate(targetID, {
			onSuccess: () => {
				toast.success(t("stackList.delete.started", "Stack deletion started. Kubernetes resources and DB data are being removed."));
			},
			onError: () => {
				setTerminatingStatusByID((prev) => {
					const next = { ...prev };
					delete next[targetID];
					return next;
				});
				toast.error(t("stackList.delete.failed", "Failed to start stack deletion."));
			},
		});
	};

	const columns: ColumnDef<Stack, unknown>[] = [
		{
			accessorKey: "name",
			header: t("stackList.table.stackName", "Stack Name"),
			cell: ({ row }) => (
				<div className="flex items-center gap-2">
					{selectedStackId === row.original.id && (
						<div className="h-1.5 w-1.5 shrink-0 rounded-full bg-[#6366f1]" />
					)}
					<span className="truncate font-semibold" title={row.original.name}>{row.original.name}</span>
				</div>
			),
		},
				{
			accessorKey: "clusterName",
			header: t("stackList.table.cluster", "Cluster"),
			cell: ({ row }) => (
				<span className="text-[var(--color-text-secondary)]">
					{row.original.clusterName}
				</span>
			),
		},
		{
			accessorKey: "status",
			header: () => <span className="whitespace-nowrap">{t("stackList.table.status", "Status")}</span>,
			cell: ({ row }) => {
				const normalizedRowStatus = normalizeStackStatus(row.original.status, clusterConnectionByID.get(row.original.clusterId));
				const s = STATUS_STYLES[normalizedRowStatus] ?? STATUS_STYLES.pending;
				return (
					<span
						className="inline-block min-w-[72px] whitespace-nowrap rounded-md px-[9px] py-[3px] text-center text-xs font-semibold"
						style={{ backgroundColor: s.bg, color: s.color }}
					>
						{getStackStatusLabel(t, normalizedRowStatus)}
					</span>
				);
			},
		},
		{
			accessorKey: "createdAt",
			header: () => <span className="whitespace-nowrap">{t("stackList.table.createdAt", "Created At")}</span>,
			cell: ({ row }) => (
				<span className="whitespace-nowrap text-[13px] text-[var(--color-text-secondary)]">
					{formatDate(row.original.createdAt)}
				</span>
			),
		},
	];

	return (
		<div>
			<Breadcrumb items={[{ label: t("sidebar.stackList", "Stack List") }]} />

			<div className="mb-6 flex items-start justify-between">
				<div className="flex items-center gap-2.5">
					<div className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(99,102,241,0.15)] text-[#818cf8]">
						<List size={18} />
					</div>
					<div>
						<h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
							{t("stackList.title", "Stack List")}
						</h1>
						<p className="m-0 mt-0.5 text-[13px] text-[var(--color-text-secondary)]">
							{t("stackList.description", "Deployed DevSecOps stack list")}
						</p>
					</div>
				</div>
				<Button
					variant="primary"
					size="md"
					onClick={() =>
						navigate("/stack/templates", { state: { from: "stack-list" } })
					}
				>
					<Plus size={15} />
					{t("stackList.actions.newStack", "New Stack")}
				</Button>
			</div>

			<div className="grid gap-4 xl:grid-cols-[minmax(300px,38%)_minmax(0,62%)]">
				<div className="min-w-0">
					<DataTable
						key={`stack-list-${tablePageSize}`}
						columns={columns}
						data={filtered}
						pageSize={tablePageSize}
						toolbar={
							<>
								<NativeSelect
									value={statusFilter}
									onChange={(e) => setStatusFilter(e.target.value)}
									className="cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)] [&>option]:bg-[var(--color-surface-base)] [&>option]:text-[var(--color-text-primary)]"
								>
									<option value="" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">{t("stackList.filters.allStatus", "All Status")}</option>
									<option value="healthy" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">{t("stackList.status.healthy", "Running")}</option>
									<option value="completed" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">{t("stackList.status.completed", "Completed")}</option>
									<option value="running" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">{t("stackList.status.running", "Running")}</option>
									<option value="terminating" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">{t("stackList.status.terminating", "Terminating")}</option>
									<option value="pending" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">{t("stackList.status.pending", "Pending")}</option>
									<option value="failed" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">{t("stackList.status.failed", "Failed")}</option>
									<option value="cancelled" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">{t("stackList.status.cancelled", "Cancelled")}</option>
								</NativeSelect>
								<NativeSelect
									value={clusterFilter}
									onChange={(e) => setClusterFilter(e.target.value)}
									className="cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)] [&>option]:bg-[var(--color-surface-base)] [&>option]:text-[var(--color-text-primary)]"
								>
									<option value="" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">{t("stackList.filters.allClusters", "All Clusters")}</option>
									{clusterOptions.map((clusterName) => (
										<option key={clusterName} value={clusterName} className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">
											{clusterName}
										</option>
									))}
								</NativeSelect>
								<div className="relative ml-auto">
									<Search
										size={13}
										className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--color-text-secondary)]"
									/>
									<input
										placeholder={t("stackList.searchPlaceholder", "Search stacks...")}
										value={search}
										onChange={(e) => setSearch(e.target.value)}
										className="w-[220px] rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] py-[7px] pl-[30px] pr-3 text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)]"
									/>
								</div>
							</>
						}
						getRowKey={(row) => row.id}
						onRowClick={(row) => setExpandedStackId(row.id)}
						emptyMessage={isLoading ? t("stackList.loading", "Loading stacks...") : t("stackList.empty", "No stacks found.")}
					/>
					<div className="mt-2 hidden text-[12px] text-[var(--color-text-secondary)] xl:block">
						{t("stackList.listHint", "Selecting a stack from the list updates the detail panel immediately.")}
					</div>
				</div>

				{isDesktopLayout && (
					<div>
						{expandedStack ? (
							<div className="h-full pr-1">
								<StackDetailPanel
									key={expandedStack.id}
									stack={expandedStack}
									clusterConnectionStatus={clusterConnectionByID.get(expandedStack.clusterId)}
									isDeleting={deleteStack.isPending}
									onAddTools={() => navigate(`/stack/${expandedStack.id}/add-tools`)}
									onDelete={() => setDeleteStackId(expandedStack.id)}
									onBackToList={() => setExpandedStackId(null)}
								/>
							</div>
						) : (
							<div className="rounded-[var(--card-radius)] border border-dashed border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-8 text-center text-[13px] text-[var(--color-text-secondary)]">
								{t("stackList.emptyDetail", "Select a stack from the list to view details here.")}
							</div>
						)}
					</div>
				)}
			</div>

			{!isDesktopLayout && expandedStack && (
				<StackDetailPanel
					key={`${expandedStack.id}-mobile`}
					stack={expandedStack}
					clusterConnectionStatus={clusterConnectionByID.get(expandedStack.clusterId)}
					isDeleting={deleteStack.isPending}
					onAddTools={() => navigate(`/stack/${expandedStack.id}/add-tools`)}
					onDelete={() => setDeleteStackId(expandedStack.id)}
					onBackToList={() => setExpandedStackId(null)}
					className="mt-4"
				/>
			)}

			<ConfirmDialog
				open={deleteStackId !== null}
				onClose={() => setDeleteStackId(null)}
				onConfirm={handleDeleteStack}
				title={t("stackList.confirm.deleteTitle", "Delete Stack")}
				description={t("stackList.confirm.deleteDescription", "Deleting this stack may affect related deployment data. Continue?")}
				confirmLabel={t("common.delete", "Delete")}
				loading={deleteStack.isPending}
			/>
		</div>
	);
}
