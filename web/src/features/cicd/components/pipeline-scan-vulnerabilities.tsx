// 파이프라인 실행이 만든 이미지의 취약점 목록.
//
// 스캔 상세의 건수만으로는 무엇을 고쳐야 하는지 알 수 없다. 여기서 목록을 펼친다.
// 목록은 서버가 CI 리포트를 읽어 만든다 — 실행을 고를 때마다 CI 를 부르지 않도록
// 사용자가 펼쳤을 때만 조회한다.

import { useId, useState } from "react";
import { useTranslation } from "react-i18next";
import { ChevronRight } from "lucide-react";
import { iconProps } from "../../../components/ui/icon";
import { VulnerabilityList } from "../../../components/shared/vulnerability-list";
import { DEFAULT_VULNERABILITY_FILTER } from "../../../lib/vulnerability-list";
import { cn } from "../../../lib/utils";
import type { VulnerabilityListFilter } from "../../../types";
import { usePipelineScanVulnerabilities } from "../api/cicd-api";

export function PipelineScanVulnerabilities({
  pipelineId,
  scanId,
}: {
  pipelineId: string;
  /** 스캔 결과의 id (scan_<실행 id>). */
  scanId: string;
}) {
  const { t } = useTranslation();
  const [expanded, setExpanded] = useState(false);
  // 접었다 펼쳐도 고른 필터를 유지한다. 다른 실행으로 옮기면 호출부가 key 로 새로 만든다.
  const [filter, setFilter] = useState<VulnerabilityListFilter>(
    DEFAULT_VULNERABILITY_FILTER,
  );
  const regionId = useId();
  const { data, isLoading, isError, isFetching } =
    usePipelineScanVulnerabilities(pipelineId, scanId, filter, expanded);

  return (
    <div className="mt-2 border-t border-[var(--color-border-default)] pt-2">
      <button
        type="button"
        aria-expanded={expanded}
        aria-controls={expanded ? regionId : undefined}
        onClick={() => setExpanded((open) => !open)}
        className="inline-flex items-center gap-1 rounded-[var(--radius-sm)] text-[12px] font-semibold text-[var(--color-primary)] hover:underline"
      >
        <ChevronRight
          {...iconProps("xs")}
          className={cn("shrink-0 transition-transform", expanded && "rotate-90")}
        />
        {expanded
          ? t("common.vulnerabilityList.hide")
          : t("common.vulnerabilityList.show")}
      </button>
      {expanded && (
        <div id={regionId} className="mt-2">
          <VulnerabilityList
            data={data}
            isLoading={isLoading}
            isError={isError}
            isFetching={isFetching}
            filter={filter}
            onFilterChange={setFilter}
          />
        </div>
      )}
    </div>
  );
}
