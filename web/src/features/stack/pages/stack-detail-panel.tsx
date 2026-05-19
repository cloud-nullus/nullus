import {
	AlertCircle,
	Archive,
	ArrowUpCircle,
	BarChart2,
	Box,
	Boxes,
	Check,
	ChevronDown,
	ChevronUp,
	ClipboardList,
	FileText,
	GitBranch,
	History,
	Info,
	Layers,
	Monitor,
	RotateCcw,
	Server,
	Terminal,
	XCircle,
} from "lucide-react";
import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
	Bar,
	BarChart,
	CartesianGrid,
	Cell,
	Legend,
	Pie,
	PieChart,
	ResponsiveContainer,
	Tooltip,
	XAxis,
	YAxis,
} from "recharts";
import { NativeSelect } from "../../../components/ui/native-select";
import { cn } from "../../../lib/utils";
import type { Stack } from "../api/stack-api";
import { useStackWorkloads } from "../api/stack-api";
import type { StackWorkloadPipeline } from "../../../types";

type InnerTab = "info" | "monitoring" | "history" | "version-upgrade";

export const STATUS_STYLES: Record<string, { bg: string; color: string; label: string }> = {
	pending: { bg: "rgba(245,158,11,0.15)", color: "#f59e0b", label: "Pending" },
	validating: { bg: "rgba(99,102,241,0.15)", color: "#a5b4fc", label: "Validating" },
	installing: { bg: "rgba(59,130,246,0.15)", color: "#60a5fa", label: "Installing" },
	configuring: { bg: "rgba(59,130,246,0.15)", color: "#60a5fa", label: "Configuring" },
	health_check: { bg: "rgba(59,130,246,0.15)", color: "#60a5fa", label: "Health Check" },
	completed: { bg: "rgba(34,197,94,0.15)", color: "#22c55e", label: "Completed" },
	failed: { bg: "rgba(239,68,68,0.15)", color: "#ef4444", label: "Failed" },
	rolling_back: { bg: "rgba(245,158,11,0.15)", color: "#f59e0b", label: "Rolling Back" },
	rolled_back: { bg: "rgba(100,116,139,0.15)", color: "#64748b", label: "Rolled Back" },
	running: { bg: "rgba(59,130,246,0.15)", color: "#60a5fa", label: "Running" },
	success: { bg: "rgba(34,197,94,0.15)", color: "#22c55e", label: "Success" },
	cancelled: { bg: "rgba(100,116,139,0.15)", color: "#64748b", label: "Cancelled" },
};

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

function StackInfoTab() {
	return (
		<div className="flex flex-col gap-6">
			<div className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.01)] p-4">
				<div className="mb-4 flex items-center gap-2 text-[13px] font-semibold text-[var(--color-text-primary)]">
					<Archive size={13} /> Artifacts
				</div>
				<ArtifactsPanel />
			</div>

			<div className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.01)] p-4">
				<div className="mb-4 flex items-center gap-2 text-[13px] font-semibold text-[var(--color-text-primary)]">
					<GitBranch size={13} /> Pipeline Tools
				</div>
				<PipelineToolsPanel />
			</div>

			<div className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.01)] p-4">
				<div className="mb-4 flex items-center gap-2 text-[13px] font-semibold text-[var(--color-text-primary)]">
					<Monitor size={13} /> Monitoring Tools
				</div>
				<MonitoringToolsPanel />
			</div>

			<div className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.01)] p-4">
				<div className="mb-4 flex items-center gap-2 text-[13px] font-semibold text-[var(--color-text-primary)]">
					<FileText size={13} /> Logging Tools
				</div>
				<LoggingToolsPanel />
			</div>

			<div className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.01)] p-4">
				<div className="mb-4 flex items-center gap-2 text-[13px] font-semibold text-[var(--color-text-primary)]">
					<Server size={13} /> Resources
				</div>
				<ResourcesPanel />
			</div>
		</div>
	);
}

function UsageBar({ value, color }: { value: number; color: string }) {
	const normalized = Math.max(0, Math.min(100, value));
	return (
		<div className="mt-2 h-1.5 w-full overflow-hidden rounded-[3px] bg-[rgba(255,255,255,0.08)]">
			<svg
				className="h-full w-full"
				viewBox="0 0 100 6"
				preserveAspectRatio="none"
				aria-hidden="true"
			>
				<rect width={normalized} height="6" rx="3" fill={color} />
			</svg>
		</div>
	);
}

