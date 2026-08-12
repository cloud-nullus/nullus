import { Box, Cpu, HardDrive, MemoryStick } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import {
  Area,
  Bar,
  BarChart,
  Cell,
  CartesianGrid,
  ComposedChart,
  Legend,
  Line,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { useTranslation } from "react-i18next";
import { cn } from "../../../lib/utils";
import { useStackMonitoring } from "../../stack/api/stack-api";
import { toolLogoURL } from "../../stack/utils/tool-logo";
import { CHART_LEGEND_PROPS, CHART_STYLE } from "./monitoring-chart-widgets";

// recharts 는 SVG 라 CSS 변수를 그대로 쓴다. 이 파일이 chart.js(<canvas>)를 쓰던
// 동안에는 var() 를 못 읽어 차트가 검게 렌더됐고, 그걸 메우려고 theme/resolve-token
// 의 resolveColor() 로 색 58곳을 감싸고 있었다. 캔버스가 없어졌으니 그 다리도 없다.

// 차트 높이는 퍼센트가 아니라 픽셀로 준다. height="100%" 는 부모의 확정 높이를
// 측정해야 하는데, 이 차트들은 상세 패널의 flex 안(ListDetailPanel → 탭 본문)에
// 있어서 첫 측정 시점에 높이가 0 이다 — 브라우저 콘솔에 
// "The width(-1) and height(-1) of chart should be greater than 0" 가 4번 찍혔다.
// 감싸는 div 가 이미 h-[250px] 이라 값은 같고, 측정 의존만 없앤다.
// 이 저장소의 다른 차트(monitoring-cicd-view 200, cicd-list 160)도 픽셀을 쓴다.

/** 축 눈금: 1 미만은 소수 둘째 자리까지, 그 밖은 반올림. chart.js 콜백과 같은 규칙이다. */
function formatAxisValue(value: number | string): string {
  const n = Number(value);
  if (!Number.isFinite(n)) return "0";
  if (n !== 0 && Math.abs(n) < 1) return n.toFixed(2);
  return `${Math.round(n)}`;
}

/**
 * OSS 막대 아래 로고 줄.
 *
 * 차트 안이 아니라 형제로 둔다. recharts 는 카테고리를 플롯 영역에 균등 배치하므로,
 * 플롯 영역과 같은 좌우 여백(YAxis 폭 40px, margin.right 8px)을 준 flex 행이면
 * 각 칸의 중앙이 곧 막대 그룹의 중앙이다 — 측정이 필요 없다.
 *
 * 개편 전에는 chart.js 인스턴스 ref 로 `xScale.getPixelForValue(idx)` 를 읽어
 * 절대 배치했다. rAF + resize 리스너 + 위치 state 가 딸렸고, 리렌더 타이밍에
 * 따라 아이콘이 막대와 어긋났다. 그 셋을 전부 지웠다.
 */
const OSS_PLOT_INSET = "pl-10 pr-2";

function OssLogoRow({ items }: { items: { key: string; fullName: string; iconUrl: string }[] }) {
  return (
    <div className={cn("pointer-events-none flex h-5 items-start", OSS_PLOT_INSET)}>
      {items.map((item) => (
        <div key={`oss-icon-${item.key}`} className="flex min-w-0 flex-1 justify-center">
          <div className="relative h-4 w-4">
            <img
              src={item.iconUrl}
              alt={`${item.fullName} icon`}
              className="h-4 w-4 rounded-[3px]"
              loading="lazy"
              onError={(event) => {
                event.currentTarget.style.display = "none";
                const fallback = event.currentTarget
                  .nextElementSibling as HTMLElement | null;
                if (fallback) fallback.style.display = "flex";
              }}
            />
            <span
              className="hidden h-4 w-4 items-center justify-center rounded-[3px] bg-[color-mix(in_srgb,_var(--color-text-secondary)_25%,_transparent)] text-[9px] font-bold text-[var(--color-text-primary)]"
              aria-hidden="true"
            >
              {item.fullName.slice(0, 1).toUpperCase()}
            </span>
          </div>
        </div>
      ))}
    </div>
  );
}

type MonitoringRange = "realtime" | "1h" | "6h" | "24h" | "7d";

type MonitoringSample = {
  ts: number;
  overall: ScopeMetrics;
  byTool: Record<string, ScopeMetrics>;
};

type ScopeMetrics = {
  cpuRequest: number;
  cpuLimit: number;
  cpuUsage: number | null;
  memoryRequest: number;
  memoryLimit: number;
  memoryUsage: number | null;
  storageRequest: number;
  storageLimit: number;
  storageUsage: number | null;
  readyPods: number;
  totalPods: number;
  statusCounts: Record<string, number>;
};

function UsageBar({ value, color }: { value: number; color: string }) {
  const normalized = Math.max(0, Math.min(100, value));
  return (
    <div className="mt-2 h-1.5 w-full overflow-hidden rounded-[3px] bg-[color-mix(in_srgb,_var(--color-text-primary)_8%,_transparent)]">
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

function normalizeToPercent(value: number, maxValue: number): number {
  if (maxValue <= 0) return 0;
  return Math.max(0, Math.min(100, Math.round((value / maxValue) * 100)));
}

function selectSeries(
  samples: MonitoringSample[],
  range: MonitoringRange,
): MonitoringSample[] {
  if (samples.length === 0) return [];
  if (range === "realtime") {
    return samples.slice(-60);
  }

  const now = Date.now();
  const windowMs: Record<Exclude<MonitoringRange, "realtime">, number> = {
    "1h": 60 * 60 * 1000,
    "6h": 6 * 60 * 60 * 1000,
    "24h": 24 * 60 * 60 * 1000,
    "7d": 7 * 24 * 60 * 60 * 1000,
  };
  const cutoff = now - windowMs[range];
  const ranged = samples.filter((s) => s.ts >= cutoff);
  if (ranged.length <= 120) return ranged;

  const stride = Math.ceil(ranged.length / 120);
  return ranged.filter((_, idx) => idx % stride === 0);
}

function toStatusCountMap(
  items: Array<{ name: string; count: number }>,
): Record<string, number> {
  return items.reduce<Record<string, number>>((acc, item) => {
    acc[item.name] = item.count;
    return acc;
  }, {});
}

function toScopeMetricsFromPods(
  pods: Array<{
    phase: string;
    ready: boolean;
    cpu_request_millicores: number;
    cpu_limit_millicores: number;
    cpu_usage_millicores: number;
    memory_request_mib: number;
    memory_limit_mib: number;
    memory_usage_mib: number;
    storage_request_gib?: number;
    storage_limit_gib?: number;
    storage_usage_gib?: number;
  }>,
): ScopeMetrics {
  const statusCounts: Record<string, number> = {};
  let cpuRequest = 0;
  let cpuLimit = 0;
  let cpuUsage = 0;
  let memoryRequest = 0;
  let memoryLimit = 0;
  let memoryUsage = 0;
  let storageRequest = 0;
  let storageLimit = 0;
  let storageUsage = 0;
  let storageUsageHit = false;
  let readyPods = 0;

  for (const pod of pods) {
    const phase = pod.phase?.trim() || "Unknown";
    statusCounts[phase] = (statusCounts[phase] ?? 0) + 1;
    cpuRequest += pod.cpu_request_millicores;
    cpuLimit += pod.cpu_limit_millicores;
    cpuUsage += pod.cpu_usage_millicores;
    memoryRequest += pod.memory_request_mib;
    memoryLimit += pod.memory_limit_mib;
    memoryUsage += pod.memory_usage_mib;
    storageRequest += pod.storage_request_gib ?? 0;
    storageLimit += pod.storage_limit_gib ?? 0;
    if (typeof pod.storage_usage_gib === "number") {
      storageUsage += pod.storage_usage_gib;
      storageUsageHit = true;
    }
    if (pod.ready) readyPods += 1;
  }

  return {
    cpuRequest,
    cpuLimit,
    cpuUsage,
    memoryRequest,
    memoryLimit,
    memoryUsage,
    storageRequest,
    storageLimit,
    storageUsage: storageUsageHit ? storageUsage : null,
    readyPods,
    totalPods: pods.length,
    statusCounts,
  };
}

function isResourceLinkedToPods(
  resourceName: string,
  podNames: string[],
): boolean {
  for (const podName of podNames) {
    if (podName === resourceName || podName.startsWith(`${resourceName}-`)) {
      return true;
    }
  }
  return false;
}

export function StackMonitoringOverview({ stackId }: { stackId: string }) {
  const { t } = useTranslation();
  const [range, setRange] = useState<MonitoringRange>("realtime");
  const [scope, setScope] = useState<string>("all");
  const [samples, setSamples] = useState<MonitoringSample[]>([]);
  const { data: monitoring, isLoading } = useStackMonitoring(stackId, 5000);

  const scopeOptions = useMemo(
    () => [
      { key: "all", label: "All" },
      ...(monitoring?.oss_statuses ?? []).map((tool) => ({
        key: tool.key,
        label: tool.name,
      })),
    ],
    [monitoring],
  );

  useEffect(() => {
    if (!scopeOptions.some((item) => item.key === scope)) {
      setScope("all");
    }
  }, [scopeOptions, scope]);

  useEffect(() => {
    if (!monitoring?.summary) return;

    const overall: ScopeMetrics = {
      cpuRequest: monitoring.summary.cpu_request_millicores,
      cpuLimit: monitoring.summary.cpu_limit_millicores,
      cpuUsage: monitoring.summary.usage_available
        ? monitoring.summary.cpu_usage_millicores
        : null,
      memoryRequest: monitoring.summary.memory_request_mib,
      memoryLimit: monitoring.summary.memory_limit_mib,
      memoryUsage: monitoring.summary.usage_available
        ? monitoring.summary.memory_usage_mib
        : null,
      storageRequest: monitoring.summary.storage_request_gib ?? 0,
      storageLimit: monitoring.summary.storage_limit_gib ?? 0,
      storageUsage: monitoring.summary.storage_usage_available
        ? (monitoring.summary.storage_usage_gib ?? 0)
        : null,
      readyPods: monitoring.summary.ready_pods,
      totalPods: monitoring.summary.total_pods,
      statusCounts: toStatusCountMap(monitoring.pod_status_counts ?? []),
    };

    const byTool = (monitoring.oss_statuses ?? []).reduce<
      Record<string, ScopeMetrics>
    >((acc, tool) => {
      const metrics = toScopeMetricsFromPods(tool.pods ?? []);
      if (!monitoring.summary.usage_available) {
        metrics.cpuUsage = null;
        metrics.memoryUsage = null;
      }
      acc[tool.key] = metrics;
      return acc;
    }, {});

    const next: MonitoringSample = {
      ts: Date.now(),
      overall,
      byTool,
    };

    setSamples((prev) => [...prev, next].slice(-4000));
  }, [monitoring]);

  const activeTool = useMemo(
    () =>
      (monitoring?.oss_statuses ?? []).find((tool) => tool.key === scope) ??
      null,
    [monitoring, scope],
  );

  const currentMetrics = useMemo<ScopeMetrics>(() => {
    if (!monitoring?.summary) {
      return {
        cpuRequest: 0,
        cpuLimit: 0,
        cpuUsage: null,
        memoryRequest: 0,
        memoryLimit: 0,
        memoryUsage: null,
        storageRequest: 0,
        storageLimit: 0,
        storageUsage: null,
        readyPods: 0,
        totalPods: 0,
        statusCounts: {},
      };
    }
    if (scope === "all") {
      return {
        cpuRequest: monitoring.summary.cpu_request_millicores,
        cpuLimit: monitoring.summary.cpu_limit_millicores,
        cpuUsage: monitoring.summary.usage_available
          ? monitoring.summary.cpu_usage_millicores
          : null,
        memoryRequest: monitoring.summary.memory_request_mib,
        memoryLimit: monitoring.summary.memory_limit_mib,
        memoryUsage: monitoring.summary.usage_available
          ? monitoring.summary.memory_usage_mib
          : null,
        storageRequest: monitoring.summary.storage_request_gib ?? 0,
        storageLimit: monitoring.summary.storage_limit_gib ?? 0,
        storageUsage: monitoring.summary.storage_usage_available
          ? (monitoring.summary.storage_usage_gib ?? 0)
          : null,
        readyPods: monitoring.summary.ready_pods,
        totalPods: monitoring.summary.total_pods,
        statusCounts: toStatusCountMap(monitoring.pod_status_counts ?? []),
      };
    }
    if (!activeTool) {
      return {
        cpuRequest: 0,
        cpuLimit: 0,
        cpuUsage: null,
        memoryRequest: 0,
        memoryLimit: 0,
        memoryUsage: null,
        storageRequest: 0,
        storageLimit: 0,
        storageUsage: null,
        readyPods: 0,
        totalPods: 0,
        statusCounts: {},
      };
    }
    const scoped = toScopeMetricsFromPods(activeTool.pods ?? []);
    if (!monitoring.summary.usage_available) {
      scoped.cpuUsage = null;
      scoped.memoryUsage = null;
    }
    return scoped;
  }, [monitoring, scope, activeTool]);

  const usageData = useMemo(() => {
    const selected = selectSeries(samples, range);
    return selected.map((item) => {
      const ts = new Date(item.ts);
      const scoped =
        scope === "all"
          ? item.overall
          : (item.byTool[scope] ?? {
              cpuRequest: 0,
              cpuLimit: 0,
              cpuUsage: null,
              memoryRequest: 0,
              memoryLimit: 0,
              memoryUsage: null,
              storageRequest: 0,
              storageLimit: 0,
              storageUsage: null,
              readyPods: 0,
              totalPods: 0,
              statusCounts: {},
            });
      return {
        time:
          range === "7d"
            ? ts.toLocaleDateString("en-US", {
                month: "2-digit",
                day: "2-digit",
              })
            : ts.toLocaleTimeString("en-US", {
                hour: "2-digit",
                minute: "2-digit",
                second: "2-digit",
                hour12: false,
              }),
        cpuRequest: scoped.cpuRequest,
        cpuLimit: scoped.cpuLimit,
        cpuUsage: scoped.cpuUsage,
        memoryRequest: scoped.memoryRequest,
        memoryLimit: scoped.memoryLimit,
        memoryUsage: scoped.memoryUsage,
      };
    });
  }, [samples, range, scope]);

  const cpuMaxInWindow = useMemo(() => {
    const values = usageData.flatMap((item) => [
      item.cpuRequest,
      item.cpuLimit,
      item.cpuUsage ?? 0,
    ]);
    return Math.max(0, ...values);
  }, [usageData]);

  const memoryMaxInWindow = useMemo(() => {
    const values = usageData.flatMap((item) => [
      item.memoryRequest,
      item.memoryLimit,
      item.memoryUsage ?? 0,
    ]);
    return Math.max(0, ...values);
  }, [usageData]);

  const podStatusData = useMemo(() => {
    const palette = [
      "var(--color-success)",
      "var(--color-warning)",
      "var(--color-error)",
      "var(--color-info)",
      "var(--color-accent-alt)",
      "var(--color-text-secondary)",
    ];
    const counts = Object.entries(currentMetrics.statusCounts).map(
      ([name, count]) => ({ name, count }),
    );
    return counts.map((item, idx) => ({
      name: item.name,
      value: item.count,
      color: palette[idx % palette.length],
    }));
  }, [currentMetrics]);

  const ossBars = useMemo(() => {
    if (scope === "all") {
      return [...(monitoring?.oss_statuses ?? [])]
        .sort((a, b) => a.name.localeCompare(b.name))
        .map((tool) => ({
          key: tool.key,
          fullName: tool.name,
          iconUrl: toolLogoURL(tool.name),
          pods: tool.pod_count,
          ready: tool.ready_pods,
        }));
    }
    if (!activeTool) return [];
    return [
      {
        key: activeTool.key,
        fullName: activeTool.name,
        iconUrl: toolLogoURL(activeTool.name),
        pods: activeTool.pod_count,
        ready: activeTool.ready_pods,
      },
    ];
  }, [monitoring, scope, activeTool]);

  const visibleResources = useMemo(() => {
    const all = monitoring?.installed_resources ?? [];
    if (scope === "all" || !activeTool) return all;
    const podNames = (activeTool.pods ?? []).map((pod) => pod.name);
    return all.filter((res) => isResourceLinkedToPods(res.name, podNames));
  }, [monitoring, scope, activeTool]);

  const kpiCards = useMemo(() => {
    if (!monitoring?.summary) {
      return [
        {
          label: "Current CPU",
          value: "-",
          icon: <Cpu size={18} />,
          color: "var(--color-info)",
          iconWrapClassName: "bg-[color-mix(in_srgb,_var(--color-info)_15%,_transparent)] text-[var(--color-info)]",
          bar: 0,
          metricScale: { current: null, request: 0, limit: 0, unit: "Core" },
        },
        {
          label: "Current Memory",
          value: "-",
          icon: <MemoryStick size={18} />,
          color: "var(--color-accent-alt)",
          iconWrapClassName: "bg-[color-mix(in_srgb,_var(--color-accent-alt)_15%,_transparent)] text-[var(--color-accent-alt)]",
          bar: 0,
          metricScale: { current: null, request: 0, limit: 0, unit: "GiB" },
        },
        {
          label: "Current Storage",
          value: "-",
          icon: <HardDrive size={18} />,
          color: "var(--color-success)",
          iconWrapClassName: "bg-[color-mix(in_srgb,_var(--color-success)_15%,_transparent)] text-[var(--color-success)]",
          bar: 0,
        },
        {
          label: "Ready Pods",
          value: "-",
          icon: <Box size={18} />,
          color: "var(--color-warning)",
          iconWrapClassName: "bg-[color-mix(in_srgb,_var(--color-warning)_15%,_transparent)] text-[var(--color-warning)]",
          bar: 0,
        },
      ];
    }

    const readyRatio =
      currentMetrics.totalPods > 0
        ? Math.round(
            (currentMetrics.readyPods / currentMetrics.totalPods) * 100,
          )
        : 0;
    const cpuCurrentBar =
      currentMetrics.cpuUsage !== null
        ? normalizeToPercent(
            currentMetrics.cpuUsage,
            currentMetrics.cpuRequest ||
              currentMetrics.cpuLimit ||
              cpuMaxInWindow ||
              1,
          )
        : 0;
    const memoryCurrentBar =
      currentMetrics.memoryUsage !== null
        ? normalizeToPercent(
            currentMetrics.memoryUsage,
            currentMetrics.memoryRequest ||
              currentMetrics.memoryLimit ||
              memoryMaxInWindow ||
              1,
          )
        : 0;
    const storageCurrentBar =
      currentMetrics.storageUsage !== null
        ? normalizeToPercent(
            currentMetrics.storageUsage,
            currentMetrics.storageRequest || currentMetrics.storageLimit || 1,
          )
        : 0;
    const cpuUsageC =
      currentMetrics.cpuUsage !== null ? currentMetrics.cpuUsage / 1000 : null;
    const cpuRequestC = currentMetrics.cpuRequest / 1000;
    const cpuLimitC = currentMetrics.cpuLimit / 1000;
    const memoryUsageGiB =
      currentMetrics.memoryUsage !== null
        ? currentMetrics.memoryUsage / 1024
        : null;
    const memoryRequestGiB = currentMetrics.memoryRequest / 1024;
    const memoryLimitGiB = currentMetrics.memoryLimit / 1024;
    const storageUsageGiB = currentMetrics.storageUsage;
    const storageRequestGiB = currentMetrics.storageRequest;
    const storageLimitGiB = currentMetrics.storageLimit;

    return [
      {
        label: "Current CPU",
        value: cpuUsageC !== null ? `${cpuUsageC.toFixed(2)} Core` : "N/A",
        icon: <Cpu size={18} />,
        color: "var(--color-info)",
        iconWrapClassName: "bg-[color-mix(in_srgb,_var(--color-info)_15%,_transparent)] text-[var(--color-info)]",
        bar: cpuCurrentBar,
        metricScale: {
          current: cpuUsageC,
          request: cpuRequestC,
          limit: cpuLimitC,
          unit: "Core",
        },
      },
      {
        label: "Current Memory",
        value:
          memoryUsageGiB !== null ? `${memoryUsageGiB.toFixed(2)} GiB` : "N/A",
        icon: <MemoryStick size={18} />,
        color: "var(--color-accent-alt)",
        iconWrapClassName: "bg-[color-mix(in_srgb,_var(--color-accent-alt)_15%,_transparent)] text-[var(--color-accent-alt)]",
        bar: memoryCurrentBar,
        metricScale: {
          current: memoryUsageGiB,
          request: memoryRequestGiB,
          limit: memoryLimitGiB,
          unit: "GiB",
        },
      },
      {
        label: "Current Storage",
        value:
          storageUsageGiB !== null
            ? `${storageUsageGiB.toFixed(2)} GiB`
            : storageLimitGiB > 0
              ? `${storageLimitGiB.toFixed(2)} GiB`
              : storageRequestGiB > 0
                ? `${storageRequestGiB.toFixed(2)} GiB`
                : "0.00 GiB",
        icon: <HardDrive size={18} />,
        color: "var(--color-success)",
        iconWrapClassName: "bg-[color-mix(in_srgb,_var(--color-success)_15%,_transparent)] text-[var(--color-success)]",
        bar: storageCurrentBar,
        metricScale: {
          current: storageUsageGiB,
          request: storageRequestGiB,
          limit: storageRequestGiB,
          unit: "GiB",
          showLimit: false,
        },
      },
      {
        label: "Ready Pods",
        value: `${currentMetrics.readyPods} / ${currentMetrics.totalPods}`,
        icon: <Box size={18} />,
        color: "var(--color-warning)",
        iconWrapClassName: "bg-[color-mix(in_srgb,_var(--color-warning)_15%,_transparent)] text-[var(--color-warning)]",
        bar: readyRatio,
      },
    ];
  }, [monitoring, currentMetrics, cpuMaxInWindow, memoryMaxInWindow]);

  // recharts 는 행 배열을 먹는다. chart.js 의 datasets 구조를 옮기면서 단위 변환
  // (밀리코어→코어, MiB→GiB)은 그대로 둔다.
  const cpuSeries = useMemo(
    () =>
      usageData.map((item) => ({
        time: item.time,
        request: item.cpuRequest / 1000,
        limit: item.cpuLimit / 1000,
        current: item.cpuUsage === null ? null : item.cpuUsage / 1000,
      })),
    [usageData],
  );

  const memorySeries = useMemo(
    () =>
      usageData.map((item) => ({
        time: item.time,
        request: item.memoryRequest / 1024,
        limit: item.memoryLimit / 1024,
        current: item.memoryUsage === null ? null : item.memoryUsage / 1024,
      })),
    [usageData],
  );

  const cardClassName =
    "rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-[var(--card-padding)]";

  return (
    <div>
      <div className={cn(cardClassName, "mb-6")}>
        <div className="mb-3.5 flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <h2 className="m-0 text-[15px] font-bold text-[var(--color-text-primary)]">
              Resource Trend
            </h2>
            {isLoading && (
              <span className="rounded-full bg-[color-mix(in_srgb,_var(--color-primary)_15%,_transparent)] px-2 py-0.5 text-[11px] font-semibold text-[var(--color-primary)]">
                Loading...
              </span>
            )}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <div className="flex gap-1.5">
              {scopeOptions.map((item) => {
                const active = scope === item.key;
                return (
                  <button
                    key={item.key}
                    type="button"
                    onClick={() => setScope(item.key)}
                    className={cn(
                      "cursor-pointer rounded-[7px] border px-2.5 py-[5px] text-xs font-semibold",
                      active
                        ? "border-[color-mix(in_srgb,_var(--color-success)_65%,_transparent)] bg-[color-mix(in_srgb,_var(--color-success)_20%,_transparent)] text-[var(--color-success)]"
                        : "border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_3%,_transparent)] text-[var(--color-text-secondary)]",
                    )}
                  >
                    {item.label}
                  </button>
                );
              })}
            </div>
            <div className="h-4 w-px bg-[var(--color-border-default)]" />
            <div className="flex gap-1.5">
              {(["realtime", "1h", "6h", "24h", "7d"] as const).map((item) => {
                const active = range === item;
                return (
                  <button
                    key={item}
                    type="button"
                    onClick={() => setRange(item)}
                    className={cn(
                      "cursor-pointer rounded-[7px] border px-2.5 py-[5px] text-xs font-bold",
                      active
                        ? "border-[color-mix(in_srgb,_var(--color-warning)_60%,_transparent)] bg-[color-mix(in_srgb,_var(--color-warning)_20%,_transparent)] text-[var(--color-warning)]"
                        : "border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_3%,_transparent)] text-[var(--color-text-secondary)]",
                    )}
                  >
                    {item === "realtime" ? "Live 5s" : item}
                  </button>
                );
              })}
            </div>
          </div>
        </div>
        <div className="mb-4 grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-4">
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
              {card.metricScale ? (
                <div className="mt-2">
                  {(() => {
                    const showLimit = card.metricScale.showLimit !== false;
                    const scaleLimit = showLimit
                      ? card.metricScale.limit
                      : card.metricScale.request;
                    const scaleMax = Math.max(scaleLimit, 0.000001);
                    const reqPos = Math.max(
                      0,
                      Math.min(
                        100,
                        (card.metricScale.request / scaleMax) * 100,
                      ),
                    );
                    const limPos = showLimit ? 100 : reqPos;
                    const curPos =
                      card.metricScale.current === null
                        ? null
                        : Math.max(
                            0,
                            Math.min(
                              100,
                              (card.metricScale.current / scaleMax) * 100,
                            ),
                          );


                    return (
                      <>
                        <div className="relative h-4">
                          <div className="absolute left-0 right-0 top-1 h-2 rounded-full bg-[color-mix(in_srgb,_var(--color-text-secondary)_22%,_transparent)]" />
                          {curPos !== null && (
                            <div
                              className="absolute left-0 top-1 h-2 rounded-full"
                              style={{
                                width: `${curPos}%`,
                                backgroundColor: card.color,
                              }}
                            />
                          )}
                          <div
                            className="absolute top-0.5 h-[10px] w-px bg-[var(--color-info)]"
                            style={{ left: `${reqPos}%` }}
                          />
                          {showLimit && (
                            <div
                              className="absolute top-0.5 h-[10px] w-px bg-[var(--color-warning)]"
                              style={{ left: `${limPos}%` }}
                            />
                          )}
                        </div>
                        {/*
                          값 비율 위치에 라벨을 절대 배치하던 구조를 걷어냈다.
                          request 와 limit 이 가까우면(예: Req 3.23 / Lim 1.50) 라벨이
                          반드시 겹쳐 글자가 잘렸다. 충돌 처리를 덧붙이는 대신
                          겹칠 수 없는 범례 한 줄로 바꿨다 — 읽기도 더 쉽다.
                        */}
                        <div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-[10px] font-semibold text-[var(--color-text-secondary)]">
                          <span className="inline-flex items-center gap-1">
                            <span className="inline-block h-2 w-px bg-[var(--color-info)]" aria-hidden="true" />
                            {t('monitoring.scale.request', 'Req')} {card.metricScale.request.toFixed(2)}
                          </span>
                          {showLimit && (
                            <span className="inline-flex items-center gap-1">
                              <span className="inline-block h-2 w-px bg-[var(--color-warning)]" aria-hidden="true" />
                              {t('monitoring.scale.limit', 'Lim')} {card.metricScale.limit.toFixed(2)}
                            </span>
                          )}
                          {curPos !== null && (
                            <span className="inline-flex items-center gap-1">
                              <span
                                className="inline-block h-2 w-2 rounded-full"
                                style={{ backgroundColor: card.color }}
                                aria-hidden="true"
                              />
                              {t('monitoring.scale.current', 'Now')} {(card.metricScale.current ?? 0).toFixed(2)}
                            </span>
                          )}
                        </div>
                      </>
                    );
                  })()}
                </div>
              ) : (
                <UsageBar value={card.bar} color={card.color} />
              )}
            </div>
          ))}
        </div>
        <div className="grid grid-cols-1 gap-3.5 xl:grid-cols-2">
          <div className="rounded-[10px] border border-[var(--color-border-default)] bg-[var(--color-surface-base)] p-2.5">
            <div className="mb-2 text-[13px] font-bold text-[var(--color-text-primary)]">
              CPU (Request / Limit / Current)
            </div>
            <div className="h-[250px]">
              <ResponsiveContainer width="100%" height={250}>
                <ComposedChart data={cpuSeries} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
                  <defs>
                    <linearGradient id="cpu-request-fill" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor="var(--color-info)" stopOpacity={0.28} />
                      <stop offset="100%" stopColor="var(--color-info)" stopOpacity={0.04} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid stroke={CHART_STYLE.grid} strokeDasharray="3 3" />
                  <XAxis dataKey="time" stroke="var(--color-text-secondary)" tick={CHART_STYLE.tick} minTickGap={24} />
                  <YAxis
                    stroke="var(--color-text-secondary)"
                    tick={CHART_STYLE.tick}
                    tickFormatter={formatAxisValue}
                    width={46}
                    label={{ value: "Core", angle: -90, position: "insideLeft", fill: "var(--color-text-secondary)", fontSize: 11 }}
                  />
                  <Tooltip contentStyle={CHART_STYLE.tooltip} />
                  <Legend {...CHART_LEGEND_PROPS} />
                  <Area type="monotone" dataKey="request" name="CPU Request" stroke="var(--color-info)" strokeWidth={2} fill="url(#cpu-request-fill)" dot={false} activeDot={{ r: 3 }} />
                  <Line type="monotone" dataKey="limit" name="CPU Limit" stroke="var(--color-warning)" strokeWidth={2} dot={false} activeDot={{ r: 3 }} />
                  <Line type="monotone" dataKey="current" name="CPU Current" stroke="var(--color-success)" strokeWidth={2} dot={false} activeDot={{ r: 3 }} connectNulls={false} />
                </ComposedChart>
              </ResponsiveContainer>
            </div>
          </div>

          <div className="rounded-[10px] border border-[var(--color-border-default)] bg-[var(--color-surface-base)] p-2.5">
            <div className="mb-2 text-[13px] font-bold text-[var(--color-text-primary)]">
              Memory (Request / Limit / Current)
            </div>
            <div className="h-[250px]">
              <ResponsiveContainer width="100%" height={250}>
                <ComposedChart data={memorySeries} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
                  <defs>
                    <linearGradient id="memory-request-fill" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor="var(--color-info)" stopOpacity={0.28} />
                      <stop offset="100%" stopColor="var(--color-info)" stopOpacity={0.04} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid stroke={CHART_STYLE.grid} strokeDasharray="3 3" />
                  <XAxis dataKey="time" stroke="var(--color-text-secondary)" tick={CHART_STYLE.tick} minTickGap={24} />
                  <YAxis
                    stroke="var(--color-text-secondary)"
                    tick={CHART_STYLE.tick}
                    tickFormatter={formatAxisValue}
                    width={46}
                    label={{ value: "GiB", angle: -90, position: "insideLeft", fill: "var(--color-text-secondary)", fontSize: 11 }}
                  />
                  <Tooltip contentStyle={CHART_STYLE.tooltip} />
                  <Legend {...CHART_LEGEND_PROPS} />
                  <Area type="monotone" dataKey="request" name="Memory Request" stroke="var(--color-info)" strokeWidth={2} fill="url(#memory-request-fill)" dot={false} activeDot={{ r: 3 }} />
                  <Line type="monotone" dataKey="limit" name="Memory Limit" stroke="var(--color-warning)" strokeWidth={2} dot={false} activeDot={{ r: 3 }} />
                  <Line type="monotone" dataKey="current" name="Memory Current" stroke="var(--color-success)" strokeWidth={2} dot={false} activeDot={{ r: 3 }} connectNulls={false} />
                </ComposedChart>
              </ResponsiveContainer>
            </div>
          </div>

          <div className="rounded-[10px] border border-[var(--color-border-default)] bg-[var(--color-surface-base)] p-2.5">
            <div className="mb-2 text-[13px] font-bold text-[var(--color-text-primary)]">
              OSS Pod Coverage
            </div>
            <div className="h-[250px]">
              <ResponsiveContainer width="100%" height={250}>
                <BarChart data={ossBars} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
                  <CartesianGrid stroke={CHART_STYLE.grid} strokeDasharray="3 3" />
                  {/* 눈금 글자는 숨긴다 — 도구 이름은 아래 로고 줄과 툴팁이 들고 있다. */}
                  <XAxis
                    dataKey="fullName"
                    stroke="var(--color-text-secondary)"
                    interval={0}
                    tickLine={false}
                    tick={false}
                    height={4}
                  />
                  <YAxis stroke="var(--color-text-secondary)" tick={CHART_STYLE.tick} allowDecimals={false} width={40} />
                  <Tooltip contentStyle={CHART_STYLE.tooltip} cursor={{ fill: "color-mix(in srgb, var(--color-text-secondary) 8%, transparent)" }} />
                  <Legend {...CHART_LEGEND_PROPS} />
                  <Bar dataKey="pods" name="Total Pods" fill="color-mix(in srgb, var(--color-primary) 72%, transparent)" radius={[6, 6, 0, 0]} />
                  <Bar dataKey="ready" name="Ready Pods" fill="color-mix(in srgb, var(--color-success) 72%, transparent)" radius={[6, 6, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            </div>
            <OssLogoRow items={ossBars} />
          </div>

          <div className="rounded-[10px] border border-[var(--color-border-default)] bg-[var(--color-surface-base)] p-2.5">
            <div className="mb-2 text-[13px] font-bold text-[var(--color-text-primary)]">
              Pod Status
            </div>
            <div className="h-[250px]">
              <ResponsiveContainer width="100%" height={250}>
                <PieChart>
                  <Pie
                    data={podStatusData}
                    dataKey="value"
                    nameKey="name"
                    cx="50%"
                    cy="45%"
                    innerRadius="52%"
                    outerRadius="84%"
                    stroke="color-mix(in srgb, var(--color-text-primary) 80%, transparent)"
                    strokeWidth={2}
                  >
                    {podStatusData.map((item) => (
                      <Cell key={item.name} fill={item.color} />
                    ))}
                  </Pie>
                  <Tooltip contentStyle={CHART_STYLE.tooltip} />
                  <Legend verticalAlign="bottom" {...CHART_LEGEND_PROPS} />
                </PieChart>
              </ResponsiveContainer>
            </div>
          </div>
        </div>

        <div className="mt-3 text-xs text-[var(--color-text-secondary)]">
          Metrics refresh every 5 seconds. Scoped charts and tables reflect the
          currently selected OSS range.
        </div>
      </div>
      <div className={cardClassName}>
        <h2 className="m-0 mb-4 text-[15px] font-bold text-[var(--color-text-primary)]">
          Tool Health
        </h2>
        <div className="mb-4 overflow-x-auto rounded-[10px] border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)]">
          <table className="min-w-full text-left text-[12px] text-[var(--color-text-secondary)]">
            <thead>
              <tr className="border-b border-[var(--color-border-default)] text-[11px] uppercase tracking-[0.05em]">
                <th className="px-3 py-2">Kind</th>
                <th className="px-3 py-2">Resource</th>
                <th className="px-3 py-2">Ready/Desired</th>
                <th className="px-3 py-2">Status</th>
              </tr>
            </thead>
            <tbody>
              {visibleResources.map((item) => (
                <tr
                  key={`${item.kind}-${item.name}`}
                  className="border-b border-[color-mix(in_srgb,_var(--color-text-primary)_4%,_transparent)]"
                >
                  <td className="px-3 py-2">{item.kind}</td>
                  <td className="px-3 py-2 font-medium text-[var(--color-text-primary)]">
                    {item.name}
                  </td>
                  <td className="px-3 py-2">
                    {item.ready_replicas}/{item.desired_replicas}
                  </td>
                  <td className="px-3 py-2">{item.status}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <div className="mt-4 space-y-2">
          {(scope === "all"
            ? (monitoring?.oss_statuses ?? [])
            : activeTool
              ? [activeTool]
              : []
          ).map((tool) => (
            <div
              key={`pods-${tool.key}`}
              className="rounded-[10px] border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] p-3"
            >
              <div className="mb-2 flex items-center justify-between">
                <div className="text-sm font-semibold text-[var(--color-text-primary)]">
                  {tool.name} Pod Details
                </div>
                <div className="text-xs text-[var(--color-text-secondary)]">
                  ready {tool.ready_pods}/{tool.pod_count}
                </div>
              </div>
              <div className="overflow-x-auto">
                <table className="min-w-full text-left text-[12px] text-[var(--color-text-secondary)]">
                  <thead>
                    <tr className="border-b border-[var(--color-border-default)] text-[11px] uppercase tracking-[0.05em]">
                      <th className="py-1 pr-3">Pod</th>
                      <th className="py-1 pr-3">Status</th>
                      <th className="py-1 pr-3">Ready</th>
                      <th className="py-1 pr-3">CPU Req/Limit</th>
                      <th className="py-1 pr-3">Mem Req/Limit</th>
                      <th className="py-1 pr-3">Restarts</th>
                    </tr>
                  </thead>
                  <tbody>
                    {tool.pods.map((pod) => (
                      <tr
                        key={pod.name}
                        className="border-b border-[color-mix(in_srgb,_var(--color-text-primary)_4%,_transparent)]"
                      >
                        <td className="py-1 pr-3 font-medium text-[var(--color-text-primary)]">
                          {pod.name}
                        </td>
                        <td className="py-1 pr-3">{pod.status}</td>
                        <td className="py-1 pr-3">
                          {pod.ready ? "yes" : "no"}
                        </td>
                        <td className="py-1 pr-3">
                          {pod.cpu_request_millicores}m /{" "}
                          {pod.cpu_limit_millicores}m
                        </td>
                        <td className="py-1 pr-3">
                          {pod.memory_request_mib}Mi / {pod.memory_limit_mib}Mi
                        </td>
                        <td className="py-1 pr-3">{pod.restart_count}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
