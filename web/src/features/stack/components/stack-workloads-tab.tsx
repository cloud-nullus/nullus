// 스택이 클러스터에 실제로 띄운 파드 목록.
//
// 개편 전에는 이 자리에 안내 문구만 있었다: "상세 설치 카드는 숨김 처리되었습니다.
// 도구 상세 상태는 Monitoring / History 탭에서 확인할 수 있습니다." 그런데 두 탭
// 어디에도 파드 목록은 없었다 — Monitoring 은 집계 차트고 History 는 설정 스냅샷이다.
// 안내가 가리키는 곳에 물건이 없으면 안내가 아니라 막다른 길이다.
//
// 조회는 스택마다 다르다. 이 탭은 /stacks/{id}/monitoring 하나만 부르고, 네임스페이스는
// 응답이 알려 준다(서버가 stack.Namespace 에서 뽑는다). 화면에 박아 둔 조회 조건은 없다.

import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import type { ColumnDef } from "@tanstack/react-table";
import { AlertCircle, AlertTriangle, CheckCircle2 } from "lucide-react";
import { DataTable } from "../../../components/shared/data-table";
import { Badge } from "../../../components/ui/badge";
import { cn } from "../../../lib/utils";
import { useStackMonitoring } from "../api/stack-api";
import type { PodMonitoringStatus } from "../api/stack-api-types";
import { toolLogoURL } from "../utils/tool-logo";

type WorkloadRow = PodMonitoringStatus & {
  /** 이 파드를 띄운 OSS. 파드 이름만으로는 어느 도구인지 모르는 경우가 많다. */
  tool: string;
  toolVersion: string;
};

const STATUS_STYLE = {
  running: {
    icon: CheckCircle2,
    className:
      "bg-[color-mix(in_srgb,_var(--color-success)_15%,_transparent)] text-[var(--color-success)]",
  },
  warning: {
    icon: AlertTriangle,
    className:
      "bg-[color-mix(in_srgb,_var(--color-warning)_15%,_transparent)] text-[var(--color-warning)]",
  },
  error: {
    icon: AlertCircle,
    className:
      "bg-[color-mix(in_srgb,_var(--color-error)_15%,_transparent)] text-[var(--color-error)]",
  },
} as const;

// 문제 있는 파드가 먼저 눈에 들어와야 한다.
const STATUS_ORDER = { error: 0, warning: 1, running: 2 } as const;

function formatMillicores(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "-";
  return value >= 1000 ? `${(value / 1000).toFixed(2)} core` : `${Math.round(value)}m`;
}

function formatMiB(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "-";
  return value >= 1024 ? `${(value / 1024).toFixed(1)} Gi` : `${Math.round(value)} Mi`;
}

