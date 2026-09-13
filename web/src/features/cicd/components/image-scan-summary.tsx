// 파이프라인 실행이 만든 이미지의 스캔 판정.
//
// 예전 실행 이력은 ImageScan 단계가 끝났는지만 보여줬다. 단계가 "완료" 라는 것과
// 이미지가 정책을 통과했다는 것은 다른 이야기다 — 차단(block)·경고(warn)·스캔 오류(error)가
// 모두 같은 완료로 보였다. 여기서는 판정을 그대로 드러낸다.
//
// 색 규칙
//   - block 은 위험색, warn 은 경고색, pass 는 성공색이다.
//   - error 는 스캔을 하지 못했다는 뜻이라 중립색이다. 초록이면 통과로, 빨강이면
//     취약점 발견으로 읽힌다 — 둘 다 사실이 아니다.
//   - 취약점 DB 가 오래됐으면(db_stale) pass 라도 초록을 쓰지 않고 "DB 오래됨" 을 붙인다.
//     최근 취약점을 모르는 DB 로 얻은 통과는 깨끗한 통과가 아니다.

import { useTranslation } from "react-i18next";
import {
  CircleAlert,
  ExternalLink,
  ShieldAlert,
  ShieldCheck,
  ShieldX,
  TriangleAlert,
  type LucideIcon,
} from "lucide-react";
import { iconProps } from "../../../components/ui/icon";
import { Badge } from "../../../components/ui/badge";
import { SeverityCounts } from "../../../components/shared/severity-counts";
import { formatDate, formatDateTime } from "../../../lib/locale";
import { shortImageDigest } from "../../../lib/vulnerability-counts";
import type { ImageScanGateResult, PipelineImageScan } from "../../../types";

const TONE_CLASS = {
  success:
    "bg-[color-mix(in_srgb,_var(--color-success)_15%,_transparent)] text-[var(--color-success)]",
  warning:
    "bg-[color-mix(in_srgb,_var(--color-warning)_15%,_transparent)] text-[var(--color-warning)]",
  danger:
    "bg-[color-mix(in_srgb,_var(--color-error)_15%,_transparent)] text-[var(--color-error)]",
  neutral:
    "bg-[color-mix(in_srgb,_var(--color-text-secondary)_15%,_transparent)] text-[var(--color-text-secondary)]",
} as const;

const GATE_META: Record<
  ImageScanGateResult,
  { labelKey: string; tone: keyof typeof TONE_CLASS; icon: LucideIcon }
> = {
  pass: { labelKey: "cicdListPage.imageScan.gate.pass", tone: "success", icon: ShieldCheck },
  warn: { labelKey: "cicdListPage.imageScan.gate.warn", tone: "warning", icon: ShieldAlert },
  block: { labelKey: "cicdListPage.imageScan.gate.block", tone: "danger", icon: ShieldX },
  error: { labelKey: "cicdListPage.imageScan.gate.error", tone: "neutral", icon: CircleAlert },
};

// report_uri 는 CI 가 준 값이다. http(s) 가 아닌 값(javascript: 등)은 링크로 만들지 않는다.
function reportHref(uri: string | undefined): string | undefined {
  return uri && /^https?:\/\//i.test(uri) ? uri : undefined;
}

export function ImageScanGateBadge({ scan }: { scan: PipelineImageScan }) {
  const { t } = useTranslation();
  const meta = GATE_META[scan.gateResult] ?? GATE_META.error;
  const tone = scan.gateResult === "pass" && scan.dbStale ? "warning" : meta.tone;
  const Icon = meta.icon;

  return (
    <>
      <Badge pill className={TONE_CLASS[tone]}>
        <Icon {...iconProps("xs")} />
        {t(meta.labelKey)}
      </Badge>
      {scan.dbStale && (
        <Badge pill className={TONE_CLASS.warning}>
          <TriangleAlert {...iconProps("xs")} />
          {t("cicdListPage.imageScan.dbStale")}
        </Badge>
      )}
    </>
  );
}

/** 실행 목록 한 줄에 들어가는 요약. 판정과 C/H/M/L 건수만 둔다. */
export function ImageScanRowSummary({ scan }: { scan: PipelineImageScan }) {
  return (
    <span className="inline-flex flex-wrap items-center gap-1.5">
      <ImageScanGateBadge scan={scan} />
      <SeverityCounts compact counts={scan.counts} />
    </span>
  );
}

/** 선택한 실행의 상세. 판정의 근거(이미지·스캐너·DB 날짜)와 원본 리포트까지 보여준다. */
export function ImageScanDetail({
  scan,
  locale,
}: {
  scan: PipelineImageScan;
  locale: string;
}) {
  const { t } = useTranslation();
  const href = reportHref(scan.reportUri);
  const imageRef = scan.imageRepository
    ? scan.imageTag
      ? `${scan.imageRepository}:${scan.imageTag}`
      : scan.imageRepository
    : "-";
  const scanner =
    [scan.scanner, scan.scannerVersion].filter(Boolean).join(" ") || "-";
  const labelClass = "text-[var(--color-text-secondary)]";
  const valueClass = "m-0 min-w-0 text-[var(--color-text-primary)]";

  return (
    <div className="mt-3 rounded-lg border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] p-2">
      <div className="mb-2 flex flex-wrap items-center gap-2">
        <span className="text-[12px] font-semibold text-[var(--color-text-primary)]">
          {t("cicdListPage.imageScan.title")}
        </span>
        <ImageScanGateBadge scan={scan} />
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
      {scan.dbStale && (
        <p className="mb-2 mt-0 text-[11px] text-[var(--color-warning)]">
          {t("cicdListPage.imageScan.dbStaleHint")}
        </p>
      )}

      <SeverityCounts counts={scan.counts} />

      <dl className="mb-0 mt-2 grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1 text-[12px]">
        <dt className={labelClass}>{t("cicdListPage.imageScan.image")}</dt>
        <dd className={`${valueClass} break-all font-mono`}>{imageRef}</dd>
        <dt className={labelClass}>{t("cicdListPage.imageScan.digest")}</dt>
        <dd className={valueClass}>
          {scan.imageDigest ? (
            <code title={scan.imageDigest} className="font-mono">
              {shortImageDigest(scan.imageDigest)}
            </code>
          ) : (
            "-"
          )}
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
