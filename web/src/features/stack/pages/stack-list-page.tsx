import type { ColumnDef } from "@tanstack/react-table";
import {
	ChevronDown,
	ChevronUp,
	List,
	Plus,
	Search,
} from "lucide-react";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Breadcrumb } from "../../../components/shared/breadcrumb";
import { ConfirmDialog } from "../../../components/shared/confirm-dialog";
import { DataTable } from "../../../components/shared/data-table";
import { Button } from "../../../components/ui/button";
import { NativeSelect } from "../../../components/ui/native-select";
import type { Stack } from "../api/stack-api";
import { useDeleteStack, useStacks } from "../api/stack-api";
import { StackDetailPanel, STATUS_STYLES } from "./stack-detail-panel";

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
