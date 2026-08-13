import type { ColumnDef } from "@tanstack/react-table";
import { ArrowUpCircle, Boxes, ChartColumn, ClipboardList, GitBranch, History, Info, Layers, List, Plus, SlidersHorizontal, Terminal } from 'lucide-react';
import { iconProps } from '../../../components/ui/icon'
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { ConfirmDialog } from "../../../components/shared/confirm-dialog";
import { DataTable } from "../../../components/shared/data-table";
import { Tabs } from "../../../components/ui/tabs";
import { ListDetailPanel } from "../../../components/shared/list-detail-panel";
// stack-list-utils 에도 formatDate 가 있지만 로케일이 ko-KR 로 박혀 있다.
// 표의 생성 시각은 화면 언어를 따라야 하므로 lib/locale 쪽을 쓴다.
import { formatDate as localeDate, formatTime as localeTime } from "../../../lib/locale";
import { Button } from "../../../components/ui/button";
import { Modal } from "../../../components/ui/modal";
import { Select } from "../../../components/ui/select";
import { cn } from "../../../lib/utils";
import { StackMonitoringOverview } from "../../observability/components/stack-monitoring-overview";
import type { Stack } from "../api/stack-api";
import {
	useDeleteStack,
	useImportStackConfig,
	usePreviewImportStackConfig,
	useStackHistory,
	useStacks,
} from "../api/stack-api";
import { useScopedClusters } from "../../admin/api/admin-api";
import { STATUS_STYLES } from "../utils/status-style";
import { summarizeImportPreview } from "../utils/import-preview";
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
	buildConnectionInfoText,
	buildOssLoginHint,
	findToolCredential,
	toConnectionInfoView,
} from "../utils/stack-list-utils";
import { StackInfoTab } from "../components/stack-info-tab"
import { StackWorkloadsTab } from "../components/stack-workloads-tab"
import { StackConfigTab } from "../components/stack-config-tab"
import { PageHeader } from '../../../components/layout/page-header'
import { SearchInput } from '../../../components/ui/search-input'
import { Badge } from "../../../components/ui/badge"
import { TOOL_BRAND_GRADIENT } from "../../../lib/tool-brand-colors";

