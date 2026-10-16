// 파이프라인 실행의 소스 정적 분석(SonarQube) 결과.
//
// 실행 이력의 SAST 단계 상태만으로는 무엇에 걸렸는지, 경고 정책이 통과시킨 실패인지, 분석기가
// 죽어서 실패한 것인지 알 수 없다. 여기서는 판정과 그 근거(통과하지 못한 조건, 지표)를 드러낸다.
//
// 색 규칙은 이미지 스캔과 같다 — error(분석 실패)는 코드 판정이 아니라서 중립색이고, 지표를
// 모르면 0 을 그리지 않는다(0 은 "문제 0건" 으로 읽힌다).

import { useTranslation } from "react-i18next";
import {
  CircleAlert,
  ExternalLink,
  ShieldAlert,
  ShieldCheck,
  ShieldX,
  type LucideIcon,
} from "lucide-react";
import { iconProps } from "../../../components/ui/icon";
import { Badge } from "../../../components/ui/badge";
import { formatDateTime } from "../../../lib/locale";
import type {
  ImageScanGateResult,
  PipelineSASTResult,
  SASTCondition,
  SASTMetrics,
} from "../../../types";
import { TONE_CLASS } from "./image-scan-summary";

const GATE_META: Record<
  ImageScanGateResult,
  { labelKey: string; tone: keyof typeof TONE_CLASS; icon: LucideIcon }
> = {
  pass: { labelKey: "cicdListPage.sast.gate.pass", tone: "success", icon: ShieldCheck },
  warn: { labelKey: "cicdListPage.sast.gate.warn", tone: "warning", icon: ShieldAlert },
  block: { labelKey: "cicdListPage.sast.gate.block", tone: "danger", icon: ShieldX },
  error: { labelKey: "cicdListPage.sast.gate.error", tone: "neutral", icon: CircleAlert },
};

// 지표 표시 순서. 앞의 넷은 건수, 뒤의 둘은 백분율, 마지막은 코드 줄 수다.
const METRICS: {
  key: keyof SASTMetrics;
  labelKey: string;
  percent?: boolean;
}[] = [
  { key: "vulnerabilities", labelKey: "cicdListPage.sast.metrics.vulnerabilities" },
  { key: "bugs", labelKey: "cicdListPage.sast.metrics.bugs" },
  { key: "securityHotspots", labelKey: "cicdListPage.sast.metrics.securityHotspots" },
  { key: "codeSmells", labelKey: "cicdListPage.sast.metrics.codeSmells" },
  { key: "coverage", labelKey: "cicdListPage.sast.metrics.coverage", percent: true },
  { key: "duplicatedLinesDensity", labelKey: "cicdListPage.sast.metrics.duplications", percent: true },
  { key: "ncloc", labelKey: "cicdListPage.sast.metrics.ncloc" },
];

const RATING_LETTERS = ["A", "B", "C", "D", "E"];

// dashboard_url 은 CI 가 남긴 리포트에서 온다. http(s) 가 아닌 값(javascript: 등)은 링크로 만들지 않는다.
function dashboardHref(uri: string | undefined): string | undefined {
  return uri && /^https?:\/\//i.test(uri) ? uri : undefined;
}

// 등급 지표(…_rating)는 SonarQube 화면처럼 문자로 보인다(1 → A … 5 → E).
function conditionValue(metric: string, value: string | undefined): string {
  if (value === undefined || value === "") return "-";
  if (metric.endsWith("_rating")) {
    const n = Math.round(Number(value));
    if (n >= 1 && n <= RATING_LETTERS.length) return RATING_LETTERS[n - 1];
  }
  if (
    metric.endsWith("coverage") ||
    metric.endsWith("_density") ||
    metric.endsWith("hotspots_reviewed")
  ) {
    return `${value}%`;
  }
  return value;
}

function comparatorSymbol(comparator: string | undefined): string {
  switch (comparator) {
    case "GT":
      return ">";
    case "LT":
      return "<";
    default:
      return comparator ?? "";
  }
}

export function SASTGateBadge({ result }: { result: PipelineSASTResult }) {
  const { t } = useTranslation();
  const meta = GATE_META[result.gateResult] ?? GATE_META.error;
  const Icon = meta.icon;
  return (
    <Badge pill className={TONE_CLASS[meta.tone]}>
      <Icon {...iconProps("xs")} />
      {t(meta.labelKey)}
    </Badge>
  );
}

