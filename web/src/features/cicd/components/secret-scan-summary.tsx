// 파이프라인 실행의 시크릿 탐지(gitleaks) 결과.
//
// 이 결과는 다른 스캔과 무게가 다르다. 취약점은 고치면 끝나지만, 커밋에 들어간 비밀은
// 커밋을 지워도 이미 유출이다(Secret Detection 설계 §5). 그래서 탐지가 있으면 판정이 경고
// (warn)여도 "지금 폐기하고 회전하라" 를 먼저 보인다 — 경고 정책은 배포를 통과시켰을 뿐이다.
//
// 비밀값은 그리지 않는다. API 가 보내지 않고, 화면 모델(PipelineSecretScan)에도 그 칸이 없다.
// 여기서 보이는 것은 어디에 있었는지(규칙 · 파일 · 줄 · 커밋 · Fingerprint)뿐이다.

import { useTranslation } from "react-i18next";
import { ExternalLink } from "lucide-react";
import { iconProps } from "../../../components/ui/icon";
import { SecretScanGateBadge } from "../../../components/shared/gate-badge";
import { SecretRotateWarning } from "../../../components/shared/secret-rotate-warning";
import { tableHeadRowClass, tdClass, thClass } from "../../../components/shared/table-chrome";
import { formatDateTime } from "../../../lib/locale";
import type { PipelineSecretScan } from "../../../types";

// report_uri 는 CI 가 준 값이다. http(s) 가 아닌 값(javascript: 등)은 링크로 만들지 않는다.
function reportHref(uri: string | undefined): string | undefined {
  return uri && /^https?:\/\//i.test(uri) ? uri : undefined;
}

/** 탐지가 있는 판정인가. warn 도 탐지다 — 경고 정책이 통과시켰을 뿐이다. */
export const hasSecretFindings = (scan: { gateResult: string }) =>
  scan.gateResult === "block" || scan.gateResult === "warn";

/** 실행 목록 한 줄에 들어가는 요약. 판정과 탐지 건수만 둔다. */
export function SecretScanRowSummary({ scan }: { scan: PipelineSecretScan }) {
  const { t } = useTranslation();
  return (
    <span className="inline-flex flex-wrap items-center gap-1">
      <span className="text-[11px] font-semibold text-[var(--color-text-secondary)]">
        {t("cicdListPage.secretScan.label")}
      </span>
      <SecretScanGateBadge scan={scan} />
      {hasSecretFindings(scan) && scan.findingsCount !== undefined && (
        <span className="text-[11px] font-semibold text-[var(--color-error)]">
          {t("cicdListPage.secretScan.findings", { count: scan.findingsCount })}
        </span>
      )}
    </span>
  );
}

const shortHash = (value: string | undefined) => (value ? value.slice(0, 8) : "-");

/** 선택한 실행의 상세. 탐지 위치와 원본 리포트(값은 가려져 있다)까지 보여준다. */
export function SecretScanDetail({
  scan,
  locale,
}: {
  scan: PipelineSecretScan;
  locale: string;
}) {
  const { t } = useTranslation();
  const href = reportHref(scan.reportUri);
  const scanner = [scan.scanner, scan.scannerVersion].filter(Boolean).join(" ") || "-";
  const found = hasSecretFindings(scan);

  return (
    <div className="mt-3 rounded-lg border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] p-2">
      <div className="mb-2 flex flex-wrap items-center gap-2">
        <span className="text-[12px] font-semibold text-[var(--color-text-primary)]">
          {t("cicdListPage.secretScan.title")}
        </span>
        <SecretScanGateBadge scan={scan} />
        {found && scan.findingsCount !== undefined && (
          <span className="text-[12px] font-semibold text-[var(--color-error)]">
            {t("cicdListPage.secretScan.findings", { count: scan.findingsCount })}
          </span>
        )}
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

      {found && <SecretRotateWarning className="mb-2" />}
      {scan.gateResult === "error" && (
        <p className="mb-2 mt-0 text-[11px] text-[var(--color-text-secondary)]">
          {t("cicdListPage.secretScan.errorHint")}
        </p>
      )}

      {scan.rules.length > 0 && (
        <div className="mb-2 flex flex-wrap items-center gap-1.5 text-[11px]">
          <span className="font-semibold text-[var(--color-text-secondary)]">
            {t("cicdListPage.secretScan.rules")}
          </span>
          {scan.rules.map((rule) => (
            <span
              key={rule.ruleId}
              className="inline-flex items-center gap-1 rounded-md border border-[var(--color-border-default)] px-1.5 py-0.5"
            >
              <code className="font-mono text-[var(--color-text-primary)]">{rule.ruleId}</code>
              <span className="font-semibold tabular-nums text-[var(--color-text-primary)]">{rule.count}</span>
            </span>
          ))}
        </div>
      )}

      {scan.findings.length > 0 && (
        <div className="overflow-x-auto rounded-md border border-[var(--color-border-default)]">
          <table className="w-full border-collapse" aria-label={t("cicdListPage.secretScan.locations")}>
            <thead>
              <tr className={tableHeadRowClass}>
                <th className={thClass}>{t("cicdListPage.secretScan.rule")}</th>
                <th className={thClass}>{t("cicdListPage.secretScan.location")}</th>
                <th className={thClass}>{t("cicdListPage.secretScan.commit")}</th>
                <th className={thClass}>{t("cicdListPage.secretScan.fingerprint")}</th>
              </tr>
            </thead>
            <tbody>
              {scan.findings.map((finding) => (
                <tr key={finding.fingerprint || `${finding.file}:${finding.startLine}:${finding.ruleId}`}>
                  <td className={tdClass}>
                    <code className="font-mono text-[12px]">{finding.ruleId}</code>
                  </td>
                  <td className={`${tdClass} break-all font-mono text-[12px]`}>
                    {finding.startLine !== undefined ? `${finding.file}:${finding.startLine}` : finding.file}
                  </td>
                  <td className={`${tdClass} font-mono text-[12px]`} title={finding.commit}>
                    {shortHash(finding.commit)}
                  </td>
                  <td className={`${tdClass} font-mono text-[12px]`} title={finding.fingerprint}>
                    {finding.fingerprint ? `${finding.fingerprint.slice(0, 16)}…` : "-"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {found && (
        <p className="mb-0 mt-2 text-[11px] text-[var(--color-text-secondary)]">
          {t("cicdListPage.secretScan.valuesHidden")}
        </p>
      )}

      <dl className="mb-0 mt-2 grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1 text-[12px]">
        <dt className="text-[var(--color-text-secondary)]">{t("cicdListPage.imageScan.scanner")}</dt>
        <dd className="m-0 min-w-0 text-[var(--color-text-primary)]">{scanner}</dd>
        <dt className="text-[var(--color-text-secondary)]">{t("cicdListPage.imageScan.scannedAt")}</dt>
        <dd className="m-0 min-w-0 text-[var(--color-text-primary)]">
          {scan.scannedAt ? formatDateTime(scan.scannedAt, locale) : "-"}
        </dd>
      </dl>
    </div>
  );
}
