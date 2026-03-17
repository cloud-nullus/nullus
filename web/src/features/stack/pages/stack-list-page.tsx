import type { ColumnDef } from "@tanstack/react-table";
import {
	Archive,
	ArrowUpCircle,
	BarChart2,
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
	List,
	Monitor,
	Plus,
	RotateCcw,
	Search,
	Server,
	Terminal,
} from "lucide-react";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Breadcrumb } from "../../../components/shared/breadcrumb";
import { ConfirmDialog } from "../../../components/shared/confirm-dialog";
import { DataTable } from "../../../components/shared/data-table";
import { Button } from "../../../components/ui/button";
import { Input } from "../../../components/ui/input";
import { cn } from "../../../lib/utils";
import type { Stack } from "../api/stack-api";
import { useStacks } from "../api/stack-api";

type InnerTab = "info" | "monitoring" | "history" | "version-upgrade";

const STATUS_STYLES: Record<
	string,
	{ bg: string; color: string; label: string }
> = {
	running: { bg: "rgba(59,130,246,0.15)", color: "#60a5fa", label: "Running" },
	success: { bg: "rgba(34,197,94,0.15)", color: "#22c55e", label: "Success" },
	failed: { bg: "rgba(239,68,68,0.15)", color: "#ef4444", label: "Failed" },
	pending: { bg: "rgba(245,158,11,0.15)", color: "#f59e0b", label: "Pending" },
	cancelled: {
		bg: "rgba(100,116,139,0.15)",
		color: "#64748b",
		label: "Cancelled",
	},
};

function formatDate(iso: string) {
	return new Date(iso).toLocaleDateString("ko-KR", {
		year: "numeric",
		month: "2-digit",
		day: "2-digit",
	});
}

const MOCK_STACKS: Stack[] = [
	{
		id: "production-stack",
		name: "production-stack",
		templateId: "gitlab-all-in-one",
		templateName: "GitLab All-in-One",
		clusterId: "c1",
		clusterName: "prod-k8s",
		status: "success" as const,
		createdAt: "2026-01-10T00:00:00Z",
		updatedAt: "2026-03-03T14:28:00Z",
	},
	{
		id: "development-stack",
		name: "development-stack",
		templateId: "github-argocd",
		templateName: "GitHub + ArgoCD",
		clusterId: "c2",
		clusterName: "dev-k8s",
		status: "running" as const,
		createdAt: "2026-02-01T00:00:00Z",
		updatedAt: "2026-03-03T09:15:00Z",
	},
	{
		id: "staging-environment",
		name: "staging-environment",
		templateId: "gitlab-argocd",
		templateName: "GitLab + ArgoCD",
		clusterId: "c1",
		clusterName: "prod-k8s",
		status: "failed" as const,
		createdAt: "2026-02-15T00:00:00Z",
		updatedAt: "2026-03-02T18:45:00Z",
	},
	{
		id: "microservices-platform",
		name: "microservices-platform",
		templateId: "gitlab-all-in-one",
		templateName: "GitLab All-in-One",
		clusterId: "c3",
		clusterName: "staging-k8s",
		status: "success" as const,
		createdAt: "2026-01-25T00:00:00Z",
		updatedAt: "2026-03-01T11:20:00Z",
	},
];

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
					<select
						defaultValue={version}
						className="cursor-pointer rounded border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-2 py-1 text-[12px] text-[var(--color-text-primary)]"
					>
						{(versions ?? [version]).map((v) => (
							<option key={v}>{v}</option>
						))}
					</select>
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

const BAR_DATA = [
	{ h: 80, label: "00" },
	{ h: 45, label: "" },
	{ h: 30, label: "" },
	{ h: 60, label: "03" },
	{ h: 95, label: "" },
	{ h: 100, label: "" },
	{ h: 70, label: "06" },
	{ h: 88, label: "" },
	{ h: 75, label: "" },
	{ h: 92, label: "09" },
	{ h: 55, label: "" },
	{ h: 40, label: "" },
	{ h: 65, label: "12" },
	{ h: 85, label: "" },
	{ h: 50, label: "" },
	{ h: 72, label: "15" },
	{ h: 48, label: "" },
	{ h: 38, label: "17" },
];

