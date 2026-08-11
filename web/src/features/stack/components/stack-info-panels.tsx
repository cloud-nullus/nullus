import React from "react"
import { useTranslation } from 'react-i18next'
import { Archive, ArrowUpCircle, BarChart2, Boxes, FileText, GitBranch, Monitor, Server, Check } from "lucide-react"
import { NativeSelect } from "../../../components/ui/native-select"
import { cn } from "../../../lib/utils"

export function ConfigCard({
	title,
	icon,
	children,
}: {
	title: string;
	icon: React.ReactNode;
	children: React.ReactNode;
}) {
	return (
		<div className="rounded-lg border border-[var(--color-border-default)] bg-[color-mix(in srgb, var(--color-text-primary) 2%, transparent)]">
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

export function ToolOption({
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
					? "border-[color-mix(in srgb, var(--color-primary) 35%, transparent)]"
					: "border-[var(--color-border-default)] opacity-60",
			)}
		>
			<div className="flex items-start gap-2">
				<div
					className={cn(
						"mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded border",
						checked
							? "border-[var(--color-primary)] bg-[var(--color-primary)]"
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
						className="cursor-pointer rounded border border-[var(--color-border-default)] bg-[color-mix(in srgb, var(--color-text-primary) 4%, transparent)] px-2 py-1 text-[12px] text-[var(--color-text-primary)] [&>option]:bg-[var(--color-surface-base)] [&>option]:text-[var(--color-text-primary)]"
					>
						{(versions ?? [version]).map((v) => (
							<option key={v} className="bg-[var(--color-surface-base)] text-[var(--color-text-primary)]">{v}</option>
						))}
					</NativeSelect>
					<div className="flex items-center gap-1.5">
						<span className="text-[11px] text-[var(--color-primary)]">Instances:</span>
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

export function PanelHeader({ title, desc }: { title: string; desc: string }) {
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

export function ArtifactsPanel() {
	const { t } = useTranslation()
	return (
		<div>
			<PanelHeader
				title="Artifact Configuration"
				desc={t('stackInfo.panels.artifacts', 'Artifact repositories configured for this stack')}
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

export function PipelineToolsPanel() {
	const { t } = useTranslation()
	return (
		<div>
			<PanelHeader
				title="Pipeline Tools"
				desc={t('stackInfo.panels.pipeline', 'CI/CD pipeline tools configured for this stack')}
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

export function MonitoringToolsPanel() {
	const { t } = useTranslation()
	return (
		<div>
			<PanelHeader
				title="Monitoring Tools"
				desc={t('stackInfo.panels.monitoring', 'Monitoring tools configured for this stack')}
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

export function LoggingToolsPanel() {
	const { t } = useTranslation()
	return (
		<div>
			<PanelHeader title="Logging Tools" desc={t('stackInfo.panels.logging', 'Logging tools configured for this stack')} />
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

export function ResourcesPanel() {
	const { t } = useTranslation()
	return (
		<div>
			<PanelHeader title="Resources" desc={t('stackInfo.panels.resources', 'Resource allocation for this stack')} />
			<div className="mb-6 grid grid-cols-3 gap-4">
				{[
					{ label: "CPU", value: "8", unit: "cores", color: "var(--color-primary)" },
					{ label: "Memory", value: "32", unit: "Gi", color: "var(--color-success)" },
					{ label: "Storage", value: "500", unit: "Gi", color: "var(--color-warning)" },
				].map((item) => (
					<div
						key={item.label}
						className="flex flex-col items-center rounded-lg border border-[var(--color-border-default)] bg-[color-mix(in srgb, var(--color-text-primary) 2%, transparent)] p-5"
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
							{t('stackInfo.allocated', 'Allocated {{label}}', { label: item.label })}
						</div>
					</div>
				))}
			</div>
			<div className="rounded-lg border border-[var(--color-border-default)] bg-[color-mix(in srgb, var(--color-text-primary) 2%, transparent)] p-4">
				<h4 className="mb-3 flex items-center gap-1.5 text-[13px] font-semibold text-[var(--color-text-primary)]">
					<Server size={14} /> {t('stackInfo.clusterInfo', 'Cluster Info')}
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