function K8sObjectBadge({ kind, status }: { kind: string; status?: string }) {
	const styles: Record<string, { bg: string; color: string }> = {
		Deployment: { bg: "rgba(99,102,241,0.15)", color: "#a5b4fc" },
		Pod: { bg: "rgba(34,197,94,0.15)", color: "#86efac" },
		Service: { bg: "rgba(59,130,246,0.15)", color: "#60a5fa" },
		Ingress: { bg: "rgba(245,158,11,0.15)", color: "#fcd34d" },
	};
	if (kind === "Pod" && status && status !== "Running") {
		if (status === "Pending") {
			return (
				<span className="rounded-md px-1.5 py-0.5 text-[10px] font-semibold bg-[rgba(245,158,11,0.15)] text-[#fbbf24]">
					{kind}
				</span>
			);
		}
		return (
			<span className="rounded-md px-1.5 py-0.5 text-[10px] font-semibold bg-[rgba(239,68,68,0.15)] text-[#ef4444]">
				{kind}
			</span>
		);
	}
	const s = styles[kind] ?? { bg: "rgba(107,114,128,0.15)", color: "#9ca3af" };
	return (
		<span
			className="rounded-md px-1.5 py-0.5 text-[10px] font-semibold"
			style={{ background: s.bg, color: s.color }}
		>
			{kind}
		</span>
	);
}

function WorkloadRow({ pipeline }: { pipeline: StackWorkloadPipeline }) {
	const [open, setOpen] = useState(true);
	const deployStatus = pipeline.lastDeployment?.status ?? "none";
	const statusStyle = STATUS_STYLES[deployStatus] ?? STATUS_STYLES.pending;

	return (
		<>
			<tr
				className="cursor-pointer border-b border-[var(--color-border-default)] transition-colors hover:bg-[rgba(99,102,241,0.06)]"
				onClick={() => setOpen((o) => !o)}
			>
				<td className="px-3 py-2.5">
					<button type="button" className="border-none bg-transparent p-0 text-[var(--color-text-secondary)]">
						{open ? <ChevronUp size={13} /> : <ChevronDown size={13} />}
					</button>
				</td>
				<td className="px-3 py-2.5 text-[13px] font-semibold text-[var(--color-text-primary)]">
					{pipeline.name}
				</td>
				<td className="px-3 py-2.5 text-[13px] text-[var(--color-text-secondary)]">
					{pipeline.namespace}
				</td>
				<td className="px-3 py-2.5">
					<span
						className="rounded-md px-2 py-0.5 text-[11px] font-semibold"
						style={{ background: statusStyle.bg, color: statusStyle.color }}
					>
						{statusStyle.label}
					</span>
				</td>
				<td className="px-3 py-2.5 text-[12px] text-[var(--color-text-secondary)]">
					{pipeline.lastDeployment
						? new Date(pipeline.lastDeployment.startedAt).toLocaleString("ko-KR")
						: "-"}
				</td>
				<td className="px-3 py-2.5">
					<div className="flex flex-wrap gap-1">
						{pipeline.k8sObjects.filter((obj) => obj.kind !== "Pod").map((obj) => (
							<K8sObjectBadge key={`${obj.kind}-${obj.name}`} kind={obj.kind} />
						))}
						{(() => {
							const pods = pipeline.k8sObjects.filter((o) => o.kind === "Pod");
							if (pods.length === 0) return null;
							const running = pods.filter((p) => p.status === "Running").length;
							const failed = pods.filter((p) => p.status !== "Running" && p.status !== "Pending").length;
							const pending = pods.filter((p) => p.status === "Pending").length;
							return (
								<div className="flex gap-1">
									{running > 0 && (
										<span className="rounded-md px-1.5 py-0.5 text-[10px] font-semibold bg-[rgba(34,197,94,0.15)] text-[#86efac]">
											Pod {running} Running
										</span>
									)}
									{pending > 0 && (
										<span className="rounded-md px-1.5 py-0.5 text-[10px] font-semibold bg-[rgba(245,158,11,0.15)] text-[#fbbf24]">
											Pod {pending} Pending
										</span>
									)}
									{failed > 0 && (
										<span className="flex items-center gap-0.5 rounded-md px-1.5 py-0.5 text-[10px] font-semibold bg-[rgba(239,68,68,0.15)] text-[#ef4444]">
											<AlertCircle size={10} /> Pod {failed} Failed
										</span>
									)}
								</div>
							);
						})()}
					</div>
				</td>
			</tr>
			{open && (
				<tr>
					<td colSpan={7} className="bg-[rgba(255,255,255,0.02)] p-0">
						<div className="px-6 py-3">
							<div className="grid grid-cols-[repeat(auto-fill,minmax(260px,1fr))] gap-2">
								{[...pipeline.k8sObjects].sort((a, b) => {
									const priority = (o: typeof a) => {
										if (o.kind !== "Pod") return 2;
										if (o.status !== "Running" && o.status !== "Pending") return 0;
										if (o.status === "Pending") return 1;
										return 3;
									};
									return priority(a) - priority(b);
								}).map((obj) => {
									const isPod = obj.kind === "Pod";
									const isFailed = isPod && obj.status !== "Running" && obj.status !== "Pending";
									const isPending = isPod && obj.status === "Pending";
									const podStatusColor = isPod
										? obj.status === "Running" ? "#22c55e"
										: isPending ? "#f59e0b" : "#ef4444"
										: undefined;
									const borderClass = isFailed
										? "border-[#ef4444]/40 bg-[rgba(239,68,68,0.05)]"
										: isPending
										? "border-[#f59e0b]/30 bg-[rgba(245,158,11,0.03)]"
										: "border-[var(--color-border-default)]";
									return (
										<div
											key={`${obj.kind}-${obj.name}`}
											className={`flex items-center justify-between rounded-lg border px-3 py-2 ${borderClass}`}
										>
											<div className="flex items-center gap-2">
												{isFailed && <AlertCircle size={13} className="text-[#ef4444] shrink-0" />}
												<K8sObjectBadge kind={obj.kind} status={obj.status} />
												<span className={`text-[12px] font-medium ${isFailed ? "text-[#ef4444]" : "text-[var(--color-text-primary)]"}`}>
													{obj.name}
												</span>
											</div>
											<div className="flex items-center gap-2 text-[11px] text-[var(--color-text-secondary)]">
												{isPod && podStatusColor && (
													<span className="flex items-center gap-1">
														<span className="inline-block h-2 w-2 rounded-full" style={{ background: podStatusColor }} />
														<span className={`font-semibold ${isFailed ? "text-[#ef4444]" : ""}`}>{obj.status}</span>
													</span>
												)}
												{isPod && obj.node && (
													<span className="flex items-center gap-0.5 text-[var(--color-text-muted)]">
														<Server size={10} /> {obj.node}
													</span>
												)}
												{obj.replicas != null && <span>Replicas: {obj.replicas}</span>}
												{obj.port != null && obj.port > 0 && <span>:{obj.port}</span>}
												{obj.host && <span className="truncate max-w-[140px]">{obj.host}</span>}
											</div>
										</div>
									);
								})}
							</div>
						</div>
					</td>
				</tr>
			)}
		</>
	);
}

