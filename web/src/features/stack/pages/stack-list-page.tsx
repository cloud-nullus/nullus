import type { ColumnDef } from "@tanstack/react-table";
import {
	ArrowUpCircle,
	BarChart2,
	ClipboardList,
	GitBranch,
	History,
	Info,
	Layers,
	List,
	Plus,
	Search,
	Terminal,
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { Breadcrumb } from "../../../components/shared/breadcrumb";
import { ConfirmDialog } from "../../../components/shared/confirm-dialog";
import { DataTable } from "../../../components/shared/data-table";
import { Button } from "../../../components/ui/button";
import { NativeSelect } from "../../../components/ui/native-select";
import { cn } from "../../../lib/utils";
import { StackMonitoringOverview } from "../../observability/components/stack-monitoring-overview";
import type { Stack } from "../api/stack-api";
import { useDeleteStack, useStackHistory, useStacks } from "../api/stack-api";
import { useScopedClusters } from "../../admin/api/admin-api";
import { STATUS_STYLES } from "../utils/status-style";
import { useKeyboardShortcut } from "../../../hooks/use-keyboard-shortcut";
import {
	formatDate,
	getStackStatusLabel,
	isHealthyStatus,
	matchesStackStatusFilter,
	normalizeStackStatus,
} from "../utils/stack-list-utils";

export type {
	LaunchTool,
	StorageConnectionInfo,
	StackConnectionInfo,
} from "../utils/stack-list-utils";
export {
	extractConnectionInfo,
	buildOssLoginHint,
	buildConnectionInfoText,
} from "../utils/stack-list-utils";
import { StackInfoTab } from "../components/stack-info-tab"

type InnerTab = "info" | "monitoring" | "history" | "version-upgrade";


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
	const clusters = useMemo(() => clustersData?.items ?? [], [clustersData]);
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