type InnerTab = "info" | "workloads" | "config" | "monitoring" | "history" | "version-upgrade";


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
					<div className="h-5 w-1 rounded-full bg-[linear-gradient(135deg,var(--color-success),var(--color-success))]" />
					<h3 className="m-0 text-[14px] font-bold text-[var(--color-text-primary)]">{stack.name} History</h3>
				</div>
				<div className="flex items-center gap-2">
					<Button variant="outline" size="sm" onClick={() => navigate(`/stack/logs/${stack.id}`)} type="button">
						<Terminal {...iconProps('xs')} /> Open Logs
					</Button>
					<Button variant="outline" size="sm" onClick={() => navigate(`/stack/history/${stack.id}`)} type="button">
						Open Full History
					</Button>
					<Button
						variant="outline"
						size="sm"
						onClick={() => navigate(`/stack/deployments/${stack.id}/retry-history`)}
						type="button"
					>
						Retry History
					</Button>
				</div>
			</div>
			{isLoading && (
				<div className="mb-3 rounded-lg border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] px-4 py-3 text-[13px] text-[var(--color-text-secondary)]">
					Loading history...
				</div>
			)}
			{!isLoading && entries.length === 0 && (
				<div className="mb-3 rounded-lg border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] px-4 py-3 text-[13px] text-[var(--color-text-secondary)]">
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
						style={{ borderColor: isCurrent ? "var(--color-success)" : "var(--color-border-default)" }}
					>
						<div className="flex flex-wrap items-center justify-between gap-3 bg-[color-mix(in_srgb,_var(--color-text-primary)_4%,_transparent)] px-5 py-3">
							<div className="flex flex-wrap items-center gap-2.5">
								<span
									className="rounded-full px-2.5 py-0.5 text-[12px] font-bold text-white"
									style={{
										background: isCurrent
											? "var(--color-success)"
											: "var(--color-primary)",
									}}
								>
									v{entry.version}{isCurrent ? " · Current" : ""}
								</span>
								<span
									className="rounded-[8px] px-2 py-0.5 text-[11px] font-semibold"
									style={{
										background: isCurrent ? "color-mix(in srgb, var(--color-warning) 15%, transparent)" : "color-mix(in srgb, var(--color-primary) 15%, transparent)",
										color: isCurrent ? "var(--color-warning)" : "var(--color-primary)",
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
		iconBg: TOOL_BRAND_GRADIENT.gitlab,
		current: "v16.7",
		latest: "v16.9",
		tag: "Minor Update",
		tagBg: "color-mix(in srgb, var(--color-warning) 15%, transparent)",
		tagColor: "var(--color-warning)",
		upToDate: false,
	},
	{
		name: "Prometheus",
		iconBg: TOOL_BRAND_GRADIENT.nexus,
		current: "v2.48.1",
		latest: "v2.50.1",
		tag: "Patch Update",
		tagBg: "color-mix(in srgb, var(--color-success) 15%, transparent)",
		tagColor: "var(--color-success)",
		upToDate: false,
	},
	{
		name: "Grafana",
		iconBg: TOOL_BRAND_GRADIENT.argocd,
		current: "v10.3",
		latest: "v10.4",
		tag: "Minor Update",
		tagBg: "color-mix(in srgb, var(--color-warning) 15%, transparent)",
		tagColor: "var(--color-warning)",
		upToDate: false,
	},
	{
		name: "Argo CD",
		iconBg: TOOL_BRAND_GRADIENT.kubernetes,
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
				<div className="h-5 w-1 rounded-full bg-[linear-gradient(135deg,var(--color-primary),var(--color-accent-alt))]" />
				<h3 className="m-0 text-[14px] font-bold text-[var(--color-text-primary)]">
					Available Version Upgrades
				</h3>
				<span className="rounded-full bg-[color-mix(in_srgb,_var(--color-primary)_15%,_transparent)] px-2.5 py-0.5 text-[12px] font-semibold text-[var(--color-primary)]">
					3 updates available
				</span>
			</div>
			<div className="flex flex-col gap-3">
				{UPGRADE_ITEMS.map((item) => (
					<div
						key={item.name}
						className={cn(
							"flex flex-wrap items-center justify-between gap-3 rounded-lg border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] p-4",
							item.upToDate && "opacity-60",
						)}
					>
						<div className="flex items-center gap-3">
							<div
								className="flex h-9 w-9 items-center justify-center rounded-lg"
								style={{ background: item.iconBg }}
							>
								<GitBranch {...iconProps('sm')} className="text-white" />
							</div>
							<div>
								<div className="font-semibold text-[var(--color-text-primary)]">
									{item.name}
								</div>
								{item.upToDate ? (
									<div className="text-[12px] text-[var(--color-text-secondary)]">
										Current: {item.current} →{" "}
										<strong className="text-[var(--color-success)]">Up to date</strong>
									</div>
								) : (
									<div className="text-[12px] text-[var(--color-text-secondary)]">
										Current: {item.current} → Latest:{" "}
										<strong className="text-[var(--color-success)]">{item.latest}</strong>
									</div>
								)}
							</div>
						</div>
						<div className="flex items-center gap-2.5">
							{item.upToDate ? (
								<span className="rounded-full bg-[color-mix(in_srgb,_var(--color-success)_15%,_transparent)] px-2.5 py-0.5 text-[11px] font-semibold text-[var(--color-success)]">
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
										className="flex items-center gap-1.5 rounded-md border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_4%,_transparent)] px-2.5 py-1.5 text-[12px] text-[var(--color-text-primary)]"
									>
										<ClipboardList {...iconProps('xs')} /> Changelog
									</button>
									<button
										type="button"
										onClick={handleUpgradeClick}
										className="flex items-center gap-1.5 rounded-md bg-[linear-gradient(135deg,var(--color-primary),var(--color-accent-alt))] px-2.5 py-1.5 text-[12px] font-semibold text-white"
									>
										<ArrowUpCircle {...iconProps('xs')} /> Upgrade
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
	{ key: "info", label: "Info", icon: <Info {...iconProps('xs')} /> },
	// 클러스터에 실제로 뜬 파드. 예전에는 Info 탭이 "Monitoring / History 탭에서
	// 확인하세요" 라고 안내했지만 두 탭 어디에도 파드 목록은 없었다.
	{ key: "workloads", label: "Workloads", icon: <Boxes {...iconProps('xs')} /> },
	// 배포된 OSS 의 values.yaml 을 직접 고치는 자리. 파드를 본 다음에 오는 것이
	// 맞다 — 무엇이 떠 있는지 확인하고 나서 그 설정을 손대게 된다.
	{ key: "config", label: "Config", icon: <SlidersHorizontal {...iconProps('xs')} /> },
	{ key: "history", label: "History", icon: <History {...iconProps('xs')} /> },
	{
		key: "version-upgrade",
		label: "Version Upgrade",
		icon: <ArrowUpCircle {...iconProps('xs')} />,
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
	flush = false,
}: {
	stack: Stack;
	clusterConnectionStatus?: string;
	isDeleting: boolean;
	onAddTools: () => void;
	onDelete: () => void;
	onBackToList: () => void;
	className?: string;
	/** ListDetailPanel 안에 들어갈 때 자기 테두리·모서리를 버린다. */
	flush?: boolean;
}) {
	const { t } = useTranslation();
	const [innerTab, setInnerTab] = useState<InnerTab>("info");
	const normalizedStatus = normalizeStackStatus(stack.status, clusterConnectionStatus);
	const canShowMonitoring = isHealthyStatus(stack.status, clusterConnectionStatus);
	// Info → Workloads(파드) → Monitoring(집계) 순. 구체적인 것에서 요약으로 간다.
	const innerTabs = canShowMonitoring
		? [
				...BASE_INNER_TABS.slice(0, 2),
				{ key: "monitoring" as const, label: "Monitoring", icon: <ChartColumn {...iconProps('xs')} /> },
				...BASE_INNER_TABS.slice(2),
			]
		: BASE_INNER_TABS;
	const statusStyle = STATUS_STYLES[normalizedStatus] ?? STATUS_STYLES.pending;

	useEffect(() => {
		if (!canShowMonitoring && innerTab === "monitoring") {
			setInnerTab("info");
		}
	}, [canShowMonitoring, innerTab]);

	return (
		<div
			className={cn(
				"flex h-full flex-col overflow-hidden bg-[var(--color-surface-card)]",
				flush
					? ""
					: "rounded-[var(--card-radius)] border border-[color-mix(in_srgb,_var(--color-primary)_30%,_transparent)]",
				className,
			)}
		>
			<div className="flex items-center gap-3 border-b border-[var(--color-border-default)] px-5 py-3.5">
				<div className="flex h-8 w-8 items-center justify-center rounded-lg bg-[color-mix(in_srgb,_var(--color-primary)_15%,_transparent)] text-[var(--color-primary)]">
					<Layers {...iconProps('sm')} />
				</div>
				<h3 className="m-0 text-[15px] font-bold text-[var(--color-text-primary)]">
					{stack.name}
				</h3>
				<Badge
					className=""
					style={{ background: statusStyle.bg, color: statusStyle.color }}>
					{getStackStatusLabel(t, normalizedStatus)}
				</Badge>
				<span className="text-[12px] text-[var(--color-text-secondary)]">
					· {stack.templateName} · {stack.clusterName}
				</span>
			</div>

			<Tabs
				value={innerTab}
				onChange={setInnerTab}
				items={innerTabs.map((tab) => ({
					id: tab.key,
					icon: tab.icon,
					label: t(`stackList.tabs.${tab.key}`, tab.label),
				}))}
			/>

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
				{innerTab === "workloads" && <StackWorkloadsTab stackId={stack.id} />}
				{innerTab === "config" && <StackConfigTab stackId={stack.id} />}
				{innerTab === "monitoring" && canShowMonitoring && <StackMonitoringTab stackId={stack.id} />}
				{innerTab === "history" && <StackHistoryTab stack={stack} />}
				{innerTab === "version-upgrade" && <StackVersionUpgradeTab />}
			</div>
		</div>
	);
}

export function StackListPage() {
	const { t, i18n } = useTranslation();
	const navigate = useNavigate();
	// F8-UIUX-KeyboardHints — jump straight to the install wizard.
	useKeyboardShortcut("n", () => navigate("/stack/install"));
	const [search, setSearch] = useState("");
	const [statusFilter, setStatusFilter] = useState("");
	const [clusterFilter, setClusterFilter] = useState("");
	const [expandedStackId, setExpandedStackId] = useState<string | null>(null);
  const [deleteStackId, setDeleteStackId] = useState<string | null>(null);
  const [importOpen, setImportOpen] = useState(false);
  const [importFile, setImportFile] = useState<File | null>(null);
  const [importFileError, setImportFileError] = useState("");
  const [importPayload, setImportPayload] = useState("");
  const [importPreview, setImportPreview] = useState<null | {
    mode: "create" | "update";
    name: string;
    cluster_id: string;
    existing_stack_id?: string;
    existing_state?: string;
    changes?: {
      added: Record<string, unknown>;
      removed: Record<string, unknown>;
      changed: Record<string, [unknown, unknown]>;
    };
  }>(null);
  const [terminatingStatusByID, setTerminatingStatusByID] = useState<Record<string, true>>({});
	const [viewportHeight, setViewportHeight] = useState(() =>
		typeof window !== "undefined" ? window.innerHeight : 960,
	);
	const [viewportWidth, setViewportWidth] = useState(() =>
		typeof window !== "undefined" ? window.innerWidth : 1440,
	);
  const deleteStack = useDeleteStack();
  const importStack = useImportStackConfig();
  const previewImportStack = usePreviewImportStackConfig();
	const tablePageSize = Math.max(6, Math.min(14, Math.floor((viewportHeight - 340) / 52)));
	const isDesktopLayout = viewportWidth >= 1280;
	// 288 = 사이드바 240 + 페이지 좌우 여백 2×24.
	// 38% → 30%. 상세 쪽 파이프라인 토폴로지가 스테이지를 역할별로 나누면서
	// 가로로 길어졌다 — 목록은 네 컬럼(이름·클러스터·상태·생성)만 있으면 되고,
	// 남는 폭은 토폴로지가 잘리지 않는 데 쓰는 편이 낫다.
	const listPaneWidth = Math.max(300, Math.round((viewportWidth - 288) * 0.3));

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
	const importPreviewLines = importPreview ? summarizeImportPreview(importPreview) : [];

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

  const openImportModal = () => {
    setImportFile(null);
    setImportFileError("");
    setImportPayload("");
    setImportPreview(null);
    setImportOpen(true);
  };

  const handlePreviewImportStack = async () => {
    if (!importFile) {
      setImportFileError(t("stackList.import.fileRequired", "Select an export file."));
      return;
    }

    try {
      const payload = await importFile.text();
      const preview = await previewImportStack.mutateAsync(payload);
      setImportPayload(payload);
      setImportPreview(preview);
      setImportFileError("");
    } catch (error) {
      toast.error((error as { message?: string })?.message ?? t("stackList.import.failure", "Failed to import stack."));
    }
  };

  const handleImportStack = async () => {
    if (!importPreview || !importPayload) {
      return;
    }

    try {
      const result = await importStack.mutateAsync({
        payload: importPayload,
        replaceExisting: importPreview.mode === "update",
      });
      setImportOpen(false);
      setImportFile(null);
      setImportFileError("");
      setImportPayload("");
      setImportPreview(null);
      setSearch("");
      setStatusFilter("");
      setClusterFilter("");
      toast.success(t("stackList.import.success", "Stack restored."));
      setExpandedStackId(result.id);
    } catch (error) {
      toast.error((error as { message?: string })?.message ?? t("stackList.import.failure", "Failed to import stack."));
    }
  };

	const columns: ColumnDef<Stack, unknown>[] = [
		{
			accessorKey: "name",
			header: t("stackList.table.stackName", "Stack Name"),
			// 클러스터를 별도 컬럼이 아니라 이름 아래에 둔다. 이 표는 상세 패널
			// 왼쪽의 좁은 칸에 들어가는데, 컬럼 넷을 가로로 세우면 폭이 모자라
			// 가로 스크롤이 생긴다 — 생성 시각이 화면 밖으로 밀려 안 보였다.
			cell: ({ row }) => (
				<div className="flex items-center gap-2">
					{selectedStackId === row.original.id && (
						<div className="h-1.5 w-1.5 shrink-0 rounded-full bg-[var(--color-primary)]" />
					)}
					<div className="min-w-0">
						<div className="truncate font-semibold" title={row.original.name}>{row.original.name}</div>
						<div
							className="truncate text-[11px] text-[var(--color-text-secondary)]"
							title={row.original.clusterName}
						>
							{row.original.clusterName}
						</div>
					</div>
				</div>
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
			// 같은 날 여러 번 설치하는 스택이라 날짜만으로는 순서가 안 보인다.
			// 날짜와 시각을 두 줄로 나눈다 — 한 줄로 붙이면("08/11/2026, 09:55 AM")
			// 좁은 칸에서 이 컬럼 하나가 160px 를 가져간다.
			cell: ({ row }) => (
				<div className="whitespace-nowrap text-[13px] text-[var(--color-text-secondary)]">
					<div>{localeDate(row.original.createdAt, i18n.language)}</div>
					<div className="text-[11px] text-[var(--color-text-muted)]">
						{localeTime(row.original.createdAt, i18n.language)}
					</div>
				</div>
			),
		},
	];

	// 데스크톱은 ListDetailPanel 안쪽, 그 아래 폭에서는 단독으로 선다.
	// 마크업은 그대로 두고 렌더 위치만 바뀌므로 순수 추출이다.
	const stackTable = (
		<DataTable
			flush={isDesktopLayout}
			key={`stack-list-${tablePageSize}`}
			columns={columns}
			data={filtered}
			pageSize={tablePageSize}
			toolbar={
				<>
					<Select
						value={statusFilter}
						onChange={(e) => setStatusFilter(e.target.value)}
					>
						<option value="">{t("stackList.filters.allStatus", "All Status")}</option>
						<option value="healthy">{t("stackList.status.healthy", "Running")}</option>
						<option value="completed">{t("stackList.status.completed", "Completed")}</option>
						<option value="running">{t("stackList.status.running", "Running")}</option>
						<option value="terminating">{t("stackList.status.terminating", "Terminating")}</option>
						<option value="pending">{t("stackList.status.pending", "Pending")}</option>
						<option value="failed">{t("stackList.status.failed", "Failed")}</option>
						<option value="cancelled">{t("stackList.status.cancelled", "Cancelled")}</option>
					</Select>
					<Select
						value={clusterFilter}
						onChange={(e) => setClusterFilter(e.target.value)}
					>
						<option value="">{t("stackList.filters.allClusters", "All Clusters")}</option>
						{clusterOptions.map((clusterName) => (
							<option key={clusterName} value={clusterName}>
								{clusterName}
							</option>
						))}
					</Select>
					<SearchInput
					  wrapperClassName="ml-auto w-[220px]"
					  placeholder={t("stackList.searchPlaceholder", "Search stacks...")}
					  value={search}
					  onChange={(e) => setSearch(e.target.value)}
					/>
				</>
			}
			getRowKey={(row) => row.id}
			onRowClick={(row) => setExpandedStackId(row.id)}
			emptyMessage={isLoading ? t("stackList.loading", "Loading stacks...") : t("stackList.empty", "No stacks found.")}
		/>
	);

	return (
		// 좌우 분할 화면은 페이지가 늘어나는 대신 각 칸이 자기 안에서 스크롤한다.
		// ListDetailPanel 은 h-full 이라 부모가 높이를 줘야 하는데, 여기가 그냥
		// <div> 였던 동안에는 높이가 auto 로 풀려 패널이 내용만큼 자랐다 — 그래서
		// 목록은 가만히 있고 상세만 페이지째 스크롤되는 것처럼 보였다.
		<div className="flex h-full flex-col">
			<PageHeader
			  breadcrumb={[{ label: t("sidebar.stackList", "Stack List") }]}
			  icon={<List {...iconProps('sm')} />}
			  tone="primary"
			  title={t("stackList.title", "Stack List")}
			  subtitle={t("stackList.description", "Deployed DevSecOps stack list")}
			  actions={
			    <div className="flex items-center gap-2">
			    	<Button
			    		variant="outline"
			    		size="md"
			    		onClick={openImportModal}
			    	>
			    		{t("stackList.actions.import", "Import")}
			    	</Button>
			    	<Button
			    		variant="primary"
			    		size="md"
			    		onClick={() =>
			    			navigate("/stack/templates", { state: { from: "stack-list" } })
			    		}
			    	>
			    		<Plus {...iconProps('sm')} />
			    		{t("stackList.actions.newStack", "New Stack")}
			    	</Button>
			    </div>
			  }
			/>

			{isDesktopLayout ? (
				<div className="min-h-0 flex-1">
				<ListDetailPanel
					listWidth={listPaneWidth}
					emptyDetailMessage={t("stackList.emptyDetail", "Select a stack from the list to view details here.")}
					listContent={
						<>
							{stackTable}
							<div className="px-3 py-2 text-[12px] text-[var(--color-text-secondary)]">
								{t("stackList.listHint", "Selecting a stack from the list updates the detail panel immediately.")}
							</div>
						</>
					}
					detailContent={
						expandedStack ? (
							<StackDetailPanel
								flush
								key={expandedStack.id}
								stack={expandedStack}
								clusterConnectionStatus={clusterConnectionByID.get(expandedStack.clusterId)}
								isDeleting={deleteStack.isPending}
								onAddTools={() => navigate(`/stack/${expandedStack.id}/add-tools`)}
								onDelete={() => setDeleteStackId(expandedStack.id)}
								onBackToList={() => setExpandedStackId(null)}
							/>
						) : null
					}
				/>
				</div>
			) : (
				/* 1280px 미만에서는 좌우로 나눌 폭이 없다. 목록 아래에 상세를 붙이는
				   기존 폴백을 그대로 둔다 (DESIGN.md §Layout — 데스크톱 우선). */
				<>
					{stackTable}
					{expandedStack && (
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
				</>
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

			<Modal
				open={importOpen}
				onClose={() => setImportOpen(false)}
				title={t("stackList.import.title", "Import Stack")}
				footer={
					<>
						<Button
							variant="outline"
							size="sm"
							type="button"
							onClick={() => setImportOpen(false)}
							disabled={importStack.isPending}
						>
							{t("common.cancel", "Cancel")}
						</Button>
						<Button
							variant="primary"
							size="sm"
							type="button"
							onClick={importPreview ? handleImportStack : handlePreviewImportStack}
							loading={importStack.isPending || previewImportStack.isPending}
						>
							{importPreview
								? t("stackList.import.confirm", "Restore")
								: t("stackList.import.preview", "Review Import")}
						</Button>
					</>
				}
			>
				<div className="space-y-4 text-[13px]">
					{!importPreview ? (
						<>
							<p className="text-[var(--color-text-secondary)]">
								{t("stackList.import.description", "Upload a stack export file to review and apply it.")}
							</p>
							<label className="flex flex-col gap-1">
								<span className="text-xs font-medium tracking-[0.02em] text-[var(--color-text-secondary)]">
									{t("stackList.import.file", "Export file")}
								</span>
								<input
									type="file"
									accept=".json,.yaml,.yml,application/json,text/yaml,text/x-yaml"
									onChange={(event) => {
										const file = event.target.files?.[0] ?? null;
										setImportFile(file);
										setImportFileError("");
									}}
									aria-label={t("stackList.import.file", "Export file")}
									className="box-border w-full cursor-pointer rounded-lg border border-[var(--color-border-default)] bg-[var(--color-surface-base)] px-3 py-[9px] text-sm text-[var(--color-text-primary)] outline-none transition-all duration-150 ease-in-out file:mr-3 file:rounded-md file:border-0 file:bg-[color-mix(in_srgb,_var(--color-primary)_15%,_transparent)] file:px-3 file:py-1.5 file:text-[12px] file:font-semibold file:text-[var(--color-primary)] focus:border-[var(--color-primary)]"
								/>
							</label>
							{importFile && (
								<div className="text-[12px] text-[var(--color-text-secondary)]">
									{t("stackList.import.selectedFile", "Selected:")} {importFile.name}
								</div>
							)}
							{importFileError && (
								<div className="text-[12px] text-[var(--color-error)]">{importFileError}</div>
							)}
						</>
					) : (
						<div className="space-y-3">
							<div className="text-[12px] text-[var(--color-text-secondary)]">
								{t("stackList.import.name", "Stack")}: <span className="text-[var(--color-text-primary)]">{importPreview.name}</span>
							</div>
							<div className="rounded-md border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] p-3">
								<div className="space-y-1 text-[12px] text-[var(--color-text-secondary)]">
									{importPreviewLines.map((line) => (
										<div key={line}>{line}</div>
									))}
								</div>
							</div>
							{importPreview.mode === "update" && importPreview.existing_state && (
								<div className="text-[12px] text-[var(--color-text-secondary)]">
									{t("stackList.import.currentState", "Current state")}: <span className="text-[var(--color-text-primary)]">{importPreview.existing_state}</span>
								</div>
							)}
							{importPreview.changes && (
								<div className="rounded-md border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] p-3">
									<div className="mb-2 text-[12px] font-semibold text-[var(--color-text-primary)]">
										{t("stackList.import.changes", "Import changes")}
									</div>
									<div className="text-[12px] text-[var(--color-text-secondary)]">
										{t("stackList.import.diffHint", "Field-level differences are included in this review.")}
									</div>
								</div>
							)}
						</div>
					)}
				</div>
			</Modal>
		</div>
	);
}
