import { useState } from "react"
import { useTranslation } from "react-i18next"
import { ClipboardList, ExternalLink, GitBranch, Plus } from "lucide-react"
import { Button } from "../../../components/ui/button"
import { Modal } from "../../../components/ui/modal"
import { cn } from "../../../lib/utils"
import type { Stack } from "../api/stack-api"
import { useStackHistory, useStackMonitoring } from "../api/stack-api"
import { RetryStackButton } from "./retry-stack-button"
import type { StackStatus as RetryStackStatus } from "../utils/retry-policy"
import type { PipelineNode, LaunchTool } from "../utils/stack-list-utils"
import { toolLogoURL } from "../utils/tool-logo"
import {
  buildPipelineNodesFromSnapshot,
  buildPipelineNodesFromMonitoring,
  buildInstalledToolsFromSnapshot,
  extractAccessDomain,
  toolLaunchURL,
  buildHostsText,
  extractConnectionInfo,
  buildConnectionInfoText,
  buildOssLoginHint,
  deriveGatewayName,
  toShellSingleQuoted,
  copyTextToClipboard,
  getStackStatusLabel,
} from "../utils/stack-list-utils"
import {
  ArtifactsPanel,
  PipelineToolsPanel,
  MonitoringToolsPanel,
  LoggingToolsPanel,
  ResourcesPanel,
} from "./stack-info-panels"

function ToolLogo({ name, logo }: Pick<LaunchTool, "name" | "logo">) {
	const [hasError, setHasError] = useState(false);

	if (hasError) {
		return (
			<span aria-label={`${name} logo fallback`}>
				{name.charAt(0).toUpperCase()}
			</span>
		);
	}

	return (
		<img
			src={logo}
			alt={`${name} logo`}
			className="absolute inset-0 h-full w-full object-contain p-0.5"
			onError={() => setHasError(true)}
		/>
	);
}

export function StackInfoTab({
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
	const { data: monitoringData } = useStackMonitoring(stack.id, 30_000);
	const latestSnapshot = Array.isArray(historyData) && historyData.length > 0
		? historyData[historyData.length - 1].snapshot
		: null;
	const derivedNodes = buildPipelineNodesFromSnapshot(latestSnapshot);
	const monitoringNodes = buildPipelineNodesFromMonitoring(monitoringData?.oss_statuses);
	const pipelineNodes: PipelineNode[] =
		derivedNodes.length > 0
			? derivedNodes
			: monitoringNodes.length > 0
				? monitoringNodes
				: [{ category: "Stack", oss: stack.templateName, version: "-", instances: 1, color: "#6366f1", health: "progressing", sync: "out-of-sync" }];

	const degradedState = ["failed", "rolling_back", "rolled_back", "cancelled"].includes(stack.status);
	const progressingState = ["pending", "terminating", "validating", "installing", "configuring", "health_check"].includes(stack.status);
	const runtimeNodes = pipelineNodes.map((node) => ({
		...node,
		health: degradedState ? "degraded" : progressingState ? "progressing" : "healthy",
		sync: degradedState ? "out-of-sync" : "synced",
	}));
	const snapshotTools = buildInstalledToolsFromSnapshot(latestSnapshot);
	const installedTools = snapshotTools.length > 0
		? snapshotTools
		: (monitoringData?.oss_statuses ?? [])
			.filter((tool) => tool.enabled)
			.map((tool) => ({ name: tool.name, version: tool.version }));
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
										<ToolLogo name={tool.name} logo={tool.logo} />
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