/** 실행 목록 한 줄에 들어가는 요약. 이미지 스캔 배지와 나란히 붙으므로 이름을 단다. */
export function SASTRowSummary({ result }: { result: PipelineSASTResult }) {
  const { t } = useTranslation();
  return (
    <span className="inline-flex items-center gap-1">
      <span className="text-[11px] font-semibold text-[var(--color-text-secondary)]">
        {t("cicdListPage.sast.label")}
      </span>
      <SASTGateBadge result={result} />
    </span>
  );
}

function FailedConditions({ conditions }: { conditions: SASTCondition[] }) {
  const { t } = useTranslation();
  const failed = conditions.filter((c) => c.status === "ERROR");
  if (failed.length === 0) return null;
  const label = t("cicdListPage.sast.failedConditions");
  return (
    <div className="mb-2">
      <div className="mb-1 text-[12px] font-semibold text-[var(--color-text-primary)]">{label}</div>
      <ul aria-label={label} className="m-0 flex list-none flex-col gap-0.5 p-0 text-[12px]">
        {failed.map((c) => {
          // 모르는 지표(사용자가 Quality Gate 에 더한 것)는 키 그대로 보인다.
          const name = t([`cicdListPage.sast.metricNames.${c.metric}`], {
            defaultValue: c.metric,
          });
          return (
            <li key={c.metric} className="flex flex-wrap items-baseline gap-x-2">
              <span className="text-[var(--color-text-primary)]">
                {name}
              </span>
              <span className="font-mono text-[var(--color-error)]">
                {conditionValue(c.metric, c.actual)}
              </span>
              <span className="text-[var(--color-text-secondary)]">
                ({t("cicdListPage.sast.failsWhen")} {comparatorSymbol(c.comparator)}{" "}
                {conditionValue(c.metric, c.threshold)})
              </span>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

function MetricsGrid({ metrics }: { metrics: SASTMetrics | undefined }) {
  const { t } = useTranslation();
  const shown = metrics ? METRICS.filter((m) => metrics[m.key] !== undefined) : [];
  if (!metrics || shown.length === 0) {
    return (
      <p className="m-0 text-[12px] text-[var(--color-text-secondary)]">
        {t("cicdListPage.sast.metricsUnknown")}
      </p>
    );
  }
  return (
    <div className="flex flex-wrap gap-1.5">
      {shown.map((m) => {
        const value = `${metrics[m.key]}${m.percent ? "%" : ""}`;
        const label = t(m.labelKey);
        return (
          <span
            key={m.key}
            aria-label={`${label} ${value}`}
            className="inline-flex items-center gap-1 rounded-md border border-[var(--color-border-default)] px-1.5 py-0.5 text-[11px]"
          >
            <span className="text-[var(--color-text-secondary)]">{label}</span>
            <span className="font-semibold text-[var(--color-text-primary)]">{value}</span>
          </span>
        );
      })}
    </div>
  );
}

/** 선택한 실행의 상세. 판정의 근거(통과하지 못한 조건·지표)와 SonarQube 화면 링크를 보여준다. */
export function SASTResultDetail({
  result,
  locale,
}: {
  result: PipelineSASTResult;
  locale: string;
}) {
  const { t } = useTranslation();
  const href = dashboardHref(result.dashboardUrl);
  return (
    <div className="mt-3 rounded-lg border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] p-2">
      <div className="mb-2 flex flex-wrap items-center gap-2">
        <span className="text-[12px] font-semibold text-[var(--color-text-primary)]">
          {t("cicdListPage.sast.title")}
        </span>
        <SASTGateBadge result={result} />
        {href && (
          <a
            href={href}
            target="_blank"
            rel="noopener noreferrer"
            className="ml-auto inline-flex items-center gap-1 text-[12px] text-[var(--color-primary)] hover:underline"
          >
            {t("cicdListPage.sast.openInSonarQube")}
            <ExternalLink {...iconProps("xs")} />
          </a>
        )}
      </div>

      {result.gateResult === "error" && (
        <p className="mb-2 mt-0 text-[11px] text-[var(--color-text-secondary)]">
          {t("cicdListPage.sast.errorHint")}
        </p>
      )}
      {result.gateResult === "warn" && (
        <p className="mb-2 mt-0 text-[11px] text-[var(--color-warning)]">
          {t("cicdListPage.sast.warnHint")}
        </p>
      )}

      <FailedConditions conditions={result.conditions} />
      <MetricsGrid metrics={result.metrics} />

      <dl className="mb-0 mt-2 grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1 text-[12px]">
        <dt className="text-[var(--color-text-secondary)]">{t("cicdListPage.sast.analyzedAt")}</dt>
        <dd className="m-0 min-w-0 text-[var(--color-text-primary)]">
          {result.analyzedAt ? formatDateTime(result.analyzedAt, locale) : "-"}
        </dd>
      </dl>
    </div>
  );
}