function StackMonitoringTab({ stackId }: { stackId: string }) {
	const { data: workloads } = useStackWorkloads(stackId);

	const summary = workloads?.summary;
	const pipelineList = workloads?.pipelines ?? [];

	const totalPods = (summary?.runningPods ?? 0) + (summary?.pendingPods ?? 0) + (summary?.failedPods ?? 0);
	const podStatusData = useMemo(
		() => [
			{ name: "Running", value: summary?.runningPods ?? 0, color: "#22c55e" },
			{ name: "Pending", value: summary?.pendingPods ?? 0, color: "#f59e0b" },
			{ name: "Failed", value: summary?.failedPods ?? 0, color: "#ef4444" },
		],
		[summary],
	);

	const kpiCards = [
		{
			label: "Pipelines",
			value: String(summary?.totalPipelines ?? 0),
			icon: <GitBranch size={18} />,
			color: "#60a5fa",
			iconWrapClassName: "bg-[rgba(59,130,246,0.15)] text-[#60a5fa]",
			bar: Math.min(100, (summary?.totalPipelines ?? 0) * 20),
		},
		{
			label: "Deployments",
			value: String(summary?.totalDeployments ?? 0),
			icon: <Layers size={18} />,
			color: "#a78bfa",
			iconWrapClassName: "bg-[rgba(139,92,246,0.15)] text-[#a78bfa]",
			bar: Math.min(100, (summary?.totalDeployments ?? 0) * 20),
		},
		{
			label: "Running Pods",
			value: `${summary?.runningPods ?? 0} / ${totalPods}`,
			icon: <Box size={18} />,
			color: "#34d399",
			iconWrapClassName: "bg-[rgba(16,185,129,0.15)] text-[#34d399]",
			bar: totalPods > 0 ? Math.round(((summary?.runningPods ?? 0) / totalPods) * 100) : 0,
		},
		{
			label: "Failed Pods",
			value: String(summary?.failedPods ?? 0),
			icon: <XCircle size={18} />,
			color: summary?.failedPods ? "#ef4444" : "#34d399",
			iconWrapClassName: summary?.failedPods
				? "bg-[rgba(239,68,68,0.15)] text-[#ef4444]"
				: "bg-[rgba(16,185,129,0.15)] text-[#34d399]",
			bar: totalPods > 0 ? Math.round(((summary?.failedPods ?? 0) / totalPods) * 100) : 0,
		},
	];

	const cardClassName =
		"rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-[var(--card-padding)]";

	return (
		<div>
			<div className="mb-6 grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-4">
				{kpiCards.map((card) => (
					<div key={card.label} className={cardClassName}>
						<div className="mb-2.5 flex items-center gap-2.5">
							<div
								className={cn(
									"flex h-9 w-9 shrink-0 items-center justify-center rounded-lg",
									card.iconWrapClassName,
								)}
							>
								{card.icon}
							</div>
							<span className="text-xs font-medium text-[var(--color-text-secondary)]">
								{card.label}
							</span>
						</div>
						<div className="text-[28px] font-extrabold leading-none text-[var(--color-text-primary)]">
							{card.value}
						</div>
						<UsageBar value={card.bar} color={card.color} />
					</div>
				))}
			</div>

			{/* Deployed Pods Overview */}
			{pipelineList.length > 0 && (() => {
				const allPods = pipelineList.flatMap((p) =>
					p.k8sObjects
						.filter((o) => o.kind === "Pod")
						.map((pod) => ({ ...pod, appName: p.name, appNamespace: p.namespace, deployStatus: p.lastDeployment?.status ?? "unknown", deployedAt: p.lastDeployment?.startedAt, node: pod.node ?? "-" }))
				);
				const failedFirst = [...allPods].sort((a, b) => {
					const pri = (s: string | undefined) => s === "CrashLoopBackOff" || s === "Error" || s === "Failed" ? 0 : s === "Pending" ? 1 : 2;
					return pri(a.status) - pri(b.status);
				});
				return (
					<div className={cn(cardClassName, "mb-6")}>
						<h2 className="m-0 mb-4 text-[15px] font-bold text-[var(--color-text-primary)]">
							Deployed Pods ({allPods.length})
						</h2>
						<div className="overflow-x-auto">
							<table className="w-full text-left">
								<thead>
									<tr className="border-b border-[var(--color-border-default)]">
										<th className="px-3 py-2 text-[11px] font-semibold uppercase text-[var(--color-text-muted)]">Status</th>
										<th className="px-3 py-2 text-[11px] font-semibold uppercase text-[var(--color-text-muted)]">Pod Name</th>
										<th className="px-3 py-2 text-[11px] font-semibold uppercase text-[var(--color-text-muted)]">Node</th>
										<th className="px-3 py-2 text-[11px] font-semibold uppercase text-[var(--color-text-muted)]">App</th>
										<th className="px-3 py-2 text-[11px] font-semibold uppercase text-[var(--color-text-muted)]">Namespace</th>
										<th className="px-3 py-2 text-[11px] font-semibold uppercase text-[var(--color-text-muted)]">Deploy Status</th>
										<th className="px-3 py-2 text-[11px] font-semibold uppercase text-[var(--color-text-muted)]">Deployed At</th>
									</tr>
								</thead>
								<tbody>
									{failedFirst.map((pod) => {
										const isFailed = pod.status !== "Running" && pod.status !== "Pending";
										const isPending = pod.status === "Pending";
										const dotColor = isFailed ? "#ef4444" : isPending ? "#f59e0b" : "#22c55e";
										const rowBg = isFailed ? "bg-[rgba(239,68,68,0.04)]" : isPending ? "bg-[rgba(245,158,11,0.03)]" : "";
										return (
											<tr key={`${pod.appName}-${pod.name}`} className={`border-b border-[var(--color-border-default)] ${rowBg}`}>
												<td className="px-3 py-2">
													<span className="flex items-center gap-1.5">
														<span className="inline-block h-2.5 w-2.5 rounded-full" style={{ background: dotColor }} />
														<span className={`text-[12px] font-semibold ${isFailed ? "text-[#ef4444]" : isPending ? "text-[#fbbf24]" : "text-[#22c55e]"}`}>
															{pod.status}
														</span>
													</span>
												</td>
												<td className="px-3 py-2">
													<span className={`text-[12px] font-mono ${isFailed ? "text-[#ef4444] font-bold" : "text-[var(--color-text-primary)]"}`}>
														{pod.name}
													</span>
												</td>
												<td className="px-3 py-2">
													<span className="flex items-center gap-1 text-[12px] text-[var(--color-text-secondary)]">
														<Server size={11} className="shrink-0 text-[var(--color-text-muted)]" />
														{pod.node}
													</span>
												</td>
												<td className="px-3 py-2 text-[12px] text-[var(--color-text-secondary)]">{pod.appName}</td>
												<td className="px-3 py-2 text-[12px] text-[var(--color-text-secondary)]">{pod.appNamespace}</td>
												<td className="px-3 py-2">
													<K8sObjectBadge kind={pod.deployStatus === "success" ? "Service" : "Pod"} status={pod.deployStatus} />
												</td>
												<td className="px-3 py-2 text-[11px] text-[var(--color-text-secondary)]">
													{pod.deployedAt ? new Date(pod.deployedAt).toLocaleString("ko-KR") : "-"}
												</td>
											</tr>
										);
									})}
								</tbody>
							</table>
						</div>
					</div>
				);
			})()}

			{/* CI/CD Workloads Table */}
			{pipelineList.length > 0 && (
				<div className={cn(cardClassName, "mb-6")}>
					<h2 className="m-0 mb-4 text-[15px] font-bold text-[var(--color-text-primary)]">
						CI/CD Workloads
					</h2>
					<div className="overflow-x-auto">
						<table className="w-full text-left">
							<thead>
								<tr className="border-b border-[var(--color-border-default)]">
									<th className="w-8 px-3 py-2 text-[11px] font-semibold uppercase text-[var(--color-text-muted)]" />
									<th className="px-3 py-2 text-[11px] font-semibold uppercase text-[var(--color-text-muted)]">App</th>
									<th className="px-3 py-2 text-[11px] font-semibold uppercase text-[var(--color-text-muted)]">Namespace</th>
									<th className="px-3 py-2 text-[11px] font-semibold uppercase text-[var(--color-text-muted)]">Status</th>
									<th className="px-3 py-2 text-[11px] font-semibold uppercase text-[var(--color-text-muted)]">Last Deploy</th>
									<th className="px-3 py-2 text-[11px] font-semibold uppercase text-[var(--color-text-muted)]">K8s Objects</th>
								</tr>
							</thead>
							<tbody>
								{pipelineList.map((p) => (
									<WorkloadRow key={p.id} pipeline={p} />
								))}
							</tbody>
						</table>
					</div>
				</div>
			)}

			{/* Pod Status Chart - real data only */}
			{totalPods > 0 && (
				<div className={cn(cardClassName, "mb-6")}>
					<h2 className="m-0 mb-4 text-[15px] font-bold text-[var(--color-text-primary)]">
						Pod Status Overview
					</h2>
					<div className="grid grid-cols-1 gap-3.5 xl:grid-cols-2">
						<div className="rounded-[10px] border border-[var(--color-border-default)] bg-[#0b1220] p-2.5">
							<div className="mb-2 text-[13px] font-bold text-[#f8fafc]">
								Pod Status Distribution
							</div>
							<ResponsiveContainer width="100%" height={250}>
								<PieChart>
									<Pie
										data={podStatusData}
										dataKey="value"
										nameKey="name"
										cx="50%"
										cy="50%"
										outerRadius={86}
										label
									>
										{podStatusData.map((entry) => (
											<Cell key={entry.name} fill={entry.color} />
										))}
									</Pie>
									<Tooltip
										contentStyle={{
											background: "#111827",
											border: "1px solid #374151",
											color: "#e5e7eb",
										}}
									/>
									<Legend wrapperStyle={{ color: "#e5e7eb" }} />
								</PieChart>
							</ResponsiveContainer>
						</div>

						<div className="rounded-[10px] border border-[var(--color-border-default)] bg-[#0b1220] p-2.5">
							<div className="mb-2 text-[13px] font-bold text-[#f8fafc]">
								Pods per App
							</div>
							<ResponsiveContainer width="100%" height={250}>
								<BarChart data={pipelineList.map((p) => ({
									name: p.name,
									running: p.k8sObjects.filter((o) => o.kind === "Pod" && o.status === "Running").length,
									pending: p.k8sObjects.filter((o) => o.kind === "Pod" && o.status === "Pending").length,
									failed: p.k8sObjects.filter((o) => o.kind === "Pod" && o.status !== "Running" && o.status !== "Pending").length,
								}))}>
									<CartesianGrid stroke="rgba(148,163,184,0.2)" strokeDasharray="3 3" />
									<XAxis dataKey="name" stroke="#cbd5e1" tick={{ fill: "#cbd5e1", fontSize: 11 }} />
									<YAxis stroke="#cbd5e1" tick={{ fill: "#cbd5e1", fontSize: 11 }} allowDecimals={false} />
									<Tooltip contentStyle={{ background: "#111827", border: "1px solid #374151", color: "#e5e7eb" }} />
									<Legend wrapperStyle={{ color: "#e5e7eb" }} />
									<Bar dataKey="running" fill="#22c55e" radius={[5, 5, 0, 0]} name="Running" />
									<Bar dataKey="pending" fill="#f59e0b" radius={[5, 5, 0, 0]} name="Pending" />
									<Bar dataKey="failed" fill="#ef4444" radius={[5, 5, 0, 0]} name="Failed" />
								</BarChart>
							</ResponsiveContainer>
						</div>
					</div>

					<div className="mt-3 text-xs text-[var(--color-text-secondary)]">
						Total: {totalPods} pods — {summary?.runningPods ?? 0} running, {summary?.pendingPods ?? 0} pending, {summary?.failedPods ?? 0} failed
					</div>
				</div>
			)}
		</div>
	);
}