function StackMonitoringTab() {
	const metrics = [
		{ label: "CPU Usage", value: "68%", num: 68, color: "#6366f1" },
		{ label: "Memory Usage", value: "42%", num: 42, color: "#10b981" },
		{ label: "Storage Usage", value: "31%", num: 31, color: "#f59e0b" },
		{ label: "Pipeline Success", value: "97.3%", num: 97.3, color: "#059669" },
	];
	const tools = [
		{ name: "GitLab", color: "#fc6d26" },
		{ name: "Argo CD", color: "#326ce5" },
		{ name: "Prometheus", color: "#e6522c" },
		{ name: "Grafana", color: "#f46800" },
		{ name: "Harbor", color: "#0f98c5" },
	];
	return (
		<div>
			<div className="mb-4">
				<select className="cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-2 text-sm text-[var(--color-text-primary)]">
					<option>Last 1 hour</option>
					<option>Last 24 hours</option>
					<option>Last 7 days</option>
				</select>
			</div>
			<div className="mb-6 grid grid-cols-2 gap-3 lg:grid-cols-4">
				{metrics.map((m) => (
					<div
						key={m.label}
						className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-4 text-center"
					>
						<div className="text-[26px] font-bold" style={{ color: m.color }}>
							{m.value}
						</div>
						<div className="mb-2.5 text-[12px] text-[var(--color-text-secondary)]">
							{m.label}
						</div>
						<div className="h-1.5 overflow-hidden rounded-full bg-[rgba(255,255,255,0.08)]">
							<div
								className="h-full rounded-full"
								style={{ width: `${m.num}%`, background: m.color }}
							/>
						</div>
					</div>
				))}
			</div>
			<div className="grid grid-cols-[2fr_1fr] gap-5">
				<div className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-5">
					<h3 className="mb-4 text-[14px] font-semibold text-[var(--color-text-primary)]">
						Pipeline Runs (Last 24h)
					</h3>
					<div className="flex h-24 items-end gap-1.5">
						{BAR_DATA.map((bar) => (
							<div
								key={`${bar.label}-${bar.h}`}
								className="flex flex-1 flex-col items-center gap-1"
							>
								<div
									className="w-full rounded-t"
									style={{
										height: `${bar.h}%`,
										background: bar.h === 70 ? "#ef4444" : "#6366f1",
									}}
								/>
								{bar.label && (
									<span className="text-[10px] text-[var(--color-text-secondary)]">
										{bar.label}
									</span>
								)}
							</div>
						))}
					</div>
				</div>
				<div className="rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.02)] p-5">
					<h3 className="mb-3 text-[14px] font-semibold text-[var(--color-text-primary)]">
						Stack Tools Status
					</h3>
					<div className="flex flex-col gap-2">
						{tools.map((t) => (
							<div
								key={t.name}
								className="flex items-center justify-between rounded-md bg-[rgba(255,255,255,0.04)] px-3 py-2 text-[12px]"
							>
								<span style={{ color: t.color }} className="font-medium">
									{t.name}
								</span>
								<span className="font-semibold text-[#6ee7b7]">✓ Running</span>
							</div>
						))}
					</div>
				</div>
			</div>
		</div>
	);
}

const HISTORY_ENTRIES = [
	{
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
	return (
		<div>
			<div className="mb-4 flex items-center gap-3">
				<div className="h-5 w-1 rounded-full bg-[linear-gradient(135deg,#10b981,#059669)]" />
				<h3 className="m-0 text-[14px] font-bold text-[var(--color-text-primary)]">
					DevSecOps Stack History
				</h3>
			</div>
			<div className="mb-4">
				<select className="cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-2 text-[13px] text-[var(--color-text-primary)]">
					<option>All Status</option>
					<option>Success</option>
					<option>Failed</option>
					<option>Rolled Back</option>
				</select>
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
									className="flex items-center gap-1.5 rounded-md border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-2.5 py-1.5 text-[12px] text-[var(--color-text-primary)]"
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
				{innerTab === "history" && <StackHistoryTab />}
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

	const { data: apiData, isLoading } = useStacks({
		search,
		status: statusFilter || undefined,
	});
	const stacks = apiData?.items ?? MOCK_STACKS;

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
		setDeleteStackId(null);
	};

	const columns: ColumnDef<Stack, unknown>[] = [
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
			cell: ({ row }) => {
				const isExpanded = expandedStackId === row.original.id;
				return (
					<div className="flex gap-1.5">
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
							{isExpanded ? "Close" : "Detail"}
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
				);
			},
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

			<div className="mb-4 flex flex-wrap gap-2.5">
				<div className="relative max-w-[320px] flex-[1_1_240px]">
					<Search
						size={13}
						className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--color-text-secondary)]"
					/>
					<Input
						placeholder="스택 검색..."
						value={search}
						onChange={(e) => setSearch(e.target.value)}
						className="pl-[30px]"
					/>
				</div>
				<select
					value={statusFilter}
					onChange={(e) => setStatusFilter(e.target.value)}
					className="cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[rgba(255,255,255,0.04)] px-3 py-[9px] text-sm text-[var(--color-text-primary)]"
				>
					<option value="">All Status</option>
					<option value="success">Success</option>
					<option value="running">Running</option>
					<option value="pending">Pending</option>
					<option value="failed">Failed</option>
					<option value="cancelled">Cancelled</option>
				</select>
			</div>

			<DataTable
				columns={columns}
				data={filtered}
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