export function StackWorkloadsTab({ stackId }: { stackId: string }) {
  const { t } = useTranslation();
  const { data, isLoading } = useStackMonitoring(stackId, 30_000);

  const rows = useMemo<WorkloadRow[]>(() => {
    const tools = data?.oss_statuses ?? [];
    return tools
      .filter((tool) => tool.enabled)
      .flatMap((tool) =>
        (tool.pods ?? []).map((pod) => ({
          ...pod,
          tool: tool.name,
          toolVersion: tool.version,
        })),
      )
      .sort(
        (a, b) =>
          (STATUS_ORDER[a.status] ?? 1) - (STATUS_ORDER[b.status] ?? 1) ||
          a.tool.localeCompare(b.tool) ||
          a.name.localeCompare(b.name),
      );
  }, [data]);

  const notReady = rows.filter((row) => !row.ready).length;

  const columns: ColumnDef<WorkloadRow, unknown>[] = [
    {
      accessorKey: "name",
      header: () => <span className="whitespace-nowrap">Pod</span>,
      cell: ({ row }) => (
        <div className="flex items-center gap-2">
          <img
            src={toolLogoURL(row.original.tool)}
            alt=""
            aria-hidden="true"
            className="h-4 w-4 shrink-0 object-contain"
          />
          <div className="min-w-0">
            <div className="truncate font-mono text-[12px] text-[var(--color-text-primary)]" title={row.original.name}>
              {row.original.name}
            </div>
            <div className="truncate text-[11px] text-[var(--color-text-secondary)]">
              {row.original.tool} {row.original.toolVersion}
            </div>
          </div>
        </div>
      ),
    },
    {
      accessorKey: "status",
      header: () => <span className="whitespace-nowrap">Status</span>,
      cell: ({ row }) => {
        const style = STATUS_STYLE[row.original.status] ?? STATUS_STYLE.warning;
        const Icon = style.icon;
        return (
          <Badge pill className={cn("whitespace-nowrap", style.className)}>
            <Icon size={11} />
            {/* phase 는 쿠버네티스 값(Running/Pending/…) 그대로다. ready 는 별개 —
                Running 인데 준비가 안 된 파드가 장애의 대부분이라 함께 적는다. */}
            {row.original.phase}
            {!row.original.ready && " · not ready"}
          </Badge>
        );
      },
    },
    {
      accessorKey: "restart_count",
      header: () => <span className="whitespace-nowrap">Restarts</span>,
      cell: ({ row }) => (
        // 재시작 횟수에는 경보색을 쓰지 않는다. 노드가 한 번 재부팅되면 모든
        // 파드가 1 이 되는데, 그때 표 전체가 주황색이 되면 색이 뜻을 잃는다.
        // 경보 채널은 Status 컬럼 하나로 둔다.
        <span
          className={cn(
            "text-[13px]",
            row.original.restart_count > 0
              ? "font-semibold text-[var(--color-text-primary)]"
              : "text-[var(--color-text-muted)]",
          )}
        >
          {row.original.restart_count}
        </span>
      ),
    },
    {
      accessorKey: "cpu_usage_millicores",
      header: () => <span className="whitespace-nowrap">CPU</span>,
      cell: ({ row }) => (
        <span className="whitespace-nowrap text-[13px] text-[var(--color-text-secondary)]">
          {formatMillicores(row.original.cpu_usage_millicores)}
          <span className="text-[var(--color-text-muted)]">
            {" / "}
            {formatMillicores(row.original.cpu_limit_millicores)}
          </span>
        </span>
      ),
    },
    {
      accessorKey: "memory_usage_mib",
      header: () => <span className="whitespace-nowrap">Memory</span>,
      cell: ({ row }) => (
        <span className="whitespace-nowrap text-[13px] text-[var(--color-text-secondary)]">
          {formatMiB(row.original.memory_usage_mib)}
          <span className="text-[var(--color-text-muted)]">
            {" / "}
            {formatMiB(row.original.memory_limit_mib)}
          </span>
        </span>
      ),
    },
    {
      accessorKey: "node_name",
      header: () => <span className="whitespace-nowrap">Node</span>,
      cell: ({ row }) => (
        <span className="truncate text-[12px] text-[var(--color-text-secondary)]" title={row.original.node_name}>
          {row.original.node_name || "-"}
        </span>
      ),
    },
  ];

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <h3 className="m-0 text-[14px] font-bold text-[var(--color-text-primary)]">
          {t("stackList.workloads.title", "Workloads")}
        </h3>
        {data?.namespace && (
          <Badge
            pill
            className="bg-[color-mix(in_srgb,_var(--color-primary)_15%,_transparent)] font-mono text-[var(--color-primary)]"
          >
            ns/{data.namespace}
          </Badge>
        )}
        <span className="text-[12px] text-[var(--color-text-secondary)]">
          {t("stackList.workloads.summary", {
            ready: rows.length - notReady,
            total: rows.length,
            defaultValue: "{{ready}} / {{total}} pods ready",
          })}
        </span>
      </div>

      <DataTable
        flush
        columns={columns}
        data={rows}
        getRowKey={(row) => row.name}
        pageSize={12}
        emptyMessage={
          isLoading
            ? t("stackList.workloads.loading", "Reading pods from the cluster...")
            : t(
                "stackList.workloads.empty",
                "No pods found for this stack in the cluster.",
              )
        }
      />
    </div>
  );
}