const HISTORY_ENTRIES = [
	{
		id: "deploy-v3-20260302",
		version: "v3 · Current",
		versionBg: "#059669",
		tools: "GitLab CI + Argo CD + Prometheus + Grafana",
		tag: "Tool Upgrade",
		tagBg: "rgba(245,158,11,0.15)",
		tagColor: "#fcd34d",
		borderColor: "#bbf7d0",
		reason: "Grafana v10.2 → v10.3 upgrade",
		duration: "42 min",
		result: "success",
		who: "admin@nullus.io",
		when: "2026-03-02 14:30",
		canRollback: false,
	},
	{
		id: "deploy-v2-20260228",
		version: "v2",
		versionBg: "#6366f1",
		tools: "GitLab CI + Argo CD + Prometheus + Grafana",
		tag: "Config Change",
		tagBg: "#e0f2fe",
		tagColor: "#0369a1",
		borderColor: "var(--color-border-default)",
		reason: "Storage: AWS S3 → MinIO",
		duration: "58 min",
		result: "success",
		who: "kim@nullus.io",
		when: "2026-02-28 09:15",
		canRollback: true,
	},
	{
		id: "deploy-v1-20260220",
		version: "v1 · Failed",
		versionBg: "#ef4444",
		tools: "GitLab CI + Argo CD + Prometheus",
		tag: "Initial Deploy",
		tagBg: "rgba(239,68,68,0.15)",
		tagColor: "#fca5a5",
		borderColor: "#fecaca",
		reason: "Initial stack deployment",
		duration: "12 min (aborted)",
		result: "failed",
		who: "admin@nullus.io",
		when: "2026-02-20 16:00",
		canRollback: false,
	},
];

