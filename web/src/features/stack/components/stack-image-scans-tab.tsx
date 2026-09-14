// 스택에 설치된 OSS 이미지의 취약점 보고.
//
// 보고용이다 — 설치를 막은 적이 없다. 파이프라인 이미지 스캔은 정책 게이트로 배포를
// 막지만, 이 탭은 이미 설치된 OSS 이미지(GitLab, PostgreSQL …)가 어떤 취약점을 안고
// 있는지 알려 줄 뿐이다. 그래서 차단·통과 같은 판정 배지를 쓰지 않는다.
//
// "스캔하지 않음" 과 "취약점 0" 을 섞지 않는 것이 이 화면의 핵심이다. Trivy 를 고르지 않은
// 스택이나 폐쇄망 설치는 스캔 자체를 안 한다 — 그때 요약 자리에 0 을 그리면 안전한 스택으로
// 읽힌다. 건수를 모르는 항목도 같은 이유로 0 대신 "건수 모름" 을 보여준다.
//
// 이미지는 그리드로 보인다. 스택 하나가 수십 개의 이미지를 돌린다(GitLab 스택 33개) —
// 카드를 세로로 쌓으면 어느 이미지가 더 위험한지 건수를 나란히 비교할 수 없다.

import { useMemo, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { CircleAlert, Info, ShieldAlert } from "lucide-react";
import { iconProps } from "../../../components/ui/icon";
import { Badge } from "../../../components/ui/badge";
import { StatusIcon } from "../../../components/ui/status-icon";
import { SeverityCounts } from "../../../components/shared/severity-counts";
import { tableHeadRowClass, tdClass, thClass } from "../../../components/shared/table-chrome";
import { formatDate, formatDateTime, resolveLocale } from "../../../lib/locale";
import { cn } from "../../../lib/utils";
import { shortImageDigest } from "../../../lib/vulnerability-counts";
import type { VulnerabilityCounts } from "../../../types";
import { useStackImageScans } from "../api/stack-api";
import type {
  StackImageScanItem,
  StackImageScanReport,
} from "../api/stack-api-types";
import { sortStackImageScanItems } from "../utils/image-scan-report";

const WARNING_BADGE =
  "bg-[color-mix(in_srgb,_var(--color-warning)_15%,_transparent)] text-[var(--color-warning)]";
// 스캔 실패는 취약점이 아니다. 위험색을 쓰면 실패한 이미지가 가장 위험해 보인다.
const NEUTRAL_BADGE =
  "bg-[color-mix(in_srgb,_var(--color-text-secondary)_15%,_transparent)] text-[var(--color-text-secondary)]";

// 건수 열. 색은 건수가 있는 칸에만 준다 — 모든 칸이 빨강이면 색이 뜻을 잃는다.
const SEVERITY_COLUMNS: { key: keyof VulnerabilityCounts; activeClass: string }[] = [
  { key: "critical", activeClass: "font-semibold text-[var(--color-error)]" },
  { key: "high", activeClass: "font-semibold text-[var(--color-warning)]" },
  { key: "medium", activeClass: "text-[var(--color-text-primary)]" },
  { key: "low", activeClass: "text-[var(--color-text-primary)]" },
  { key: "unknown", activeClass: "text-[var(--color-text-primary)]" },
];
// 건수 열 전부와 수정 가능 열. 실패·건수 모름 행은 이 칸들을 합쳐 한 번만 설명한다.
const COUNT_SPAN = SEVERITY_COLUMNS.length + 1;

// 열 너비(px). 이미지 열은 정하지 않는다 — 남는 폭을 전부 가져간다.
//
// 브라우저에 맡기면 내용 길이로 폭을 나눈다. 워크로드 이름이 긴 릴리스 열이 이미지보다
// 넓어지고, 레지스트리 경로가 긴 이미지 이름은 두세 줄로 접히고, 상태 열은 "스캔됨" 이
// 두 줄로 꺾일 만큼(약 50px) 좁아졌다.
//
// 기준은 표 폭 약 1080px 화면이다. 릴리스는 가장 긴 워크로드 이름
// (gitlab-sidekiq-all-in-1-v2)이 한 줄에 들어가는 폭, 상태는 그때의 1.5배다.
// 건수 열은 한국어 머리글과 네 자리 숫자가 한 줄에 들어가는 폭이다.
const COLUMN_WIDTH: Record<string, number> = {
  release: 200,
  critical: 64,
  high: 72,
  medium: 64,
  low: 64,
  unknown: 96,
  fixable: 88,
  status: 75,
};
// 이미지 열이 이보다 좁아지면 표 전체가 가로로 스크롤된다. 기준 화면에서는 스크롤이 없다.
const IMAGE_MIN_WIDTH = 320;
const TABLE_MIN_WIDTH =
  IMAGE_MIN_WIDTH + Object.values(COLUMN_WIDTH).reduce((sum, w) => sum + w, 0);

function Notice({ children }: { children: ReactNode }) {
  return (
    <div className="flex items-start gap-2 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-surface-sunken)] px-3 py-2.5 text-[12px] text-[var(--color-text-secondary)]">
      <Info {...iconProps("xs")} className="mt-0.5 shrink-0" />
      <span>{children}</span>
    </div>
  );
}

// 결과가 없는 이유를 말하는 문구. scanned 는 결과 자체가 말하므로 문구가 없다.
function statusNoticeKey(report: StackImageScanReport): string | null {
  if (report.status === "pending") return "stackList.imageScans.pending";
  if (report.status !== "not_scanned") return null;
  switch (report.reason) {
    case "scanner_not_installed":
      return "stackList.imageScans.notScanned.scannerNotInstalled";
    case "airgap":
      return "stackList.imageScans.notScanned.airgap";
    default:
      return "stackList.imageScans.notScanned.unknown";
  }
}

// 스캐너 버전과 취약점 DB 날짜는 한 번의 스캔에서 모든 이미지가 같다. 행마다 되풀이하지
// 않고 요약에 한 번 둔다. 드물게 섞여 있으면 모두 적는다.
function scanSource(items: StackImageScanItem[], locale: string, unknown: string) {
  const scanners = [...new Set(items.map((i) => i.scannerVersion).filter(Boolean))];
  const dbDates = items.map((i) => i.dbUpdatedAt).filter((d): d is string => Boolean(d)).sort();
  return {
    scanner: scanners.length > 0 ? scanners.map((v) => `trivy ${v}`).join(", ") : unknown,
    db: dbDates.length > 0 ? formatDate(dbDates[dbDates.length - 1], locale) : unknown,
  };
}

function ImageScanRow({ item }: { item: StackImageScanItem }) {
  const { t } = useTranslation();
  const failed = item.status === "failed";
  const counts = failed ? undefined : item.counts;
  const fixable = counts ? item.fixableCounts : undefined;
  const fixableTotal = fixable
    ? fixable.critical + fixable.high + fixable.medium + fixable.low + fixable.unknown
    : undefined;

  return (
    <tr data-testid="stack-image-scan-item" className="align-top">
      <td className={tdClass}>
        <span
          data-testid="stack-image-scan-image"
          className="block break-all font-mono text-[12px] font-semibold text-[var(--color-text-primary)]"
        >
          {item.image}
        </span>
        {item.imageDigest && (
          <code
            title={item.imageDigest}
            className="mt-0.5 block font-mono text-[11px] text-[var(--color-text-secondary)]"
          >
            {shortImageDigest(item.imageDigest)}
          </code>
        )}
      </td>
      <td className={tdClass}>
        <span className="block text-[12px] text-[var(--color-text-primary)]">{item.release ?? "-"}</span>
        {/* 공통 베이스 이미지는 워크로드 여러 개가 함께 쓴다(gitlab-base 7개, argocd 6개).
            좁은 릴리스 열에서 전부 펼치면 행이 여덟 줄로 늘어나므로 두 줄로 줄이고 전체는 툴팁에 둔다. */}
        {item.workloads.length > 0 && (
          <span
            title={item.workloads.join(", ")}
            className="mt-0.5 line-clamp-2 break-all font-mono text-[11px] text-[var(--color-text-secondary)]"
          >
            {item.workloads.join(", ")}
          </span>
        )}
      </td>

      {failed ? (
        // 실패한 스캔의 건수는 없다. 건수 칸을 합쳐 오류를 대신 보여준다.
        <td colSpan={COUNT_SPAN} className={cn(tdClass, "break-all text-[12px] text-[var(--color-text-secondary)]")}>
          {item.error || t("stackList.imageScans.item.failedNoReason")}
        </td>
      ) : !counts ? (
        <td colSpan={COUNT_SPAN} className={tdClass}>
          <SeverityCounts counts={undefined} />
        </td>
      ) : (
        <>
          {SEVERITY_COLUMNS.map(({ key, activeClass }) => {
            const label = `${t(`common.vulnerability.${key}`)} ${counts[key]}`;
            return (
              <td key={key} className={cn(tdClass, "text-right tabular-nums")}>
                <span
                  aria-label={label}
                  title={label}
                  className={counts[key] > 0 ? activeClass : "text-[var(--color-text-muted)]"}
                >
                  {counts[key]}
                </span>
              </td>
            );
          })}
          <td className={cn(tdClass, "text-right tabular-nums")}>
            {fixable ? (
              <span title={t("stackList.imageScans.item.fixable", { ...fixable })}>{fixableTotal}</span>
            ) : (
              "-"
            )}
          </td>
        </>
      )}

      <td className={tdClass}>
        <span className="inline-flex flex-wrap items-center gap-1">
          {failed ? (
            // 상태 열은 좁다. 배지 글자는 칸을 넘치지 않고 줄바꿈한다.
            <Badge pill className={cn(NEUTRAL_BADGE, "whitespace-normal text-left")}>
              <CircleAlert {...iconProps("xs")} className="shrink-0" />
              {t("stackList.imageScans.item.failed")}
            </Badge>
          ) : (
            <span className="text-[12px] text-[var(--color-text-secondary)]">
              {t("stackList.imageScans.item.scanned")}
            </span>
          )}
          {item.dbStale && (
            <Badge
              pill
              className={cn(WARNING_BADGE, "whitespace-normal text-left")}
              title={t("stackList.imageScans.dbStaleHint")}
            >
              <StatusIcon tone="warning" size="xs" inheritColor className="shrink-0" />
              {t("stackList.imageScans.dbStale")}
            </Badge>
          )}
        </span>
      </td>
    </tr>
  );
}

function ImageScanGrid({ items }: { items: StackImageScanItem[] }) {
  const { t } = useTranslation();
  const headers = [
    { key: "image", label: t("stackList.imageScans.columns.image"), numeric: false },
    { key: "release", label: t("stackList.imageScans.columns.release"), numeric: false },
    ...SEVERITY_COLUMNS.map(({ key }) => ({
      key,
      label: t(`common.vulnerability.${key}`),
      numeric: true,
    })),
    { key: "fixable", label: t("stackList.imageScans.columns.fixable"), numeric: true },
    { key: "status", label: t("stackList.imageScans.columns.status"), numeric: false },
  ];

  return (
    // 좁은 화면에서는 표가 가로로 스크롤된다. 열을 접으면 건수를 나란히 비교할 수 없다.
    <div className="overflow-x-auto rounded-lg border border-[var(--color-border-default)]">
      <table className="w-full table-fixed border-collapse" style={{ minWidth: TABLE_MIN_WIDTH }}>
        <colgroup>
          {headers.map((h) => (
            <col key={h.key} style={COLUMN_WIDTH[h.key] ? { width: COLUMN_WIDTH[h.key] } : undefined} />
          ))}
        </colgroup>
        <thead>
          <tr className={tableHeadRowClass}>
            {headers.map((h) => (
              <th key={h.key} scope="col" className={cn(thClass, h.numeric && "text-right")}>
                {h.label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {items.map((item, index) => (
            <ImageScanRow key={`${item.release ?? ""}|${item.image}|${index}`} item={item} />
          ))}
        </tbody>
      </table>
    </div>
  );
}

function ReportBody({
  report,
  items,
  locale,
}: {
  report: StackImageScanReport;
  items: StackImageScanItem[];
  locale: string;
}) {
  const { t } = useTranslation();
  const noticeKey = statusNoticeKey(report);
  // not_scanned 인데 요약이 실려 와도 그리지 않는다. 스캔하지 않은 스택에 숫자가
  // 보이면 그 숫자를 믿게 된다.
  const summary = report.status === "not_scanned" ? undefined : report.summary;
  const source = scanSource(items, locale, t("stackList.imageScans.item.unknown"));

  return (
    <>
      {noticeKey && <Notice>{t(noticeKey)}</Notice>}

      {summary && (
        <div
          data-testid="stack-image-scan-summary"
          className="rounded-lg border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] p-3"
        >
          <div className="flex flex-wrap items-center gap-2">
            <SeverityCounts counts={summary} />
            {report.lastScannedAt && (
              <span className="ml-auto text-[12px] text-[var(--color-text-secondary)]">
                {t("stackList.imageScans.lastScannedAt", {
                  time: formatDateTime(report.lastScannedAt, locale),
                })}
              </span>
            )}
          </div>
          <p className="mb-0 mt-2 text-[11px] text-[var(--color-text-secondary)]">
            {t("stackList.imageScans.summaryHint")}
          </p>
          {items.length > 0 && (
            <p className="mb-0 mt-1 text-[11px] text-[var(--color-text-secondary)]">
              {t("stackList.imageScans.summaryScanner", source)}
            </p>
          )}
        </div>
      )}

      {report.status !== "not_scanned" && items.length > 0 && <ImageScanGrid items={items} />}

      {report.status === "scanned" && items.length === 0 && (
        <Notice>{t("stackList.imageScans.empty")}</Notice>
      )}
    </>
  );
}

export function StackImageScansTab({ stackId }: { stackId: string }) {
  const { t, i18n } = useTranslation();
  const locale = resolveLocale(i18n.resolvedLanguage || i18n.language);
  const { data, isLoading, isError } = useStackImageScans(stackId);
  const items = useMemo(
    () => sortStackImageScanItems(data?.items ?? []),
    [data],
  );

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <ShieldAlert {...iconProps("sm")} className="text-[var(--color-primary)]" />
        <h3 className="m-0 text-[14px] font-bold text-[var(--color-text-primary)]">
          {t("stackList.imageScans.title")}
        </h3>
      </div>
      <p className="m-0 text-[12px] text-[var(--color-text-secondary)]">
        {t("stackList.imageScans.reportOnly")}
      </p>

      {/* 404(모르는 스택)·503(스캐너 미배선)은 오류로 온다. 화면을 깨지 않고 중립 안내로 끝낸다. */}
      {isLoading ? (
        <Notice>{t("stackList.imageScans.loading")}</Notice>
      ) : isError || !data ? (
        <Notice>{t("stackList.imageScans.noData")}</Notice>
      ) : (
        <ReportBody report={data} items={items} locale={locale} />
      )}
    </div>
  );
}
