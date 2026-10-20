// 파이프라인 실행의 의존성 취약점 스캔(Trivy fs) 결과.
//
// 이미지 스캔과 같은 틀이다 — 판정 · 심각도 건수 · DB 신선도. 다른 점은 검사 대상 수다.
// 스캐폴딩은 락파일을 만들지 않아 대부분의 앱이 "0 개 검사 · 통과" 다. 그걸 초록 통과로
// 보이면 아무것도 검사하지 않은 앱이 안전한 앱으로 읽힌다 — 그래서 대상이 0 이면 "대상 없음"
// 으로 보이고 이유를 적는다.

import { useTranslation } from "react-i18next";
import { ExternalLink } from "lucide-react";
import { iconProps } from "../../../components/ui/icon";
import { DependencyScanGateBadge } from "../../../components/shared/gate-badge";
import { SeverityCounts } from "../../../components/shared/severity-counts";
import { formatDate, formatDateTime } from "../../../lib/locale";
import type { PipelineDependencyScan } from "../../../types";

// report_uri 는 CI 가 준 값이다. http(s) 가 아닌 값(javascript: 등)은 링크로 만들지 않는다.
function reportHref(uri: string | undefined): string | undefined {
  return uri && /^https?:\/\//i.test(uri) ? uri : undefined;
}

const noTargets = (scan: PipelineDependencyScan) =>
  scan.gateResult === "pass" && scan.targets === 0;

/** 실행 목록 한 줄에 들어가는 요약. 다른 단계 배지와 나란히 붙으므로 이름을 단다. */
export function DependencyScanRowSummary({ scan }: { scan: PipelineDependencyScan }) {
  const { t } = useTranslation();
  return (
    <span className="inline-flex flex-wrap items-center gap-1">
      <span className="text-[11px] font-semibold text-[var(--color-text-secondary)]">
        {t("cicdListPage.dependencyScan.label")}
      </span>
      <DependencyScanGateBadge scan={scan} />
      {/* 대상이 없으면 건수도 없다 — 0 칩을 늘어놓지 않는다. */}
      {!noTargets(scan) && <SeverityCounts compact counts={scan.counts} />}
    </span>
  );
}

/** 선택한 실행의 상세. 판정의 근거(건수 · 검사 대상 · 스캐너 · DB 날짜)와 원본 리포트. */
export function DependencyScanDetail({
  scan,
  locale,
}: {
  scan: PipelineDependencyScan;
  locale: string;
}) {
  const { t } = useTranslation();
  const href = reportHref(scan.reportUri);
  const scanner = [scan.scanner, scan.scannerVersion].filter(Boolean).join(" ") || "-";
  const labelClass = "text-[var(--color-text-secondary)]";
  const valueClass = "m-0 min-w-0 text-[var(--color-text-primary)]";

  return (
    <div className="mt-3 rounded-lg border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] p-2">
      <div className="mb-2 flex flex-wrap items-center gap-2">
        <span className="text-[12px] font-semibold text-[var(--color-text-primary)]">
          {t("cicdListPage.dependencyScan.title")}
        </span>
        <DependencyScanGateBadge scan={scan} />
        {href && (
          <a
            href={href}
            target="_blank"
            rel="noopener noreferrer"
            className="ml-auto inline-flex items-center gap-1 text-[12px] text-[var(--color-primary)] hover:underline"
          >
            {t("cicdListPage.imageScan.viewReport")}
            <ExternalLink {...iconProps("xs")} />
          </a>
        )}
      </div>

      {scan.gateResult === "error" && (
        <p className="mb-2 mt-0 text-[11px] text-[var(--color-text-secondary)]">
          {t("cicdListPage.imageScan.errorHint")}
        </p>
      )}
      {noTargets(scan) && (
        <p className="mb-2 mt-0 text-[11px] text-[var(--color-text-secondary)]">
          {t("cicdListPage.dependencyScan.noTargetsHint")}
        </p>
      )}
      {scan.dbStale && (
        <p className="mb-2 mt-0 text-[11px] text-[var(--color-warning)]">
          {t("cicdListPage.imageScan.dbStaleHint")}
        </p>
      )}

      {!noTargets(scan) && <SeverityCounts counts={scan.counts} />}

      <dl className="mb-0 mt-2 grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1 text-[12px]">
        <dt className={labelClass}>{t("cicdListPage.dependencyScan.targets")}</dt>
        <dd className={valueClass}>
          {scan.targets ?? t("cicdListPage.imageScan.unknown")}
        </dd>
        <dt className={labelClass}>{t("cicdListPage.imageScan.scanner")}</dt>
        <dd className={valueClass}>{scanner}</dd>
        <dt className={labelClass}>{t("cicdListPage.imageScan.dbUpdatedAt")}</dt>
        <dd className={valueClass}>
          {scan.dbUpdatedAt
            ? formatDate(scan.dbUpdatedAt, locale)
            : t("cicdListPage.imageScan.unknown")}
        </dd>
        <dt className={labelClass}>{t("cicdListPage.imageScan.scannedAt")}</dt>
        <dd className={valueClass}>
          {scan.scannedAt ? formatDateTime(scan.scannedAt, locale) : "-"}
        </dd>
      </dl>
    </div>
  );
}
