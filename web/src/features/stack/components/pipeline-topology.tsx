// 파이프라인 토폴로지 레일.
//
// 개편 전에는 스테이지마다 카드를 두고 그 안에 이렇게 적었다:
//
//   OSS      gitlab-package-registry + gitlab + gitlab-registry + minio
//   Version  9.5.1 / 9.5.1 / 9.5.1 / 5.2.0
//
// 이름과 버전을 각각 join 한 문자열 두 개라, 어느 게 5.2.0 인지 알려면 자리를
// 세야 했다. 원본 데이터에는 도구별로 {name, version, instances} 가 살아 있는데
// 렌더 직전에 버리고 있었다. 이제 도구 한 줄에 로고·이름·버전을 함께 둔다.
//
// 스테이지 옆의 24px 색 동그라미도 걷어냈다. 카테고리마다 임의로 배정한 색이라
// 아무 뜻이 없었고, 자리는 로고 크기만큼 차지했다. 그 자리는 실제 브랜드 로고가
// 가져간다(public/tool-icons) — 읽지 않아도 GitLab 인지 Argo 인지 보인다.
//
// health/sync 배지는 노드마다 달지 않는다. 값이 stack.status 하나에서 나오므로
// 노드 8개가 늘 같은 값을 반복했다 — 1비트를 8번 그린 셈이다. 헤더에 한 번만 둔다.

import { useState } from "react";
import { ChevronRight } from "lucide-react";
import { cn } from "../../../lib/utils";
import type { PipelineNode, PipelineTool } from "../utils/stack-list-utils";
import { toolLogoURL } from "../utils/tool-logo";

function ToolMark({ name }: { name: string }) {
  const [failed, setFailed] = useState(false);

  if (failed) {
    return (
      <span
        aria-hidden="true"
        className="flex h-4 w-4 shrink-0 items-center justify-center rounded-[var(--radius-sm)] bg-[color-mix(in_srgb,_var(--color-text-primary)_10%,_transparent)] text-[9px] font-bold text-[var(--color-text-secondary)]"
      >
        {name.charAt(0).toUpperCase()}
      </span>
    );
  }

  return (
    <img
      src={toolLogoURL(name)}
      alt=""
      aria-hidden="true"
      className="h-4 w-4 shrink-0 object-contain"
      onError={() => setFailed(true)}
    />
  );
}

function ToolRow({ tool }: { tool: PipelineTool }) {
  return (
    <div
      className={cn(
        "flex items-start gap-1.5 py-[3px]",
        // 앞 스테이지에 이미 나온 도구는 한 톤 낮춘다 — 새 배포가 아니라
        // 같은 파드가 역할을 하나 더 맡고 있다는 뜻이다.
        tool.shared && "opacity-60",
      )}
    >
      <div className="pt-px">
        <ToolMark name={tool.name} />
      </div>
      {/* truncate 를 쓰지 않는다. gitlab-package-registry 처럼 긴 이름이
          "gitlab-package-…" 로 잘리면 어느 배포인지 구분이 안 된다.
          하이픈에서 접히게 두는 편이 낫다. */}
      <span className="min-w-0 flex-1 break-words text-[12px] leading-[16px] text-[var(--color-text-primary)]">
        {tool.name}
      </span>
      <span className="shrink-0 font-mono text-[11px] leading-[16px] text-[var(--color-text-secondary)]">
        {tool.version}
      </span>
    </div>
  );
}

function StageCard({ node }: { node: PipelineNode }) {
  const sharedOnly = node.tools.every((tool) => tool.shared);

  return (
    <div className="flex w-[186px] shrink-0 flex-col rounded-[var(--radius-sm)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)]">
      <div className="truncate border-b border-[var(--color-border-default)] px-2.5 py-1.5 text-[11px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
        {node.category}
      </div>
      <div className="flex-1 px-2.5 py-1.5">
        {node.tools.map((tool) => (
          <ToolRow key={`${node.category}-${tool.name}`} tool={tool} />
        ))}
      </div>
      <div className="border-t border-[var(--color-border-default)] px-2.5 py-1 text-[11px] text-[var(--color-text-secondary)]">
        {sharedOnly
          ? // 파드가 앞 스테이지에 이미 계상됐다. 0 이라고 쓰면 안 떠 있는
            // 것처럼 읽히므로 어디에 있는지를 쓴다.
            "shared instance"
          : `${node.instances} ${node.instances === 1 ? "instance" : "instances"}`}
      </div>
    </div>
  );
}

export function PipelineTopologyRail({ nodes }: { nodes: PipelineNode[] }) {
  return (
    <div className="overflow-x-auto pb-1">
      <ol className="m-0 flex list-none items-stretch p-0">
        {nodes.map((node, idx) => (
          <li key={node.category} className="flex items-stretch">
            {idx > 0 && (
              <div
                className="flex w-6 shrink-0 items-center justify-center"
                aria-hidden="true"
              >
                {/* 선은 카드 사이를 잇고, 꺾쇠가 방향을 준다. 개편 전에는
                    카드 안에 절대배치한 2px 막대라 카드 높이가 달라지면
                    선과 카드가 어긋났다. */}
                <div className="h-px w-full bg-[var(--color-border-default)]" />
                <ChevronRight
                  size={12}
                  className="-ml-1 shrink-0 text-[var(--color-text-muted)]"
                />
              </div>
            )}
            <StageCard node={node} />
          </li>
        ))}
      </ol>
    </div>
  );
}
