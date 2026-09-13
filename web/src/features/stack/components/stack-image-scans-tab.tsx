// 스택에 설치된 OSS 이미지의 취약점 보고.
//
// 보고용이다 — 설치를 막은 적이 없다. 파이프라인 이미지 스캔은 정책 게이트로 배포를
// 막지만, 이 탭은 이미 설치된 OSS 이미지(GitLab, PostgreSQL …)가 어떤 취약점을 안고
// 있는지 알려 줄 뿐이다. 그래서 차단·통과 같은 판정 배지를 쓰지 않는다.
//
// "스캔하지 않음" 과 "취약점 0" 을 섞지 않는 것이 이 화면의 핵심이다. Trivy 를 고르지 않은
// 스택이나 폐쇄망 설치는 스캔 자체를 안 한다 — 그때 요약 자리에 0 을 그리면 안전한 스택으로
// 읽힌다. 건수를 모르는 항목도 같은 이유로 0 대신 "건수 모름" 을 보여준다.

import { useMemo, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { CircleAlert, Info, ShieldAlert, TriangleAlert } from "lucide-react";
import { iconProps } from "../../../components/ui/icon";
import { Badge } from "../../../components/ui/badge";
import { SeverityCounts } from "../../../components/shared/severity-counts";
import { formatDate, formatDateTime, resolveLocale } from "../../../lib/locale";
import { shortImageDigest } from "../../../lib/vulnerability-counts";
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

function ImageScanItemCard({
  item,
  locale,
}: {
  item: StackImageScanItem;
  locale: string;
}) {
  const { t } = useTranslation();
  const failed = item.status === "failed";
  const labelClass = "text-[var(--color-text-secondary)]";
  const valueClass = "m-0 min-w-0 text-[var(--color-text-primary)]";

  return (
    <li
      data-testid="stack-image-scan-item"
      className="rounded-lg border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] p-3"
    >
      <div className="flex flex-wrap items-center gap-2">
        <span
          data-testid="stack-image-scan-image"
          className="min-w-0 break-all font-mono text-[12px] font-semibold text-[var(--color-text-primary)]"
        >
          {item.image}
        </span>
        {failed && (
          <Badge pill className={NEUTRAL_BADGE}>
            <CircleAlert {...iconProps("xs")} />
            {t("stackList.imageScans.item.failed")}
          </Badge>
        )}
        {item.dbStale && (
          <Badge pill className={WARNING_BADGE}>
            <TriangleAlert {...iconProps("xs")} />
            {t("stackList.imageScans.dbStale")}
          </Badge>
        )}
      </div>

      {/* 실패한 스캔의 건수는 없다. 오류를 대신 보여준다. */}
      {failed ? (
        <p className="mb-0 mt-2 break-all text-[12px] text-[var(--color-text-secondary)]">
          {item.error || t("stackList.imageScans.item.failedNoReason")}
        </p>
      ) : (
        <div className="mt-2 flex flex-wrap items-center gap-2">
          <SeverityCounts counts={item.counts} />
          {item.counts && item.fixableCounts && (
            <span className="text-[11px] text-[var(--color-text-secondary)]">
              {t("stackList.imageScans.item.fixable", { ...item.fixableCounts })}
            </span>
          )}
        </div>
      )}

      <dl className="mb-0 mt-2 grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1 text-[12px]">
        <dt className={labelClass}>{t("stackList.imageScans.item.release")}</dt>
        <dd className={valueClass}>{item.release ?? "-"}</dd>
        <dt className={labelClass}>{t("stackList.imageScans.item.workloads")}</dt>
        <dd className={`${valueClass} break-all font-mono`}>
          {item.workloads.length > 0 ? item.workloads.join(", ") : "-"}
        </dd>
        <dt className={labelClass}>{t("stackList.imageScans.item.digest")}</dt>
        <dd className={valueClass}>
          {item.imageDigest ? (
            <code title={item.imageDigest} className="font-mono">
              {shortImageDigest(item.imageDigest)}
            </code>
          ) : (
            "-"
          )}
        </dd>
        <dt className={labelClass}>{t("stackList.imageScans.item.scanner")}</dt>
        {/* 설치 이미지 스캔은 스택의 Trivy 로만 돈다(reason 이 scanner_not_installed 인 이유). */}
        <dd className={valueClass}>
          {item.scannerVersion ? `trivy ${item.scannerVersion}` : "-"}
        </dd>
        <dt className={labelClass}>{t("stackList.imageScans.item.dbUpdatedAt")}</dt>
        <dd className={valueClass}>
          {item.dbUpdatedAt
            ? formatDate(item.dbUpdatedAt, locale)
            : t("stackList.imageScans.item.unknown")}
          {item.dbStale && (
            <span className="ml-2 text-[11px] text-[var(--color-warning)]">
              {t("stackList.imageScans.dbStaleHint")}
            </span>
          )}
        </dd>
        <dt className={labelClass}>{t("stackList.imageScans.item.scannedAt")}</dt>
        <dd className={valueClass}>
          {item.scannedAt ? formatDateTime(item.scannedAt, locale) : "-"}
        </dd>
      </dl>
    </li>
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
        </div>
      )}

      {report.status !== "not_scanned" && items.length > 0 && (
        <ul className="m-0 flex list-none flex-col gap-2 p-0">
          {items.map((item, index) => (
            <ImageScanItemCard
              key={`${item.release ?? ""}|${item.image}|${index}`}
              item={item}
              locale={locale}
            />
          ))}
        </ul>
      )}

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