function StackHistoryTab() {
	const navigate = useNavigate();
	return (
		<div>
			<div className="mb-4 flex items-center gap-3">
				<div className="h-5 w-1 rounded-full bg-[linear-gradient(135deg,#10b981,#059669)]" />
				<h3 className="m-0 text-[14px] font-bold text-[var(--color-text-primary)]">
					DevSecOps Stack History
				</h3>
			</div>
			<div className="flex flex-col gap-3">
				{HISTORY_ENTRIES.map((entry) => (
					<div
						key={entry.version}
						className="overflow-hidden rounded-lg border"
						style={{ borderColor: entry.borderColor }}
					>
						<div className="flex flex-wrap items-center justify-between gap-3 bg-[rgba(255,255,255,0.04)] px-5 py-3">
							<div className="flex flex-wrap items-center gap-2.5">
								<span
									className="rounded-full px-2.5 py-0.5 text-[12px] font-bold text-white"
									style={{ background: entry.versionBg }}
								>
									{entry.version}
								</span>
								<span className="text-[13px] font-semibold text-[var(--color-text-secondary)]">
									{entry.tools}
								</span>
								<span
									className="rounded-[8px] px-2 py-0.5 text-[11px] font-semibold"
									style={{ background: entry.tagBg, color: entry.tagColor }}
								>
									{entry.tag}
								</span>
							</div>
							<div className="text-[12px] text-[var(--color-text-secondary)]">
								👤 {entry.who} &nbsp;🕐 {entry.when}
							</div>
						</div>
						<div className="flex flex-wrap items-center justify-between gap-3 px-5 py-3">
							<div className="flex flex-wrap gap-5 text-[13px]">
								<span>
									<strong className="text-[var(--color-text-primary)]">
										Reason:
									</strong>{" "}
									<span className="text-[var(--color-text-secondary)]">
										{entry.reason}
									</span>
								</span>
								<span>
									<strong className="text-[var(--color-text-primary)]">
										Duration:
									</strong>{" "}
									<span className="text-[var(--color-text-secondary)]">
										{entry.duration}
									</span>
								</span>
								<span
									className="rounded-full px-2.5 py-0.5 text-[12px] font-semibold"
									style={
										entry.result === "success"
											? {
												background: "rgba(16,185,129,0.15)",
												color: "#6ee7b7",
											}
											: { background: "rgba(239,68,68,0.15)", color: "#fca5a5" }
									}
								>
									{entry.result === "success"
										? "✓ Success"
										: "✗ Failed · Auto Rolled Back"}
								</span>
							</div>
							<div className="flex gap-2">
								<button
									type="button"
									onClick={() => navigate(`/stack/logs/${entry.id}`)}
									className="flex items-center gap-1.5 rounded-md border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-2.5 py-1.5 text-[12px] text-[var(--color-text-primary)] transition-colors duration-150 hover:border-[rgba(99,102,241,0.4)] hover:bg-[rgba(99,102,241,0.08)] hover:text-[#a5b4fc]"
								>
									<Terminal size={12} /> Logs
								</button>
								{entry.canRollback && (
									<button
										type="button"
										className="flex items-center gap-1.5 rounded-md border border-[#6366f1] bg-[rgba(99,102,241,0.12)] px-2.5 py-1.5 text-[12px] text-[#a5b4fc]"
									>
										<RotateCcw size={12} /> Rollback to {entry.version}
									</button>
								)}
							</div>
						</div>
					</div>
				))}
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

const INNER_TABS: { key: InnerTab; label: string; icon: React.ReactNode }[] = [
	{ key: "info", label: "Info", icon: <Info size={13} /> },
	{ key: "monitoring", label: "Monitoring", icon: <BarChart2 size={13} /> },
	{ key: "history", label: "History", icon: <History size={13} /> },
	{
		key: "version-upgrade",
		label: "Version Upgrade",
		icon: <ArrowUpCircle size={13} />,
	},
];

export function StackDetailPanel({ stack }: { stack: Stack }) {
	const [innerTab, setInnerTab] = useState<InnerTab>("info");
	const statusStyle = STATUS_STYLES[stack.status] ?? STATUS_STYLES.pending;

	return (
		<div className="mt-2.5 overflow-hidden rounded-[var(--card-radius)] border border-[rgba(99,102,241,0.3)] bg-[var(--color-surface-card)]">
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
					{statusStyle.label}
				</span>
				<span className="text-[12px] text-[var(--color-text-secondary)]">
					· {stack.templateName} · {stack.clusterName}
				</span>
			</div>

			<div className="flex border-b border-[var(--color-border-default)]">
				{INNER_TABS.map((tab) => (
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
						{tab.icon} {tab.label}
					</button>
				))}
			</div>

			<div className="p-5">
				{innerTab === "info" && <StackInfoTab />}
				{innerTab === "monitoring" && <StackMonitoringTab stackId={stack.id} />}
				{innerTab === "history" && <StackHistoryTab />}
				{innerTab === "version-upgrade" && <StackVersionUpgradeTab />}
			</div>
		</div>
	);
}
