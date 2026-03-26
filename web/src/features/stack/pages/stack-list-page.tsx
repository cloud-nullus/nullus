import type { ColumnDef } from "@tanstack/react-table";
import {
	AlertCircle,
	Archive,
	ArrowUpCircle,
	BarChart2,
	Box,
	Boxes,
	Check,
	CheckCircle,
	ChevronDown,
	ChevronUp,
	ClipboardList,
	Cpu,
	FileText,
	GitBranch,
	HardDrive,
	History,
	Info,
	Layers,
	List,
	MemoryStick,
	Monitor,
	Plus,
	Search,
	Server,
	Terminal,
	XCircle,
} from "lucide-react";
import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
	Area,
	AreaChart,
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
import { Breadcrumb } from "../../../components/shared/breadcrumb";
import { ConfirmDialog } from "../../../components/shared/confirm-dialog";
import { DataTable } from "../../../components/shared/data-table";
import { Button } from "../../../components/ui/button";
import { NativeSelect } from "../../../components/ui/native-select";
import { cn } from "../../../lib/utils";
import type { Stack } from "../api/stack-api";
import { useDeleteStack, useStackHistory, useStacks } from "../api/stack-api";

type InnerTab = "info" | "monitoring" | "history" | "version-upgrade";

const STATUS_STYLES: Record<string, { bg: string; color: string; label: string }> = {
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

type MonitoringRange = "1h" | "6h" | "24h" | "7d";
type ToolHealthStatus = "running" | "warning" | "error";

const TOOL_STATUS_CONFIG: Record<
	ToolHealthStatus,
	{ icon: React.ReactNode; badgeClassName: string; label: string }
> = {
	running: {
		icon: <CheckCircle size={13} />,
		badgeClassName: "bg-[rgba(34,197,94,0.15)] text-[#22c55e]",
		label: "Running",
	},
	warning: {
		icon: <AlertCircle size={13} />,
		badgeClassName: "bg-[rgba(245,158,11,0.15)] text-[#f59e0b]",
		label: "Warning",
	},
	error: {
		icon: <XCircle size={13} />,
		badgeClassName: "bg-[rgba(239,68,68,0.15)] text-[#ef4444]",
		label: "Error",
	},
};

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

function generateMonitoringSeries(range: MonitoringRange) {
	const pointsByRange: Record<MonitoringRange, number> = {
		"1h": 6,
		"6h": 12,
		"24h": 24,
		"7d": 28,
	};
	const hoursByRange: Record<MonitoringRange, number> = {
		"1h": 1,
		"6h": 6,
		"24h": 24,
		"7d": 24 * 7,
	};

	const now = Date.now();
	const points = pointsByRange[range];
	const totalHours = hoursByRange[range];
	const hourStep = totalHours / points;

	return Array.from({ length: points }, (_, index) => {
		const ageHours = totalHours - hourStep * (index + 1);
		const ts = new Date(now - ageHours * 60 * 60 * 1000);
		const label =
			range === "7d"
				? ts.toLocaleDateString("en-US", { weekday: "short" })
				: ts.toLocaleTimeString("en-US", {
					hour: "2-digit",
					minute: "2-digit",
					hour12: false,
				});

		const cpuWave = 56 + Math.sin(index / 2.5) * 16 + (index % 3) * 2.1;
		const memoryWave = 63 + Math.cos(index / 3.2) * 10 + (index % 4) * 1.8;

		return {
			time: label,
			cpu: Math.max(12, Math.min(96, Math.round(cpuWave))),
			memory: Math.max(24, Math.min(97, Math.round(memoryWave))),
		};
	});
}

function StackMonitoringTab() {
	const [range, setRange] = useState<MonitoringRange>("24h");
	const usageData = useMemo(() => generateMonitoringSeries(range), [range]);

	const pipelineBars = useMemo(
		() => [
			{ day: "Mon", success: 16, failed: 2 },
			{ day: "Tue", success: 19, failed: 3 },
			{ day: "Wed", success: 15, failed: 4 },
			{ day: "Thu", success: 21, failed: 2 },
			{ day: "Fri", success: 24, failed: 3 },
			{ day: "Sat", success: 11, failed: 2 },
			{ day: "Sun", success: 9, failed: 1 },
		],
		[],
	);

	const podStatusData = useMemo(
		() => [
			{ name: "Running", value: 24, color: "#22c55e" },
			{ name: "Pending", value: 2, color: "#f59e0b" },
			{ name: "Failed", value: 1, color: "#ef4444" },
		],
		[],
	);

	const kpiCards = [
		{
			label: "CPU 사용률",
			value: "68%",
			icon: <Cpu size={18} />,
			color: "#60a5fa",
			iconWrapClassName: "bg-[rgba(59,130,246,0.15)] text-[#60a5fa]",
			bar: 68,
		},
		{
			label: "메모리 사용률",
			value: "42%",
			icon: <MemoryStick size={18} />,
			color: "#a78bfa",
			iconWrapClassName: "bg-[rgba(139,92,246,0.15)] text-[#a78bfa]",
			bar: 42,
		},
		{
			label: "스토리지",
			value: "31%",
			icon: <HardDrive size={18} />,
			color: "#34d399",
			iconWrapClassName: "bg-[rgba(16,185,129,0.15)] text-[#34d399]",
			bar: 31,
		},
		{
			label: "Pod 수",
			value: "24 / 27",
			icon: <Box size={18} />,
			color: "#fbbf24",
			iconWrapClassName: "bg-[rgba(245,158,11,0.15)] text-[#fbbf24]",
			bar: 89,
		},
	];

	const tools: { name: string; version: string; status: ToolHealthStatus }[] = [
		{ name: "GitLab", status: "running", version: "16.7" },
		{ name: "Argo CD", status: "running", version: "2.9.3" },
		{ name: "Prometheus", status: "running", version: "2.48.1" },
		{ name: "Grafana", status: "warning", version: "10.3" },
		{ name: "Harbor", status: "running", version: "2.8.2" },
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

			<div className={cn(cardClassName, "mb-6")}>
				<div className="mb-3.5 flex flex-wrap items-center justify-between gap-3">
					<h2 className="m-0 text-[15px] font-bold text-[var(--color-text-primary)]">
						Monitoring Charts
					</h2>
					<div className="flex gap-1.5">
						{(["1h", "6h", "24h", "7d"] as const).map((item) => {
							const active = range === item;
							return (
								<button
									key={item}
									type="button"
									onClick={() => setRange(item)}
									className={cn(
										"cursor-pointer rounded-[7px] border px-2.5 py-[5px] text-xs font-bold",
										active
											? "border-[rgba(245,158,11,0.6)] bg-[rgba(245,158,11,0.2)] text-[#fcd34d]"
											: "border-[var(--color-border-default)] bg-[rgba(255,255,255,0.03)] text-[var(--color-text-secondary)]",
									)}
								>
									{item}
								</button>
							);
						})}
					</div>
				</div>

				<div className="grid grid-cols-1 gap-3.5 xl:grid-cols-2">
					<div className="rounded-[10px] border border-[var(--color-border-default)] bg-[#0b1220] p-2.5">
						<div className="mb-2 text-[13px] font-bold text-[#f8fafc]">
							CPU Usage
						</div>
						<ResponsiveContainer width="100%" height={250}>
							<AreaChart data={usageData}>
								<defs>
									<linearGradient
										id="stackCpuGradient"
										x1="0"
										y1="0"
										x2="0"
										y2="1"
									>
										<stop offset="5%" stopColor="#f59e0b" stopOpacity={0.58} />
										<stop offset="95%" stopColor="#f59e0b" stopOpacity={0.06} />
									</linearGradient>
								</defs>
								<CartesianGrid
									stroke="rgba(148,163,184,0.2)"
									strokeDasharray="3 3"
								/>
								<XAxis
									dataKey="time"
									stroke="#cbd5e1"
									tick={{ fill: "#cbd5e1", fontSize: 11 }}
								/>
								<YAxis
									domain={[0, 100]}
									stroke="#cbd5e1"
									tick={{ fill: "#cbd5e1", fontSize: 11 }}
								/>
								<Tooltip
									contentStyle={{
										background: "#111827",
										border: "1px solid #374151",
										color: "#e5e7eb",
									}}
								/>
								<Legend wrapperStyle={{ color: "#e5e7eb" }} />
								<Area
									type="monotone"
									dataKey="cpu"
									stroke="#f59e0b"
									strokeWidth={2}
									fill="url(#stackCpuGradient)"
									name="CPU %"
								/>
							</AreaChart>
						</ResponsiveContainer>
					</div>

					<div className="rounded-[10px] border border-[var(--color-border-default)] bg-[#0b1220] p-2.5">
						<div className="mb-2 text-[13px] font-bold text-[#f8fafc]">
							Memory Usage
						</div>
						<ResponsiveContainer width="100%" height={250}>
							<AreaChart data={usageData}>
								<defs>
									<linearGradient
										id="stackMemoryGradient"
										x1="0"
										y1="0"
										x2="0"
										y2="1"
									>
										<stop offset="5%" stopColor="#3b82f6" stopOpacity={0.54} />
										<stop offset="95%" stopColor="#3b82f6" stopOpacity={0.08} />
									</linearGradient>
								</defs>
								<CartesianGrid
									stroke="rgba(148,163,184,0.2)"
									strokeDasharray="3 3"
								/>
								<XAxis
									dataKey="time"
									stroke="#cbd5e1"
									tick={{ fill: "#cbd5e1", fontSize: 11 }}
								/>
								<YAxis
									domain={[0, 100]}
									stroke="#cbd5e1"
									tick={{ fill: "#cbd5e1", fontSize: 11 }}
								/>
								<Tooltip
									contentStyle={{
										background: "#111827",
										border: "1px solid #374151",
										color: "#e5e7eb",
									}}
								/>
								<Legend wrapperStyle={{ color: "#e5e7eb" }} />
								<Area
									type="monotone"
									dataKey="memory"
									stroke="#3b82f6"
									strokeWidth={2}
									fill="url(#stackMemoryGradient)"
									name="Memory %"
								/>
							</AreaChart>
						</ResponsiveContainer>
					</div>

					<div className="rounded-[10px] border border-[var(--color-border-default)] bg-[#0b1220] p-2.5">
						<div className="mb-2 text-[13px] font-bold text-[#f8fafc]">
							Pipeline Success Rate
						</div>
						<ResponsiveContainer width="100%" height={250}>
							<BarChart data={pipelineBars}>
								<CartesianGrid
									stroke="rgba(148,163,184,0.2)"
									strokeDasharray="3 3"
								/>
								<XAxis
									dataKey="day"
									stroke="#cbd5e1"
									tick={{ fill: "#cbd5e1", fontSize: 11 }}
								/>
								<YAxis
									stroke="#cbd5e1"
									tick={{ fill: "#cbd5e1", fontSize: 11 }}
								/>
								<Tooltip
									contentStyle={{
										background: "#111827",
										border: "1px solid #374151",
										color: "#e5e7eb",
									}}
								/>
								<Legend wrapperStyle={{ color: "#e5e7eb" }} />
								<Bar dataKey="success" fill="#22c55e" radius={[5, 5, 0, 0]} />
								<Bar dataKey="failed" fill="#ef4444" radius={[5, 5, 0, 0]} />
							</BarChart>
						</ResponsiveContainer>
					</div>

					<div className="rounded-[10px] border border-[var(--color-border-default)] bg-[#0b1220] p-2.5">
						<div className="mb-2 text-[13px] font-bold text-[#f8fafc]">
							Pod Status
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
				</div>

				<div className="mt-3 text-xs text-[var(--color-text-secondary)]">
					Pipeline summary: 97.3% success, 145 total runs, average build 2m 34s.
				</div>
			</div>

			<div className={cardClassName}>
				<h2 className="m-0 mb-4 text-[15px] font-bold text-[var(--color-text-primary)]">
					Tool Health
				</h2>
				<div className="grid grid-cols-[repeat(auto-fill,minmax(200px,1fr))] gap-3">
					{tools.map((tool) => {
						const cfg = TOOL_STATUS_CONFIG[tool.status];
						return (
							<div
								key={tool.name}
								className="rounded-[10px] border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-3.5"
							>
								<div className="mb-1.5 flex items-center justify-between">
									<span className="text-sm font-bold text-[var(--color-text-primary)]">
										{tool.name}
									</span>
									<span
										className={cn(
											"inline-flex items-center gap-1 rounded-[5px] px-2 py-0.5 text-[11px] font-semibold",
											cfg.badgeClassName,
										)}
									>
										{cfg.icon}
										{cfg.label}
									</span>
								</div>
								<div className="text-xs text-[var(--color-text-secondary)]">
									v{tool.version}
								</div>
							</div>
						);
					})}
				</div>
			</div>
		</div>
	);
}

function StackHistoryTab({ stack }: { stack: Stack }) {
	const navigate = useNavigate();
	const { data: historyData, isLoading } = useStackHistory(stack.id);
	const entries = Array.isArray(historyData) ? historyData : [];
	const latestEntryID = entries[entries.length - 1]?.id;

	return (
		<div>
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
			<div className="flex flex-col gap-3">
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

function StackDetailPanel({ stack }: { stack: Stack }) {
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
				{innerTab === "monitoring" && <StackMonitoringTab />}
				{innerTab === "history" && <StackHistoryTab stack={stack} />}
				{innerTab === "version-upgrade" && <StackVersionUpgradeTab />}
			</div>
		</div>
	);
}

export function StackListPage() {
	const navigate = useNavigate();
	const [search, setSearch] = useState("");
	const [statusFilter, setStatusFilter] = useState("");
	const [expandedStackId, setExpandedStackId] = useState<string | null>(null);
	const [deleteStackId, setDeleteStackId] = useState<string | null>(null);
	const deleteStack = useDeleteStack();

	const { data: apiData, isLoading } = useStacks({
		search,
		status: statusFilter || undefined,
	});
	const stacks = apiData?.items ?? [];

	const filtered = stacks.filter((s) => {
		const q = search.toLowerCase();
		const matchesSearch =
			!search ||
			s.name.toLowerCase().includes(q) ||
			s.templateName.toLowerCase().includes(q) ||
			s.clusterName.toLowerCase().includes(q);
		const matchesStatus = !statusFilter || s.status === statusFilter;
		return matchesSearch && matchesStatus;
	});
	const expandedStack = filtered.find((s) => s.id === expandedStackId) ?? null;

	const handleDeleteStack = () => {
		if (!deleteStackId) return;
		deleteStack.mutate(deleteStackId, {
			onSuccess: () => setDeleteStackId(null),
		});
	};

	const columns: ColumnDef<Stack, unknown>[] = [
		{
			id: "expand",
			header: "",
			enableSorting: false,
			cell: ({ row }) => {
				const isExpanded = expandedStackId === row.original.id;
				return (
					<Button
						variant={isExpanded ? "secondary" : "ghost"}
						size="sm"
						type="button"
						onClick={(e) => {
							e.stopPropagation();
							setExpandedStackId((prev) =>
								prev === row.original.id ? null : row.original.id,
							);
						}}
					>
						{isExpanded ? <ChevronUp size={13} /> : <ChevronDown size={13} />}
					</Button>
				);
			},
		},
		{
			accessorKey: "name",
			header: "스택 이름",
			cell: ({ row }) => (
				<div className="flex items-center gap-2">
					{expandedStackId === row.original.id && (
						<div className="h-1.5 w-1.5 shrink-0 rounded-full bg-[#6366f1]" />
					)}
					<span className="font-semibold">{row.original.name}</span>
				</div>
			),
		},
		{
			accessorKey: "templateName",
			header: "템플릿",
			cell: ({ row }) => (
				<span className="text-[var(--color-text-secondary)]">
					{row.original.templateName}
				</span>
			),
		},
		{
			accessorKey: "clusterName",
			header: "클러스터",
			cell: ({ row }) => (
				<span className="text-[var(--color-text-secondary)]">
					{row.original.clusterName}
				</span>
			),
		},
		{
			accessorKey: "status",
			header: "상태",
			cell: ({ row }) => {
				const s = STATUS_STYLES[row.original.status] ?? STATUS_STYLES.pending;
				return (
					<span
						className="rounded-md px-[9px] py-[3px] text-xs font-semibold"
						style={{ backgroundColor: s.bg, color: s.color }}
					>
						{s.label}
					</span>
				);
			},
		},
		{
			accessorKey: "createdAt",
			header: "생성일",
			cell: ({ row }) => (
				<span className="text-[13px] text-[var(--color-text-secondary)]">
					{formatDate(row.original.createdAt)}
				</span>
			),
		},
		{
			id: "actions",
			header: "Actions",
			enableSorting: false,
			cell: ({ row }) => (
				<div className="flex gap-2">
					<Button
						variant="outline"
						size="sm"
						type="button"
						onClick={(e) => {
							e.stopPropagation();
							navigate(`/stack/${row.original.id}/add-tools`);
						}}
					>
						<Plus size={13} /> Add Tools
					</Button>
					<Button
						variant="danger"
						size="sm"
						type="button"
						onClick={(e) => {
							e.stopPropagation();
							setDeleteStackId(row.original.id);
						}}
					>
						Delete
					</Button>
				</div>
			),
		},
	];

	return (
		<div>
			<Breadcrumb items={[{ label: "Stack List" }]} />

			<div className="mb-6 flex items-start justify-between">
				<div className="flex items-center gap-2.5">
					<div className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(99,102,241,0.15)] text-[#818cf8]">
						<List size={18} />
					</div>
					<div>
						<h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
							Stack List
						</h1>
						<p className="m-0 mt-0.5 text-[13px] text-[var(--color-text-secondary)]">
							배포된 DevSecOps 스택 목록
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
					New Stack
				</Button>
			</div>

			<DataTable
				columns={columns}
				data={filtered}
				toolbar={
					<>
						<NativeSelect
							value={statusFilter}
							onChange={(e) => setStatusFilter(e.target.value)}
							className="cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)] [&>option]:bg-[var(--color-surface-base)] [&>option]:text-[var(--color-text-primary)]"
						>
							<option value="" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">All Status</option>
							<option value="success" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Success</option>
							<option value="running" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Running</option>
							<option value="pending" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Pending</option>
							<option value="failed" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Failed</option>
							<option value="cancelled" className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">Cancelled</option>
						</NativeSelect>
						<div className="relative ml-auto">
							<Search
								size={13}
								className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--color-text-secondary)]"
							/>
							<input
								placeholder="스택 검색..."
								value={search}
								onChange={(e) => setSearch(e.target.value)}
								className="w-[220px] rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] py-[7px] pl-[30px] pr-3 text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)]"
							/>
						</div>
					</>
				}
				getRowKey={(row) => row.id}
				onRowClick={(row) =>
					setExpandedStackId((prev) => (prev === row.id ? null : row.id))
				}
				emptyMessage={isLoading ? "스택을 불러오는 중..." : "스택이 없습니다."}
			/>

			{expandedStack && (
				<StackDetailPanel key={expandedStack.id} stack={expandedStack} />
			)}

			<ConfirmDialog
				open={deleteStackId !== null}
				onClose={() => setDeleteStackId(null)}
				onConfirm={handleDeleteStack}
				title="Delete Stack"
				description="이 스택을 삭제하면 관련 배포 정보가 영향을 받을 수 있습니다. 계속하시겠습니까?"
				confirmLabel="Delete"
			/>
		</div>
	);
}
